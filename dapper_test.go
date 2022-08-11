package trace

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/aluka-7/utils"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/metadata"
)

type mockReport struct {
	sps []*Span
}

func (m *mockReport) WriteSpan(sp *Span) error {
	m.sps = append(m.sps, sp)
	return nil
}

func (m *mockReport) Close() error {
	return nil
}
func extendTag() (tags []Tag) {
	tags = append(tags,
		TagString("ip", utils.InternalIP()),
	)
	return
}
func TestDapper(t *testing.T) {
	t.Run("test HTTP dapper", func(t *testing.T) {
		report := &mockReport{}
		t1 := NewTracer("service1", extendTag(), report, true)
		t2 := NewTracer("service2", extendTag(), report, true)
		sp1 := t1.New("opt_1")
		sp2 := sp1.Fork("", "opt_client")
		header := make(http.Header)
		t1.Inject(sp2, HTTPFormat, header)
		sp3, err := t2.Extract(HTTPFormat, header)
		if err != nil {
			t.Fatal(err)
		}
		sp3.Finish(nil)
		sp2.Finish(nil)
		sp1.Finish(nil)

		assert.Len(t, report.sps, 3)
		assert.Equal(t, report.sps[2].context.ParentId, uint64(0))
		assert.Equal(t, report.sps[0].context.TraceId, report.sps[1].context.TraceId)
		assert.Equal(t, report.sps[2].context.TraceId, report.sps[1].context.TraceId)

		assert.Equal(t, report.sps[1].context.ParentId, report.sps[2].context.SpanId)
		assert.Equal(t, report.sps[0].context.ParentId, report.sps[1].context.SpanId)
	})

	t.Run("test gRPC dapper", func(t *testing.T) {
		report := &mockReport{}
		t1 := NewTracer("service1", extendTag(), report, true)
		t2 := NewTracer("service2", extendTag(), report, true)
		sp1 := t1.New("opt_1")
		sp2 := sp1.Fork("", "opt_client")
		md := make(metadata.MD)
		t1.Inject(sp2, GRPCFormat, md)
		sp3, err := t2.Extract(GRPCFormat, md)
		if err != nil {
			t.Fatal(err)
		}
		sp3.Finish(nil)
		sp2.Finish(nil)
		sp1.Finish(nil)

		assert.Len(t, report.sps, 3)
		assert.Equal(t, report.sps[2].context.ParentId, uint64(0))
		assert.Equal(t, report.sps[0].context.TraceId, report.sps[1].context.TraceId)
		assert.Equal(t, report.sps[2].context.TraceId, report.sps[1].context.TraceId)

		assert.Equal(t, report.sps[1].context.ParentId, report.sps[2].context.SpanId)
		assert.Equal(t, report.sps[0].context.ParentId, report.sps[1].context.SpanId)
	})
	t.Run("test normal", func(t *testing.T) {
		report := &mockReport{}
		t1 := NewTracer("service1", extendTag(), report, true)
		sp1 := t1.New("test123")
		sp1.Finish(nil)
	})
	t.Run("test debug dapper", func(t *testing.T) {
		report := &mockReport{}
		t1 := NewTracer("service1", extendTag(), report, true)
		t2 := NewTracer("service2", extendTag(), report, true)
		sp1 := t1.New("opt_1", EnableDebug())
		sp2 := sp1.Fork("", "opt_client")
		header := make(http.Header)
		t1.Inject(sp2, HTTPFormat, header)
		sp3, err := t2.Extract(HTTPFormat, header)
		if err != nil {
			t.Fatal(err)
		}
		sp3.Finish(nil)
		sp2.Finish(nil)
		sp1.Finish(nil)

		assert.Len(t, report.sps, 3)
		assert.Equal(t, report.sps[2].context.ParentId, uint64(0))
		assert.Equal(t, report.sps[0].context.TraceId, report.sps[1].context.TraceId)
		assert.Equal(t, report.sps[2].context.TraceId, report.sps[1].context.TraceId)

		assert.Equal(t, report.sps[1].context.ParentId, report.sps[2].context.SpanId)
		assert.Equal(t, report.sps[0].context.ParentId, report.sps[1].context.SpanId)
	})
}

func BenchmarkSample(b *testing.B) {
	err := fmt.Errorf("test error")
	report := &mockReport{}
	t1 := NewTracer("service1", extendTag(), report, true)
	for i := 0; i < b.N; i++ {
		sp1 := t1.New("test_opt1")
		sp1.SetTag(TagString("test", "123"))
		sp2 := sp1.Fork("", "opt2")
		sp3 := sp2.Fork("", "opt3")
		sp3.SetTag(TagString("test", "123"))
		sp3.Finish(nil)
		sp2.Finish(&err)
		sp1.Finish(nil)
	}
}

func BenchmarkDisableSample(b *testing.B) {
	err := fmt.Errorf("test error")
	report := &mockReport{}
	t1 := NewTracer("service1", extendTag(), report, true)
	for i := 0; i < b.N; i++ {
		sp1 := t1.New("test_opt1")
		sp1.SetTag(TagString("test", "123"))
		sp2 := sp1.Fork("", "opt2")
		sp3 := sp2.Fork("", "opt3")
		sp3.SetTag(TagString("test", "123"))
		sp3.Finish(nil)
		sp2.Finish(&err)
		sp1.Finish(nil)
	}
}
