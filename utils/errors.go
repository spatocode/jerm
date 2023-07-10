package utils

import (
	"fmt"
	"os"
)

func BulabaException(message ...any) {
	fmt.Println(message...)
	os.Exit(1)
}
