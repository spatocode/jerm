package config

import (
	"os"
	"testing"

	"github.com/spatocode/jerm/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestIgnoredFilesWhileCopying(t *testing.T) {
	assert := assert.New(t)
	jermJson := "../assets/jerm.json"
	jermIgnore := "../assets/.jermignore"

	pr := NewPythonRuntime()
	p := pr.(*Python)
	err := p.copyNecessaryFilesToPackageDir("../assets/tests", "../assets", "../assets/tests/.jermignore")
	testfile1Exists := utils.FileExists("../assets/testfile1")
	testfile2Exists := utils.FileExists("../assets/testfile2")
	jermJsonExists := utils.FileExists(jermJson)
	jermIgnoreExists := utils.FileExists(jermIgnore)

	assert.Nil(err)
	assert.False(testfile1Exists)
	assert.False(testfile2Exists)
	assert.True(jermJsonExists)
	assert.True(jermIgnoreExists)

	cleanup([]string{jermJson, jermIgnore})
}

func cleanup(files []string) {
	for _, file := range files {
		os.Remove(file)
	}
}
