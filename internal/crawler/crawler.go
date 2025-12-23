package crawler

import (
	"io"
	"net/http"
	"time"
)

var defaultHTTPClient = &http.Client{Timeout: 60 * time.Second}

func Fetch(url string) (string, error) {
	resp, err := defaultHTTPClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	return string(b), err
}
