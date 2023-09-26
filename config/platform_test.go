package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlaformDefaults(t *testing.T) {
	assert := assert.New(t)
	p := &Platform{Name: Lambda}
	assert.Equal(0, p.Memory)
	assert.Equal(0, p.Timeout)
	p.Defaults()
	assert.Equal(DefaultMemory, p.Memory)
	assert.Equal(DefaultTimeout, p.Timeout)
}
