package utils

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/spatocode/jerm/internal/log"
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

func GetStdIn(prompt string) (string, error) {
	if prompt != "" {
		log.PrintInfo(prompt)
	}
	reader := bufio.NewReader(os.Stdin)
	value, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	fmt.Println()
	return strings.TrimSpace(value), nil
}
