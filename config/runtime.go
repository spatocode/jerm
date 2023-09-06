package config

import (
	"fmt"
	"strings"

	"github.com/spatocode/jerm/internal/log"
	"github.com/spatocode/jerm/internal/utils"
)

type Runtime struct {
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

var (
	defaultIgnoredGlobs = []string{
		".zip",
		".exe",
		".git",
		".DS_Store",
		"pip",
		"venv",
		"__pycache__",
		".hg",
		".Python",
		"setuputils",
		"tar.gz",
		".git",
		".vscode",
		"docutils",
	}
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

func (r *Runtime) lambdaRuntime() string {
	if r.Version == "" {
		return DefaultPythonRuntime
	}
	v := strings.Split(r.Version, ".")
	return fmt.Sprintf("%s%s.%s", r.Name, v[0], v[1])
}

func (r *Runtime) python() {
	r.Name = RuntimePython
	p := NewPythonConfig()
	version, err := p.getVersion()
	if err != nil {
		log.Debug("error encountered while getting python version.")
	}
	r.Version = version

	if p.isDjango() {
		entry, err := p.getDjangoProject()
		if err != nil {
			log.Debug(err.Error())
		}
		r.Entry = entry
	}
}

func (r *Runtime) golang() {
	r.Name = RuntimeGo
}

func (r *Runtime) node() {
	r.Name = RuntimeNode
}
