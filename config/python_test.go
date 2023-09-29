package config

import (
	"fmt"
	"testing"

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
	assert.NotNil(err)
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
