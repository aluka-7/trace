package trace

import (
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aluka-7/utils"
)

var _tracer Tracer = noopTracer{}

// BuiltinFormat 用于在"trace"包中划分与Tracer.Inject()和Tracer.Extract()方法一起使用的值。
type BuiltinFormat byte

const (
	// SystemTraceID 跟踪键
	SystemTraceID    = "trace-id"
	SystemTraceDebug = "trace-debug"
	flagSampled      = 0x01
	flagDebug        = 0x02

	maxLevel = 64
	// 硬码重置概率为0.00025, 1/4000
	probability = 0.00025

	// 支持格式列表:

	// HTTPFormat 将Trace表示为HTTP标头字符串对.
	// HTTPFormat格式要求键和值按原样作为HTTP标头有效(即,字符大小写可能不稳定,并且键中不允许使用特殊字符,值应转义URL,等等).
	// 载体必须是"http.Header".
	HTTPFormat BuiltinFormat = iota

	// GRPCFormat 承运人必须是`google.golang.org/grpc/metadata.MD`.
	GRPCFormat
)

// Config config.
type Config struct {
	// 报告网络,例如:Unix,TCP,UDP
	Network string `json:"network"`
	// 对于TCP和UDP网络，地址的格式为“ host：port”。
	// 对于Unix网络，该地址必须是文件系统路径。
	Addr string `json:"address"`
	// 报告超时
	Timeout utils.Duration `json:"timeout"`
	// DisableSample
	DisableSample bool `json:"disable_sample"`
	// ProtocolVersion
	ProtocolVersion int32 `json:"protocol_version"`
	// Probability probability sampling
	Probability float32
}

// Trace trace common interface.
type Trace interface {
	// TraceId 返回当前跟踪ID.
	TraceId() string

	// Fork 用客户端跟踪派生一个跟踪。
	Fork(serviceName, operationName string) Trace

	Follow(serviceName, operationName string) Trace

	// Finish 当跟踪完成时调用它.
	Finish(err *error)
	// Scan扫描跟踪信息。
	// 不推荐使用:不推荐使用Scan方法,请使用Inject代替Scan
	// Scan(ti *Info)

	// SetTag 添加跟踪标签。
	//
	// 如果为`key`设置了预先存在的标签，则该标签将被覆盖。
	//
	// 标记值可以是数字类型,字符串或布尔值.
	// 其他标记值类型的行为在OpenTracing级别上未定义。
	// 如果跟踪系统不知道如何处理特定的值类型,则可以忽略该标签,但不要惊慌。
	// 注意当前仅支持旧标签:TagAnnotation TagAddress TagComment other将被忽略
	SetTag(tags ...Tag) Trace

	//SetLog  LogFields是一种有效且经过类型检查的方式来记录key:value 注意当前不支持
	SetLog(logs ...LogField) Trace

	// Visit 跟踪访问k-v对，并分别为fn.
	Visit(fn func(k, v string))

	// SetTitle 重置跟踪标题
	SetTitle(title string)
}

// Tracer 是用于跟踪创建和传播的简单,轻界面.
type Tracer interface {
	// New trace instance with given title.
	New(operationName string, opts ...Option) Trace
	// Inject 接收Trace实例,并注入它以便在`carrier`中传播.
	// 载体的实际类型取决于格式的值.
	Inject(t Trace, format interface{}, carrier interface{}) error
	// Extract 返回给定`format`和`carrier`的Trace实例.
	// 如果未找到跟踪,则返回`ErrTraceNotFound`.
	Extract(format interface{}, carrier interface{}) (Trace, error)
}

// Init init trace report.
func Init(serviceName string, tags []Tag, cfg *Config) {
	fmt.Println("Loading Trace Engine")
	report := newReport(cfg.Network, cfg.Addr, time.Duration(cfg.Timeout), cfg.ProtocolVersion)
	SetGlobalTracer(NewTracer(serviceName, tags, report, cfg.DisableSample))
}

// SetGlobalTracer SetGlobalTracer
func SetGlobalTracer(tracer Tracer) {
	fmt.Println("Loading Set Global Tracer")
	_tracer = tracer
}

