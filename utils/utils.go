package utils

import (
	"fmt"
	"math/rand"
	"os"
)

func EnsureProjectExists() {
	initFilename := "bulaba.json"
	if _, err := os.Stat(initFilename); err == nil {
		msg := fmt.Sprintf("Error: Project already initialized with a %s file\n", initFilename)
		BulabaException(msg)
	}
}

func GenerateRandomNumber() int {
	low := 10000000
	high := 99999999
	return low + rand.Intn(high-low)
}
