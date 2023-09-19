package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadPromptInput(t *testing.T) {
	assert := assert.New(t)
	reader := strings.NewReader(" Foo \n")
	input, err := ReadPromptInput("Testing Prompt", reader)
	assert.Nil(err)
	assert.Equal("Foo", input)
}
