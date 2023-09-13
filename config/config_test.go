package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadConfigAlwaysReturnsConfig(t *testing.T) {
	assert := assert.New(t)
	cfg, _ := ReadConfig("")
	assert.NotNil(cfg)
}

func TestConfigGetFunctionName(t *testing.T) {
	assert := assert.New(t)
	cfg := &Config{Name: "test", Stage: "env",}
	assert.Equal(fmt.Sprintf("%s-%s", cfg.Name, cfg.Stage), cfg.GetFunctionName())
}