// NewTracer new a tracer.
func NewTracer(serviceName string, tags []Tag, report reporter, disableSample bool) Tracer {
	sampler := newSampler(probability)
	stdLog := log.New(os.Stderr, "trace", log.LstdFlags)
	return &dapper{
		serviceName:   serviceName,
		disableSample: disableSample,
		propagators:   map[interface{}]propagator{HTTPFormat: httpPropagator{}, GRPCFormat: gRpcPropagator{}},
		reporter:      report,
		sampler:       sampler,
		tags:          tags,
		pool:          &sync.Pool{New: func() interface{} { return new(Span) }},
		stdLog:        stdLog,
	}
}

var defaultOption = option{}

type option struct {
	Debug bool
}

// Option dapper Option
type Option func(*option)

// EnableDebug enable debug mode
func EnableDebug() Option {
	return func(opt *option) {
		opt.Debug = true
	}
}

// New trace instance with given operationName.
func New(operationName string, opts ...Option) Trace {
	return _tracer.New(operationName, opts...)
}

// Inject 接收Trace实例,并将其注入以在`carrier`中传播.载体的实际类型取决于format的值.
func Inject(t Trace, format interface{}, carrier interface{}) error {
	return _tracer.Inject(t, format, carrier)
}

// Extract 返回给定`format`和`carrier'的Trace实例. 如果未找到跟踪，则返回`ErrTraceNotFound`.
func Extract(format interface{}, carrier interface{}) (Trace, error) {
	return _tracer.Extract(format, carrier)
}

// Close trace flush data.
func Close() error {
	if closer, ok := _tracer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// SpanContext实现opentracing.SpanContext
type spanContext struct {
	// TraceId 表示跟踪的全局唯一Id;通常生成为随机数.
	TraceId uint64

	// SpanId 表示范围ID,在其跟踪范围内必须是唯一的,但不必是全局唯一的.
	SpanId uint64

	// ParentId 是指父范围的Id;如果当前范围是根范围,则应为0.
	ParentId uint64

	// Flags 是包含诸如“采样”和“调试”之类的位图.
	Flags byte

	// Probability
	Probability float32

	// Level现在的水平
	Level int
}

func (c spanContext) isSampled() bool {
	return (c.Flags & flagSampled) == flagSampled
}

func (c spanContext) isDebug() bool {
	return (c.Flags & flagDebug) == flagDebug
}

// IsValid check spanContext valid
func (c spanContext) IsValid() bool {
	return c.TraceId != 0 && c.SpanId != 0
}

// emptyContext emptyContext
var emptyContext = spanContext{}

// 将spanContext转换为String
// {TraceId}:{SpanId}:{ParentId}:{flags}:[extend...]
// TraceId: uint64 base16
// SpanId: uint64 base16
// ParentId: uint64 base16
// 标志:
// - :0 sampled flag
// - :1 debug flag
// extend:
// sample-rate: s-{base16(BigEndian(float32))}
func (c spanContext) String() string {
	base := make([]string, 4)
	base[0] = strconv.FormatUint(c.TraceId, 16)
	base[1] = strconv.FormatUint(c.SpanId, 16)
	base[2] = strconv.FormatUint(c.ParentId, 16)
	base[3] = strconv.FormatUint(uint64(c.Flags), 16)
	return strings.Join(base, ":")
}

// 从字符串解析spanContext
func contextFromString(value string) (spanContext, error) {
	if value == "" {
		return emptyContext, errEmptyTracerString
	}
	items := strings.Split(value, ":")
	if len(items) < 4 {
		return emptyContext, errInvalidTracerString
	}
	parseHexUint64 := func(hex []string) ([]uint64, error) {
		ret := make([]uint64, len(hex))
		var err error
		for i, hex := range hex {
			ret[i], err = strconv.ParseUint(hex, 16, 64)
			if err != nil {
				break
			}
		}
		return ret, err
	}
	ret, err := parseHexUint64(items[0:4])
	if err != nil {
		return emptyContext, errInvalidTracerString
	}
	sc := spanContext{
		TraceId:  ret[0],
		SpanId:   ret[1],
		ParentId: ret[2],
		Flags:    byte(ret[3]),
	}
	return sc, nil
}
