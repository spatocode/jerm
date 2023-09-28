package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/otiai10/copy"
	"github.com/spatocode/jerm/config/handlers"
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
	// It returns the package path, the function name and error if any.
	// The package path can be an executable for runtimes that compiles
	// to standalone executable.
	Build(*Config) (string, string, error)

	// Entry is the directory where the cloud function handler resides.
	// The directory can be a file.
	Entry() string

	// lambdaRuntime returns the name of runtime as specified by AWS Lambda
	lambdaRuntime() (string, error)
}

// Base Runtime
type Runtime struct {
	utils.ShellCommand
	Name            string
	Version         string
	handlerTemplate string
}

// NewRuntime instantiates a new runtime
func NewRuntime() RuntimeInterface {
	command := utils.Command()
	r := &Runtime{}
	switch {
	case utils.FileExists("requirements.txt"):
		return NewPythonRuntime(command)
	case utils.FileExists("main.go"):
		return NewGoRuntime(command)
	case utils.FileExists("package.json"):
		return NewNodeRuntime(command)
	case utils.FileExists("index.html"):
		r.Name = RuntimeStatic
		r.handlerTemplate = handlers.AwsLambdaHandlerStaticPage
	default:
		r.Name = RuntimeUnknown
	}
	r.ShellCommand = command
	return r
}

// Build builds the project for deployment
func (r *Runtime) Build(config *Config) (string, string, error) {
	tempDir, err := os.MkdirTemp(os.TempDir(), "jerm-package")
	if err != nil {
		return "", "", err
	}

	err = r.copyNecessaryFilesToPackageDir(config.Dir, tempDir, jermIgnoreFile)
	if err != nil {
		return "", "", err
	}

	if r.Name == RuntimeStatic {
		handlerFilepath := filepath.Join(tempDir, "index.js")
		_, err := r.createFunctionEntry(config, handlerFilepath)
		if err != nil {
			return "", "", err
		}
	}

	return tempDir, DefaultServerlessFunction, nil
}

// createFunctionEntry creates a serverless function handler file
func (r *Runtime) createFunctionEntry(config *Config, file string) (string, error) {
	log.Debug("creating lambda handler...")
	f, err := os.Create(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	_, err = f.Write([]byte(r.handlerTemplate))
	if err != nil {
		return "", err
	}
	return DefaultServerlessFunction, nil
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
