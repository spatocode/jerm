package utils

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"

	"github.com/fatih/color"
)

func FileExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}

func Request(location string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(context.TODO(),
		http.MethodGet, location, nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	return res, err
}

func GetShellCommandOutput(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	out, err := cmd.Output()
	return string(out), err
}

func LogError(msg string, v ...interface{})  {
	color.Red(fmt.Sprintf(msg, v...))
}

func LogInfo(msg string, v ...interface{})  {
	color.Cyan(fmt.Sprintf(msg, v...))
}

func LogWarn(msg string, v ...interface{})  {
	color.Yellow(fmt.Sprintf(msg, v...))
}
