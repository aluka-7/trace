package trace

// MockTrace .
type MockTrace struct {
	Spans []*MockSpan
}

// New .
func (m *MockTrace) New(operationName string, opts ...Option) Trace {
	span := &MockSpan{OperationName: operationName, MockTrace: m}
	m.Spans = append(m.Spans, span)
	return span
}

// Inject .
func (m *MockTrace) Inject(t Trace, format interface{}, carrier interface{}) error {
	return nil
}

// Extract .
func (m *MockTrace) Extract(format interface{}, carrier interface{}) (Trace, error) {
	return &MockSpan{}, nil
}

// MockSpan .
type MockSpan struct {
	*MockTrace
	OperationName string
	FinishErr     error
	Finished      bool
	Tags          []Tag
	Logs          []LogField
}

func (m *MockSpan) Fork(serviceName string, operationName string) Trace {
	span := &MockSpan{OperationName: operationName, MockTrace: m.MockTrace}
	m.Spans = append(m.Spans, span)
	return span
}

func (m *MockSpan) Follow(serviceName string, operationName string) Trace {
	span := &MockSpan{OperationName: operationName, MockTrace: m.MockTrace}
	m.Spans = append(m.Spans, span)
	return span
}

func (m *MockSpan) Finish(err *error) {
	if err != nil {
		m.FinishErr = *err
	}
	m.Finished = true
}

func (m *MockSpan) SetTag(tags ...Tag) Trace {
	m.Tags = append(m.Tags, tags...)
	return m
}

func (m *MockSpan) SetLog(logs ...LogField) Trace {
	m.Logs = append(m.Logs, logs...)
	return m
}

func (m *MockSpan) Visit(fn func(k, v string)) {}

func (m *MockSpan) SetTitle(title string) {
	m.OperationName = title
}

func (m *MockSpan) TraceId() string {
	return ""
}
