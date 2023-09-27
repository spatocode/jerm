package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spatocode/jerm/internal/log"
	"github.com/spatocode/jerm/internal/utils"
)

// Go runtime
type Go struct {
	*Runtime
}

// NewGoConfig instantiates a new Go runtime
func NewGoRuntime() RuntimeInterface {
	runtime := &Runtime{}
	g := &Go{runtime}
	g.Name = RuntimeGo
	version, err := g.getVersion()
	if err != nil {
		log.Debug(fmt.Sprintf("encountered an error while getting go version. Default to %s", DefaultGoVersion))
		g.Version = DefaultGoVersion
		return g
	}
	g.Version = version
	return g
}

// Gets the go version
func (g *Go) getVersion() (string, error) {
	log.Debug("getting go version...")
	goVersion, err := utils.GetShellCommandOutput("go", "version")
	if err != nil {
		return "", err
	}
	s := strings.Split(goVersion, " ")
	if len(s) > 1 {
		version := strings.Split(s[2], "go")
		return strings.TrimSpace(version[1]), nil
	}
	return "", errors.New("encountered error on go version")
}

// Build builds the go deployment package
// It returns the executable path and the function name
func (g *Go) Build(config *Config, functionContent string) (string, string, error) {
	_, err := utils.GetShellCommandOutput("go", "mod", "tidy")
	if err != nil {
		return "", "", err
	}

	env := []string{"GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0"}
	_, err = utils.GetShellCommandOutputWithEnv(env, "go", "build", "main.go")
	if err != nil {
		return "", "", err
	}

	return "main", "main", nil
}

// Entry is the directory where the cloud function handler resides.
// The directory can be a file.
func (g *Go) Entry() string {
	return "main.go"
}

// lambdaRuntime is the name of the go runtime as specified by AWS Lambda
func (g *Go) lambdaRuntime() (string, error) {
	v := strings.Split(g.Version, ".")
	return fmt.Sprintf("%s%s.x", g.Name, v[0]), nil
}
