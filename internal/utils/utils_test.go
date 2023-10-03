package utils

import (
	"os"
	"os/exec"
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

func TestRunCommandWith(t *testing.T) {
	assert := assert.New(t)
	ce := cmdExecutor{cmd: fakeExecCommand}
	out, err := ce.RunCommand("test", "arg")
	assert.Nil(err)
	out = strings.Split(out, "\n")[0]
	assert.Equal("PASS", out)
}

func TestRunCommandWithEnv(t *testing.T) {
	assert := assert.New(t)
	ce := cmdExecutor{cmd: fakeExecCommand}
	out, err := ce.RunCommandWithEnv([]string{"JERM_ENV"}, "test", "arg")
	assert.Nil(err)
	out = strings.Split(out, "\n")[0]
	assert.Equal("PASS", out)
}

func TestRemoveLocalFile(t *testing.T) {
	assert := assert.New(t)
	file := "../../assets/test.whl"
	helperCreateFile(t, file)
	err := RemoveLocalFile(file)
	assert.Nil(err)
	assert.False(FileExists(file))
}

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func helperCreateFile(t *testing.T, file string) {
	_, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
}
