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

func TestIgnoredFiles(t *testing.T) {
	assert := assert.New(t)
	files, err := ReadIgnoredFiles("../assets/tests/.jermignore")
	expected := []string{"testfile1", "testfile2"}
	assert.Nil(err)
	assert.Equal(expected, files)
}
