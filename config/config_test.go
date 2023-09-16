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

func TestReadConfig(t *testing.T) {
	assert := assert.New(t)
	c, err := ReadConfig("../test/data/jerm.json")
	role := "arn:aws:iam::269360183919:role/bodystats-dev-JermTestLambdaServiceExecutionRole"
	assert.Nil(err)
	assert.Equal("bodystats", c.Name)
	assert.Equal("dev", c.Stage)
	assert.Equal("jerm-1699348021", c.Bucket)
	assert.Equal("us-west-2", c.Region)
	assert.Equal("python3.11", c.Lambda.Runtime)
	assert.Equal(30, c.Lambda.Timeout)
	assert.Equal(role, c.Lambda.Role)
	assert.Equal(512, c.Lambda.Memory)
	assert.Equal(false, c.Lambda.KeepWarm)
	assert.Equal("/home/ubuntu/bodystats", c.Dir)
	assert.Equal("bodyie", c.Entry)
}
