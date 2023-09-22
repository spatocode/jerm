package config

import (
	"errors"
	"os"

	"github.com/spatocode/jerm/internal/utils"
)

const (
	RuntimeUnknown            = "unknown"
	RuntimePython             = "python"
	RuntimeGo                 = "go"
	RuntimeNode               = "nodejs"
	RuntimeStatic             = "static"
	DefaultNodeVersion        = "18.13.0"
	DefaultPythonVersion      = "3.9.0"
	DefaultGoVersion          = "1.19.0"
	DefaultServerlessFunction = "handler"
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

type RuntimeInterface interface {
	// Builds the deployment package for the underlying runtime
	Build(*Config, string) (string, string, error)

	// Entry is the directory where the cloud function handler resides.
	// The directory can be a file.
	Entry() string

	// lambdaRuntime is the name of runtime as specified by AWS Lambda
	lambdaRuntime() (string, error)
}

// Base Runtime
type Runtime struct {
	Name    string
	Version string
}

// NewRuntime instantiates a new runtime
func NewRuntime() RuntimeInterface {
	r := &Runtime{}
	switch {
	case utils.FileExists("requirements.txt"):
		return NewPythonRuntime()
	case utils.FileExists("main.go"):
		return NewGoRuntime()
	case utils.FileExists("package.json"):
		return NewNodeRuntime()
	case utils.FileExists("index.html"):
		r.Name = RuntimeStatic
	default:
		r.Name = RuntimeUnknown
	}
	return r
}

func (r *Runtime) Build(*Config, string) (string, string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", "", err
	}

	return dir, DefaultServerlessFunction, nil
}

func (r *Runtime) Entry() string {
	return ""
}

func (r *Runtime) lambdaRuntime() (string, error) {
	return "", errors.New("cannot detect runtime. please specify runtime in your Jerm.json file")
}
