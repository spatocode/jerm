package utils

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/spatocode/jerm/internal/log"
)

func RemoveLocalFile(zipPath string) error {
	err := os.Remove(zipPath)
	if err != nil {
		return err
	}
	return nil
}

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

// GetStdIn gets a stdin prompt from user
func ReadPromptInput(prompt string, input io.Reader) (string, error) {
	if prompt != "" {
		log.PrintInfo(prompt)
	}
	reader := bufio.NewReader(input)
	value, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func GetWorkspaceName() (string, error) {
	workDir, err := os.Getwd()
	if err != nil {
		log.Debug(err.Error())
		return "", err
	}
	splitPath := strings.Split(workDir, "/")
	workspaceName := splitPath[len(splitPath)-1]
	return workspaceName, err
}
