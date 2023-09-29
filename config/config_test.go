package config

import (
	"fmt"
	"os"
	"testing"

	"github.com/spatocode/jerm/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestConfigGetFunctionName(t *testing.T) {
	assert := assert.New(t)
	cfg := &Config{Name: "test", Stage: "env"}
	assert.Equal(fmt.Sprintf("%s-%s", cfg.Name, cfg.Stage), cfg.GetFunctionName())
}

func TestConfigDefaults(t *testing.T) {
	assert := assert.New(t)
	cfg := &Config{}
	err := cfg.defaults()
	workspace, _ := utils.GetWorkspaceName()
	workDir, _ := os.Getwd()
	assert.Nil(err)
	assert.Equal(workspace, cfg.Name)
	assert.Equal(DefaultStage, Stage(cfg.Stage))
	assert.Contains(cfg.Bucket, "jerm-")
	assert.Contains(cfg.Dir, workDir)
	assert.NotNil(cfg.Region)
}

func TestConfigToJson(t *testing.T) {
	assert := assert.New(t)
	testfile := "../assets/test.json"
	cfg := &Config{}

	assert.False(utils.FileExists(testfile))
	err := cfg.ToJson(testfile)
	assert.Nil(err)
	assert.True(utils.FileExists(testfile))

	helperCleanup(t, []string{testfile})
}

func TestReadConfig(t *testing.T) {
	assert := assert.New(t)
	c, err := ReadConfig("../assets/tests/jerm.json")
	role := "arn:aws:iam::269360183919:role/bodystats-dev-JermTestLambdaServiceExecutionRole"
	assert.Nil(err)
	assert.Equal("bodystats", c.Name)
	assert.Equal("dev", c.Stage)
	assert.Equal("jerm-1699348021", c.Bucket)
	assert.Equal("us-west-2", c.Region)
	assert.Equal("python3.11", c.Platform.Runtime)
	assert.Equal(30, c.Platform.Timeout)
	assert.Equal(role, c.Platform.Role)
	assert.Equal(512, c.Platform.Memory)
	assert.Equal(false, c.Platform.KeepWarm)
	assert.Equal("/home/ubuntu/bodystats", c.Dir)
}

func TestIgnoredFiles(t *testing.T) {
	assert := assert.New(t)
	files, err := ReadIgnoredFiles("../assets/tests/.jermignore")
	expected := []string{"testfile1", "testfile2"}
	assert.Nil(err)
	assert.Equal(expected, files)
}

func TestAwsS3BucketCorrectNaming(t *testing.T) {
	assert := assert.New(t)
	cfg := &Config{}
	assert.True(cfg.isValidAwsS3BucketName("1test1."))
}

func TestAwsS3BucketNoSpecialCharactersAsidesDotsAndHyphen(t *testing.T) {
	assert := assert.New(t)
	cfg := &Config{}
	assert.False(cfg.isValidAwsS3BucketName("tes/t"))
}

func TestAwsS3BucketNoSthreehyphenconfiguratorPrefix(t *testing.T) {
	assert := assert.New(t)
	cfg := &Config{}
	assert.False(cfg.isValidAwsS3BucketName("sthree-configuratortest"))
}

func TestAwsS3BucketNameNoAdjacentDots(t *testing.T) {
	assert := assert.New(t)
	cfg := &Config{}
	assert.False(cfg.isValidAwsS3BucketName("te..st"))
}

func TestAwsS3BucketNameNoLessThan3Characters(t *testing.T) {
	assert := assert.New(t)
	cfg := &Config{}
	assert.False(cfg.isValidAwsS3BucketName("te"))
}

func TestAwsS3BucketNameNoMoreThan63Characters(t *testing.T) {
	assert := assert.New(t)
	cfg := &Config{}
	n := "wdteeumuimumnbvrewqsdfvtyuioopplnhgxtygbwhjsgfdfghhgfcxdsdfvbgfv"
	assert.False(cfg.isValidAwsS3BucketName(n))
}

func TestAwsS3BucketNameNoxnDoubleHyphenPrefix(t *testing.T) {
	assert := assert.New(t)
	cfg := &Config{}
	assert.False(cfg.isValidAwsS3BucketName("xn--test"))
}

func TestAwsS3BucketNameNoHyphens3aliasSuffix(t *testing.T) {
	assert := assert.New(t)
	cfg := &Config{}
	assert.False(cfg.isValidAwsS3BucketName("test-s3alias"))
}

func TestAwsS3BucketNameNoDoubleHyphenOlDashS3Suffix(t *testing.T) {
	assert := assert.New(t)
	cfg := &Config{}
	assert.False(cfg.isValidAwsS3BucketName("test--ol-s3"))
}

func TestAwsS3BucketNameNosthreePrefix(t *testing.T) {
	assert := assert.New(t)
	cfg := &Config{}
	assert.False(cfg.isValidAwsS3BucketName("sthreetest"))
}

func TestAwsS3BucketNameNoIpAddress(t *testing.T) {
	assert := assert.New(t)
	cfg := &Config{}
	assert.False(cfg.isValidAwsS3BucketName("10.199.29.17"))
}
