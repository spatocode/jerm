package config

import (
	"fmt"
	"os"
	"testing"

	"github.com/spatocode/jerm/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestReadConfigAlwaysReturnsConfig(t *testing.T) {
	assert := assert.New(t)
	cfg, _ := ReadConfig("")
	assert.NotNil(cfg)
}

func TestConfigGetFunctionName(t *testing.T) {
	assert := assert.New(t)
	cfg := &Config{Name: "test", Stage: "env"}
	assert.Equal(fmt.Sprintf("%s-%s", cfg.Name, cfg.Stage), cfg.GetFunctionName())
}

func TestConfigDefaults(t *testing.T) {
	assert := assert.New(t)
	cfg := &Config{}
	err := cfg.defaults()
	workspace, _ := utils.GetWorkspaceName()
	workDir, _ := os.Getwd()
	assert.Nil(err)
	assert.Equal(workspace, cfg.Name)
	assert.Equal(DefaultStage, Stage(cfg.Stage))
	assert.Contains(cfg.Bucket, "jerm-")
	assert.Contains(cfg.Dir, workDir)
	assert.NotNil(cfg.Region)
}
