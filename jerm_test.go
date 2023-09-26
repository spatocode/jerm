package jerm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJermConfigure(t *testing.T) {
	assert := assert.New(t)
	role := "arn:aws:iam::269360183919:role/bodystats-dev-JermTestLambdaServiceExecutionRole"
	c, err := Configure("assets/tests/jerm.json")
	assert.Nil(err)
	assert.Equal("bodystats", c.Name)
	assert.Equal("dev", c.Stage)
	assert.Equal(512, c.Lambda.Memory)
	assert.Equal("jerm-1699348021", c.Bucket)
	assert.Equal("us-west-2", c.Region)
	assert.Equal(role, c.Lambda.Role)
	assert.Equal("python3.11", c.Lambda.Runtime)
	assert.Equal(false, c.Lambda.KeepWarm)
	assert.Equal("/home/ubuntu/bodystats", c.Dir)
	assert.Equal(30, c.Lambda.Timeout)
	assert.Equal("bodyie", c.Entry)
}
