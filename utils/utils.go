package utils

import (
	"fmt"
	"os"
)

func EnsureProjectExists() {
	initFilename := "bulaba.json"
	if _, err := os.Stat(initFilename); err == nil {
		msg := fmt.Sprintf("Error: Project already initialized with a %s file\n", initFilename)
		BulabaException(msg)
	}
}
