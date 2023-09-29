package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewNodeRuntime(t *testing.T) {
	assert := assert.New(t)
	fakeOutput = "v13.2.0"
	r := NewNodeRuntime(fakeCommandExecutor{})
	n := r.(*Node)
	assert.Equal(RuntimeNode, n.Name)
	assert.Equal("13.2.0", n.Version)
}

func TestNodeGetVersion(t *testing.T) {
	assert := assert.New(t)
	fakeOutput = "v13.2.0"
	r := NewNodeRuntime(fakeCommandExecutor{})
	n := r.(*Node)
	v, err := n.getVersion()
	assert.Nil(err)
	assert.Equal(RuntimeNode, n.Name)
	assert.Equal("13.2.0", v)
}

func TestNodeLambdaRuntime(t *testing.T) {
	assert := assert.New(t)
	fakeOutput = "v13.2.0"
	r := NewNodeRuntime(fakeCommandExecutor{})
	n := r.(*Node)
	v, err := n.lambdaRuntime()
	assert.Nil(err)
	assert.Equal("nodejs13.x", v)
}
