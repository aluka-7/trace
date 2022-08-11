package trace

import (
	"log"
	"net/http"
	"sync"
	"time"

	"google.golang.org/grpc/metadata"
)

// Carrier 传播者必须将通用接口{}转换为此实现Carrier接口的东西,Trace可以使用Carrier表示自己。
type Carrier interface {
	Set(key, val string)
	Get(key string) string
}

// 传播者负责从特定格式的"Carrier"中注入和提取"Trace"实例
type propagator interface {
	Inject(carrier interface{}) (Carrier, error)
	Extract(carrier interface{}) (Carrier, error)
}

type httpPropagator struct{}

type httpCarrier http.Header

func (h httpCarrier) Set(key, val string) {
	http.Header(h).Set(key, val)
}

func (h httpCarrier) Get(key string) string {
	return http.Header(h).Get(key)
}

func (httpPropagator) Inject(carrier interface{}) (Carrier, error) {
	header, ok := carrier.(http.Header)
	if !ok {
		return nil, ErrInvalidCarrier
	}
	if header == nil {
		return nil, ErrInvalidTrace
	}
	return httpCarrier(header), nil
}

func (httpPropagator) Extract(carrier interface{}) (Carrier, error) {
	header, ok := carrier.(http.Header)
	if !ok {
		return nil, ErrInvalidCarrier
	}
	if header == nil {
		return nil, ErrTraceNotFound
	}
	return httpCarrier(header), nil
}

type gRpcPropagator struct{}

type gRpcCarrier map[string][]string

func (g gRpcCarrier) Get(key string) string {
	if v, ok := g[key]; ok && len(v) > 0 {
		return v[0]
	}
	return ""
}

func (g gRpcCarrier) Set(key, val string) {
	g[key] = append(g[key], val)
}

func (gRpcPropagator) Inject(carrier interface{}) (Carrier, error) {
	md, ok := carrier.(metadata.MD)
	if !ok {
		return nil, ErrInvalidCarrier
	}
	if md == nil {
		return nil, ErrInvalidTrace
	}
	return gRpcCarrier(md), nil
}

func (gRpcPropagator) Extract(carrier interface{}) (Carrier, error) {
	md, ok := carrier.(metadata.MD)
	if !ok {
		return nil, ErrInvalidCarrier
	}
	if md == nil {
		return nil, ErrTraceNotFound
	}
	return gRpcCarrier(md), nil
}

type dapper struct {
	serviceName   string
	disableSample bool
	tags          []Tag
	reporter      reporter
	propagators   map[interface{}]propagator
	pool          *sync.Pool
	stdLog        *log.Logger
	sampler       sampler
}

func (d *dapper) New(operationName string, opts ...Option) Trace {
	opt := defaultOption
	for _, fn := range opts {
		fn(&opt)
	}
	traceId := genID()
	var sampled bool
	var probability float32
	if d.disableSample {
		sampled = true
		probability = 1
	} else {
		sampled, probability = d.sampler.IsSampled(traceId, operationName)
	}
	ctx := spanContext{TraceId: traceId}
	if sampled {
		ctx.Flags = flagSampled
		ctx.Probability = probability
	}
	if opt.Debug {
		ctx.Flags |= flagDebug
		return d.newSpanWithContext(operationName, ctx).SetTag(TagString(TagSpanKind, "server")).SetTag(TagBool("debug", true))
	}
	// 为了兼容临时为 New 的 Span 设置 span.kind
	return d.newSpanWithContext(operationName, ctx).SetTag(TagString(TagSpanKind, "server"))
}

func (d *dapper) newSpanWithContext(operationName string, ctx spanContext) Trace {
	sp := d.getSpan()
	// 如果未采样范围,则仅返回具有此上下文的范围,无需清除它
	if ctx.Level > maxLevel {
		// 如果跨度达到最大限制水平，则返回noopSpan
		return noopSpan{}
	}
	level := ctx.Level + 1
	sc := spanContext{
		TraceId:  ctx.TraceId,
		ParentId: ctx.SpanId,
		Flags:    ctx.Flags,
		Level:    level,
	}
	if ctx.SpanId == 0 {
		sc.SpanId = ctx.TraceId
	} else {
		sc.SpanId = genID()
	}
	sp.operationName = operationName
	sp.context = sc
	sp.startTime = time.Now()
	sp.tags = append(sp.tags, d.tags...)
	return sp
}

func (d *dapper) Inject(t Trace, format interface{}, carrier interface{}) error {
	// if carrier implement Carrier use direct, ignore format
	carr, ok := carrier.(Carrier)
	if ok {
		t.Visit(carr.Set)
		return nil
	}
	// use Built-in propagators
	pp, ok := d.propagators[format]
	if !ok {
		return ErrUnsupportedFormat
	}
	carr, err := pp.Inject(carrier)
	if err != nil {
		return err
	}
	if t != nil {
		t.Visit(carr.Set)
	}
	return nil
}

func (d *dapper) Extract(format interface{}, carrier interface{}) (Trace, error) {
	sp, err := d.extract(format, carrier)
	if err != nil {
		return sp, err
	}
	// 为了兼容临时为 New 的 Span 设置 span.kind
	return sp.SetTag(TagString(TagSpanKind, "server")), nil
}

func (d *dapper) extract(format interface{}, carrier interface{}) (Trace, error) {
	// if carrier implement Carrier use direct, ignore format
	carr, ok := carrier.(Carrier)
	if !ok {
		// use Built-in propagators
		pp, ok := d.propagators[format]
		if !ok {
			return nil, ErrUnsupportedFormat
		}
		var err error
		if carr, err = pp.Extract(carrier); err != nil {
			return nil, err
		}
	}
	ctx, err := contextFromString(carr.Get(FosTraceID))
	if err != nil {
		return nil, err
	}
	return d.newSpanWithContext("", ctx), nil
}

func (d *dapper) Close() error {
	return d.reporter.Close()
}

func (d *dapper) report(sp *Span) {
	if sp.context.isSampled() {
		if err := d.reporter.WriteSpan(sp); err != nil {
			d.stdLog.Printf("marshal trace span error: %s", err)
		}
	}
	d.putSpan(sp)
}

func (d *dapper) putSpan(sp *Span) {
	if len(sp.tags) > 32 {
		sp.tags = nil
	}
	if len(sp.logs) > 32 {
		sp.logs = nil
	}
	d.pool.Put(sp)
}

func (d *dapper) getSpan() *Span {
	sp := d.pool.Get().(*Span)
	sp.dapper = d
	sp.children = 0
	sp.tags = sp.tags[:0]
	sp.logs = sp.logs[:0]
	return sp
}
