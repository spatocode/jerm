package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewGolangRuntime(t *testing.T) {
	assert := assert.New(t)
	fakeOutput = "go version go1.21.0 linux/amd64"
	r := NewGoRuntime(fakeCommandExecutor{})
	g := r.(*Go)
	assert.Equal(RuntimeGo, g.Name)
	assert.Equal("1.21.0", g.Version)
}

func TestNewGolangRuntimeDefaultVersion(t *testing.T) {
	assert := assert.New(t)
	fakeOutput = ""
	r := NewGoRuntime(fakeCommandExecutor{})
	g := r.(*Go)
	assert.Equal(RuntimeGo, g.Name)
	assert.Equal(DefaultGoVersion, g.Version)
}

func TestGoGetVersion(t *testing.T) {
	assert := assert.New(t)
	fakeOutput = "go version go1.21.0 linux/amd64"
	r := NewGoRuntime(fakeCommandExecutor{})
	g := r.(*Go)
	v, err := g.getVersion()
	assert.Nil(err)
	assert.Equal(RuntimeGo, g.Name)
	assert.Equal("1.21.0", v)
}

func TestGoGetVersionError(t *testing.T) {
	assert := assert.New(t)
	fakeOutput = ""
	r := NewGoRuntime(fakeCommandExecutor{})
	g := r.(*Go)
	v, err := g.getVersion()
	assert.NotNil(err)
	assert.Equal(RuntimeGo, g.Name)
	assert.Equal("", v)
}

func TestGoLambdaRuntime(t *testing.T) {
	assert := assert.New(t)
	fakeOutput = "go version go1.21.0 linux/amd64"
	r := NewGoRuntime(fakeCommandExecutor{})
	g := r.(*Go)
	v, err := g.lambdaRuntime()
	assert.Nil(err)
	assert.Equal("go1.x", v)
}

func TestGoBuild(t *testing.T) {
	assert := assert.New(t)
	fakeOutput = "go version go1.21.0 linux/amd64"
	r := NewGoRuntime(fakeCommandExecutor{})
	cfg := &Config{Name: "test", Stage: "env"}
	p, f, err := r.Build(cfg)
	assert.Nil(err)
	assert.Equal("main", p)
	assert.Equal("main", f)
}

func TestGoBuildError(t *testing.T) {
	assert := assert.New(t)
	fakeOutput = ""
	r := NewGoRuntime(fakeCommandExecutor{})
	cfg := &Config{Name: "test", Stage: "env"}
	p, f, err := r.Build(cfg)
	assert.NotNil(err)
	assert.Equal("", p)
	assert.Equal("", f)
}
