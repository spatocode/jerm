package config

import "fmt"

const (
	DefaultTimeout              = 30
	DefaultMemory               = 512
	Lambda         PlatformName = "lambda"
)

type PlatformName string

// Platform configuration.
type Platform struct {
	Name     PlatformName `json:"name"`
	Runtime  string       `json:"runtime"`
	Timeout  int          `json:"timeout"`
	Role     string       `json:"role"`
	Memory   int          `json:"memory"`
	Handler  string       `json:"handler"`
	KeepWarm bool         `json:"keep_warm"`
}

func (l *Platform) Defaults() error {
	var err error
	if l.Memory == 0 {
		l.Memory = DefaultMemory
	}

	if l.Timeout == 0 {
		l.Timeout = DefaultTimeout
	}

	if l.Runtime == "" {
		runtime := l.getRuntime()
		switch l.Name {
		case Lambda:
			l.Runtime, err = runtime.lambdaRuntime()
			if err != nil {
				return err
			}
			if l.Runtime == RuntimeStatic {
				l.Runtime = fmt.Sprintf("nodejs%s.x", DefaultNodeVersion[0:2])
			}
		}
	}

	return nil
}

func (l *Platform) getRuntime() RuntimeInterface {
	runtime := NewRuntime()
	return runtime
}
