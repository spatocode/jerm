package config

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spatocode/jerm/internal/utils"
)

type Runtime struct {
	Name    string
	Version string
	Entry	string
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
		slog.Debug("Error encountered while getting python version.")
	}
	r.Version = version

	if r.isDjango() {
		entry, err := r.getDjangoProject()
		if err != nil {
			slog.Debug(err.Error())
		}
		r.Entry = entry
	}
}

func (r *Runtime) isDjango() bool {
	return utils.FileExists("manage.py")
}

func (r *Runtime) getDjangoProject() (string, error) {
	workDir, _ := os.Getwd()
	djangoPath := ""
	walker := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, "settings.py") {
			d := filepath.Dir(path)
			splitPath := strings.Split(d, string(filepath.Separator))
			djangoPath = splitPath[len(splitPath)-1]
		}

		return nil
	}
	err := filepath.WalkDir(workDir, walker)
	if err != nil {
		return "", err
	}
	return djangoPath, nil
}

func (r *Runtime) golang() {
	r.Name = RuntimeGo
}

func (r *Runtime) node() {
	r.Name = RuntimeNode
}
