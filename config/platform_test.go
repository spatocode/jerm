package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlaformDefaults(t *testing.T) {
	assert := assert.New(t)
	indexHtml := "index.html"

	p := &Platform{Name: Lambda}
	assert.Equal(0, p.Memory)
	assert.Equal(0, p.Timeout)
	assert.Equal("", p.Runtime)
	err := p.Defaults()
	assert.ErrorContains(err, "cannot detect runtime. please specify runtime in your Jerm.json file")
	assert.Equal("", p.Runtime)
	assert.Equal(DefaultMemory, p.Memory)
	assert.Equal(DefaultTimeout, p.Timeout)

	helperCreateFile(t, indexHtml)
	err = p.Defaults()
	assert.Nil(err)
	assert.Equal("nodejs18.x", p.Runtime)
	helperCleanup(t, []string{indexHtml})
}
