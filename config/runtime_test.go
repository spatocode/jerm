package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spatocode/jerm/internal/utils"
	"github.com/stretchr/testify/assert"
)

var (
	fakeOutput = "fakeoutput"
)

type fakeCommandExecutor struct {
}

func (c fakeCommandExecutor) RunCommand(command string, args ...string) (string, error) {
	if fakeOutput == "" {
		return "", fmt.Errorf("fake err")
	}
	return fakeOutput, nil
}

func (c fakeCommandExecutor) RunCommandWithEnv(env []string, command string, args ...string) (string, error) {
	if fakeOutput == "" {
		return "", fmt.Errorf("fake err")
	}
	return fakeOutput, nil
}

func TestIgnoredFilesWhileCopying(t *testing.T) {
	assert := assert.New(t)
	jermJson := "../assets/jerm.json"
	jermIgnore := "../assets/.jermignore"

	pr := NewPythonRuntime(fakeCommandExecutor{})
	p := pr.(*Python)
	err := p.copyNecessaryFilesToPackageDir("../assets/tests", "../assets", "../assets/tests/.jermignore")
	testfile1Exists := utils.FileExists("../assets/testfile1")
	testfile2Exists := utils.FileExists("../assets/testfile2")
	jermJsonExists := utils.FileExists(jermJson)
	jermIgnoreExists := utils.FileExists(jermIgnore)

	assert.Nil(err)
	assert.False(testfile1Exists)
	assert.False(testfile2Exists)
	assert.True(jermJsonExists)
	assert.True(jermIgnoreExists)

	helperCleanup(t, []string{jermJson, jermIgnore})
}

func TestRuntimeBuild(t *testing.T) {
	assert := assert.New(t)

	cfg := &Config{Name: "test", Stage: "env", Dir: "../assets/tests"}
	ri := NewRuntime()
	r := ri.(*Runtime)
	pkgDir, f, err := r.Build(cfg)

	testfile1 := fmt.Sprintf("%s/testfile1", pkgDir)
	testfile2 := fmt.Sprintf("%s/testfile2", pkgDir)
	jermIgnore := fmt.Sprintf("%s/.jermignore", pkgDir)
	jermJson := fmt.Sprintf("%s/jerm.json", pkgDir)

	assert.Nil(err)
	assert.Contains(pkgDir, "jerm-package")
	assert.Equal("", f)
	assert.True(utils.FileExists(testfile1))
	assert.True(utils.FileExists(testfile2))
	assert.True(utils.FileExists(jermJson))
	assert.True(utils.FileExists(jermIgnore))
}

func TestRuntimeCreateFunctionHandler(t *testing.T) {
	assert := assert.New(t)

	cfg := &Config{Name: "test", Stage: "env", Dir: "../assets/tests"}
	ri := NewRuntime()
	r := ri.(*Runtime)

	handlerFile := filepath.Join("../assets/tests", "index.js")
	handler, err := r.createFunctionHandler(cfg, handlerFile)

	assert.Nil(err)
	assert.Equal("index.handler", handler)
	helperCleanup(t, []string{handlerFile})
}

func TestNewRuntime(t *testing.T) {
	assert := assert.New(t)
	requirementsTxt := "requirements.txt"
	mainGo := "main.go"
	indexHtml := "index.html"
	packageJson := "package.json"

	ri := NewRuntime()
	r := ri.(*Runtime)
	assert.Equal(RuntimeUnknown, r.Name)

	helperCreateFile(t, requirementsTxt)
	ri = NewRuntime()
	p := ri.(*Python)
	assert.Equal(RuntimePython, p.Name)
	helperCleanup(t, []string{requirementsTxt})

	helperCreateFile(t, packageJson)
	ri = NewRuntime()
	n := ri.(*Node)
	assert.Equal(RuntimeNode, n.Name)
	helperCleanup(t, []string{packageJson})

	helperCreateFile(t, mainGo)
	ri = NewRuntime()
	g := ri.(*Go)
	assert.Equal(RuntimeGo, g.Name)
	helperCleanup(t, []string{mainGo})

	helperCreateFile(t, indexHtml)
	ri = NewRuntime()
	r = ri.(*Runtime)
	assert.Equal(RuntimeStatic, r.Name)
	helperCleanup(t, []string{indexHtml})
}

func helperCleanup(t *testing.T, files []string) {
	for _, file := range files {
		err := os.Remove(file)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func helperCreateFile(t *testing.T, file string) {
	_, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
}
