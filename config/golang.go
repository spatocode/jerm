package config

import (
	"errors"
	"strings"

	"github.com/spatocode/jerm/internal/log"
	"github.com/spatocode/jerm/internal/utils"
)

type Golang struct{}

func NewGolangConfig() *Golang {
	return &Golang{}
}

// getVersion gets the go version
func (g *Golang) getVersion() (string, error) {
	log.Debug("Getting go version...")
	goVersion, err := utils.GetShellCommandOutput("go", "version")
	if err != nil {
		return "", err
	}
	s := strings.Split(goVersion, " ")
	if len(s) > 1 {
		version := strings.Split(s[2], "go")
		return version[1], nil
	}
	return "", errors.New("Encountered error on go version")
}

// Build builds the go deployment package
func (g *Golang) Build(config *Config) (string, error) {
	return "", nil
}
