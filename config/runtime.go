package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spatocode/jerm/internal/log"
	"github.com/spatocode/jerm/internal/utils"
)

type Runtime struct {
	Id      string
	Name    string
	Version string
	Entry   string
}

const (
	RuntimeUnknown = "unknown"
	RuntimePython  = "python"
	RuntimeGo      = "go"
	RuntimeNode    = "node"
)

func DetectRuntime() *Runtime {
	r := &Runtime{}
	switch {
	case utils.FileExists("requirements.txt"):
		r.python()
	case utils.FileExists("main.go"):
		r.golang()
	case utils.FileExists("package.json"):
		r.node()
	default:
		r.Name = RuntimeUnknown
	}
	return r
}

func (r *Runtime) lambdaRuntime() (string, error) {
	if r.Name == RuntimeUnknown {
		return "", errors.New("cannot detect runtime. please specify runtime in your Jerm.json file")
	}
	v := strings.Split(r.Version, ".")
	return fmt.Sprintf("%s%s.%s", r.Name, v[0], v[1]), nil
}

func (r *Runtime) python() {
	r.Name = RuntimePython
	p := NewPythonConfig()
	version, err := p.getVersion()
	if err != nil {
		log.Debug("error encountered while getting python version.")
		r.Version = DefaultPythonVersion
		return
	}
	r.Version = version
}

func (r *Runtime) golang() {
	r.Name = RuntimeGo
}

func (r *Runtime) node() {
	r.Name = RuntimeNode
}
