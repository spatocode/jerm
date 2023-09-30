package log

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInfoLog(t *testing.T) {
	assert := assert.New(t)

	r, w, stdout := pipeStd(t)
	Info("testing")
	w.Close()
	out, _ := io.ReadAll(r)
	os.Stdout = stdout

	assert.Contains(string(out), "INFO testing\n")
}

func TestDebugLog(t *testing.T) {
	assert := assert.New(t)

	r, w, stdout := pipeStd(t)
	Debug("testing")
	w.Close()
	out, _ := io.ReadAll(r)
	os.Stdout = stdout
	assert.NotContains(string(out), "DEBUG testing\n")

	os.Setenv("JERM_VERBOSE", "1")
	r, w, stdout = pipeStd(t)
	Debug("testing")
	w.Close()
	out, _ = io.ReadAll(r)
	os.Stdout = stdout
	assert.Contains(string(out), "DEBUG testing\n")
}

func TestWarnLog(t *testing.T) {
	assert := assert.New(t)

	r, w, stdout := pipeStd(t)
	Warn("testing")
	w.Close()
	out, _ := io.ReadAll(r)
	os.Stdout = stdout

	assert.Contains(string(out), "WARN testing\n")
}

func TestErrorLog(t *testing.T) {
	assert := assert.New(t)

	r, w, stdout := pipeStd(t)
	Error("testing")
	w.Close()
	out, _ := io.ReadAll(r)
	os.Stdout = stdout

	assert.Contains(string(out), "ERROR testing\n")
}

func pipeStd(t *testing.T) (r, w, stdout *os.File) {
	stdout = os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	return
}
