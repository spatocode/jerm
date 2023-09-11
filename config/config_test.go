package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadConfigAlwaysReturnsConfig(t *testing.T) {
	assert := assert.New(t)
	cfg, _ := ReadConfig("")
	assert.NotNil(cfg)
}
