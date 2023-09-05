package config

const (
	DefaultTimeout       = 30
	DefaultMemory        = 512
	DefaultNodeRuntime   = "nodejs18.x"
	DefaultPythonRuntime = "python3.11"
	DefaultGoRuntime     = "go1.x"
)

// Lambda configuration.
type Lambda struct {
	Runtime string `json:"runtime"`
	Timeout int    `json:"timeout"`
	Role    string `json:"role"`
	Memory  int    `json:"memory"`
	Handler string `json:"handler"`
}

func (l *Lambda) Defaults() {
	if l.Memory == 0 {
		l.Memory = DefaultMemory
	}

	if l.Timeout == 0 {
		l.Timeout = DefaultTimeout
	}

	if l.Runtime == "" {
		l.Runtime = l.detectRuntime()
	}
}

func (l *Lambda) detectRuntime() string {
	runtime := DetectRuntime()
	return runtime.lambdaRuntime()
}
