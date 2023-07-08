package utils

import (
	"fmt"
	"os"
)

func BulabaException(message ...string) {
	fmt.Println(message)
	os.Exit(1)
}
