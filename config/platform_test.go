package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlaformDefaults(t *testing.T) {
	assert := assert.New(t)
	indexHtml := "index.html"
	requirementsTxt := "requirements.txt"

	p := &Platform{Name: Lambda}
	assert.Equal(0, p.Memory)
	assert.Equal(0, p.Timeout)
	assert.Equal("", p.Runtime)
	err := p.Defaults()
	assert.EqualError(err, "cannot detect runtime. please specify runtime in your Jerm.json file")
	assert.Equal("", p.Runtime)
	assert.Equal(DefaultMemory, p.Memory)
	assert.Equal(DefaultTimeout, p.Timeout)

	helperCreateFile(t, indexHtml)
	err = p.Defaults()
	assert.Nil(err)
	assert.Equal("nodejs18.x", p.Runtime)
	helperCleanup(t, []string{indexHtml})

	helperCreateFile(t, requirementsTxt)
	err = p.Defaults()
	assert.Nil(err)
	assert.Equal("nodejs18.x", p.Runtime)
	helperCleanup(t, []string{requirementsTxt})

	p.Runtime = ""
	helperCreateFile(t, requirementsTxt)
	err = p.Defaults()
	assert.Nil(err)
	assert.NotEqual("nodejs18.x", p.Runtime)
	assert.Contains(p.Runtime, "python")
	helperCleanup(t, []string{requirementsTxt})
}
