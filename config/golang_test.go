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

func TestGoLambdaRuntime(t *testing.T) {
	assert := assert.New(t)
	fakeOutput = "go version go1.21.0 linux/amd64"
	r := NewGoRuntime(fakeCommandExecutor{})
	g := r.(*Go)
	v, err := g.lambdaRuntime()
	assert.Nil(err)
	assert.Equal("go1.x", v)
}
