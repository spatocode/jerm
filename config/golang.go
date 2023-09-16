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
	g := &Go{}
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
		return version[1], nil
	}
	return "", errors.New("encountered error on go version")
}

// Builds the go deployment package
func (g *Go) Build(config *Config) (string, error) {
	return "", nil
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
