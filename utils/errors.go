package utils

import (
	"context"
	"fmt"
	"net/http"
	"os"
)

func BulabaException(message ...any) {
	fmt.Println(message...)
	os.Exit(1)
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
