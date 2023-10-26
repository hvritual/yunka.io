package logExt

type Trace interface {
	Set(string)
	Get() string
}

type TraceLogger interface {
	Trace
	Logger
}
