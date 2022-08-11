package trace

var _ Tracer = noopTracer{}

type noopTracer struct{}

func (n noopTracer) New(title string, opts ...Option) Trace {
	return noopSpan{}
}

func (n noopTracer) Inject(t Trace, format interface{}, carrier interface{}) error {
	return nil
}

func (n noopTracer) Extract(format interface{}, carrier interface{}) (Trace, error) {
	return noopSpan{}, nil
}

type noopSpan struct{}

func (n noopSpan) TraceId() string { return "" }

func (n noopSpan) Fork(string, string) Trace {
	return noopSpan{}
}

func (n noopSpan) Follow(string, string) Trace {
	return noopSpan{}
}

func (n noopSpan) Finish(err *error) {}

func (n noopSpan) SetTag(tags ...Tag) Trace {
	return noopSpan{}
}

func (n noopSpan) SetLog(logs ...LogField) Trace {
	return noopSpan{}
}

func (n noopSpan) Visit(func(k, v string)) {}

func (n noopSpan) SetTitle(string) {}

func (n noopSpan) String() string { return "" }
