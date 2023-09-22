package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/otiai10/copy"
	"github.com/spatocode/jerm/internal/log"
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

func (r *Runtime) Build(config *Config, functionContent string) (string, string, error) {
	tempDir, err := os.MkdirTemp(os.TempDir(), "jerm-package")
	if err != nil {
		return "", "", err
	}

	err = r.copyNecessaryFilesToPackageDir(config.Dir, tempDir, jermIgnoreFile)
	if err != nil {
		return "", "", err
	}

	return tempDir, DefaultServerlessFunction, nil
}

// Copies files from src to dest ignoring file names listed in ignoreFile
func (r *Runtime) copyNecessaryFilesToPackageDir(src, dest, ignoreFile string) error {
	log.Debug(fmt.Sprintf("copying necessary files to package dir %s...", dest))

	ignoredFiles := defaultIgnoredGlobs
	files, err := ReadIgnoredFiles(ignoreFile)
	if err == nil {
		ignoredFiles = append(ignoredFiles, files...)
	}

	opt := copy.Options{
		Skip: func(srcinfo os.FileInfo, src, dest string) (bool, error) {
			for _, ignoredFile := range ignoredFiles {
				match, _ := filepath.Match(ignoredFile, srcinfo.Name())
				matchedFile := srcinfo.Name() == ignoredFile || match ||
					strings.HasSuffix(srcinfo.Name(), ignoredFile) ||
					strings.HasPrefix(srcinfo.Name(), ignoredFile)
				if matchedFile {
					return matchedFile, nil
				}
			}
			return false, nil
		},
	}
	err = copy.Copy(src, dest, opt)
	if err != nil {
		return err
	}

	return nil
}

func (r *Runtime) Entry() string {
	return ""
}

func (r *Runtime) lambdaRuntime() (string, error) {
	if r.Name == RuntimeUnknown {
		return "", errors.New("cannot detect runtime. please specify runtime in your Jerm.json file")
	}
	// TODO: Some AWS runtime id doesn't tally with this format.
	// Need to support as needed
	v := strings.Split(r.Version, ".")
	return fmt.Sprintf("%s%s", r.Name, v[0]), nil
}
