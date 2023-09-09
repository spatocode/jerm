package config

const (
	DefaultTimeout       = 30
	DefaultMemory        = 512
	DefaultNodeVersion   = "18."
	DefaultPythonVersion = "3.9"
	DefaultGoVersion     = "1.19"
)

// Lambda configuration.
type Lambda struct {
	Runtime  string `json:"runtime"`
	Timeout  int    `json:"timeout"`
	Role     string `json:"role"`
	Memory   int    `json:"memory"`
	Handler  string `json:"handler"`
	KeepWarm bool   `json:"keep_warm"`
}

func (l *Lambda) Defaults() error {
	var err error
	if l.Memory == 0 {
		l.Memory = DefaultMemory
	}

	if l.Timeout == 0 {
		l.Timeout = DefaultTimeout
	}

	if l.Runtime == "" {
		runtime := l.detectRuntime()
		l.Runtime, err = runtime.lambdaRuntime()
		if err != nil {
			return err
		}
	}

	return nil
}

func (l *Lambda) detectRuntime() *Runtime {
	runtime := DetectRuntime()
	return runtime
}
