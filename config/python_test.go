package config

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/spatocode/jerm/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestNewPythonRuntime(t *testing.T) {
	assert := assert.New(t)
	fakeOutput = "Python 3.9.0"
	r := NewPythonRuntime(fakeCommandExecutor{})
	p := r.(*Python)
	assert.Equal(RuntimePython, p.Name)
	assert.Equal("3.9.0", p.Version)
}

func TestNewPythonRuntimeDefaultVersion(t *testing.T) {
	assert := assert.New(t)
	fakeOutput = ""
	r := NewPythonRuntime(fakeCommandExecutor{})
	p := r.(*Python)
	assert.Equal(RuntimePython, p.Name)
	assert.Equal(DefaultPythonVersion, p.Version)
}

func TestPythonGetVersion(t *testing.T) {
	assert := assert.New(t)
	fakeOutput = "Python 3.9.0"
	r := NewPythonRuntime(fakeCommandExecutor{})
	p := r.(*Python)
	v, err := p.getVersion()
	assert.Nil(err)
	assert.Equal(RuntimePython, p.Name)
	assert.Equal("3.9.0", v)
}

func TestPythonGetVersionError(t *testing.T) {
	assert := assert.New(t)
	fakeOutput = ""
	r := NewPythonRuntime(fakeCommandExecutor{})
	p := r.(*Python)
	v, err := p.getVersion()
	assert.Error(err)
	assert.Equal(RuntimePython, p.Name)
	assert.Equal("", v)
}

func TestPythonGetVirtualEnvironment(t *testing.T) {
	assert := assert.New(t)
	fakeOutput = "/usr/fake"
	r := NewPythonRuntime(fakeCommandExecutor{})
	p := r.(*Python)
	venv, err := p.getVirtualEnvironment()
	assert.Nil(err)
	assert.Equal(fmt.Sprintf("%s/versions%s", fakeOutput, fakeOutput), venv)
}

func TestPythonLambdaRuntime(t *testing.T) {
	assert := assert.New(t)
	fakeOutput = "Python 3.9.0"
	r := NewPythonRuntime(fakeCommandExecutor{})
	p := r.(*Python)
	v, err := p.lambdaRuntime()
	assert.Nil(err)
	assert.Equal("python3.9", v)
}

func TestPythonIsDjango(t *testing.T) {
	assert := assert.New(t)

	fakeOutput = "Python 3.9.0"
	r := NewPythonRuntime(fakeCommandExecutor{})
	p := r.(*Python)

	managePy := "manage.py"
	helperCreateFile(t, managePy)
	is := p.IsDjango()
	assert.True(is)
	helperCleanup(t, []string{managePy})
}

func TestPythonCreateFunctionHandler(t *testing.T) {
	assert := assert.New(t)

	fakeOutput = "Python 3.9.0"
	r := NewPythonRuntime(fakeCommandExecutor{})
	p := r.(*Python)

	handlerFile := filepath.Join("../assets/tests", "handler.py")
	handler, err := p.createFunctionHandler(handlerFile, []byte("This is a test handler"))

	assert.Nil(err)
	assert.Equal("handler.handler", handler)
	helperCleanup(t, []string{handlerFile})
}

func TestPythonExtractWheel(t *testing.T) {
	assert := assert.New(t)

	fakeOutput = "Python 3.9.0"
	r := NewPythonRuntime(fakeCommandExecutor{})
	p := r.(*Python)

	file1 := "../assets/tests/test"
	file2 := "../assets/tests/test.txt"
	p.extractWheel("../assets/tests/test.whl", "../assets/tests")
	assert.True(utils.FileExists(file1))
	assert.True(utils.FileExists(file2))

	helperCleanup(t, []string{file1, file2})
}
