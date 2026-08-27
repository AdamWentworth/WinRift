package main

import (
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	client := http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://127.0.0.1:8080/healthz")
	if err != nil {
		os.Exit(1)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 16))
	if err != nil || response.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != "ok" {
		os.Exit(1)
	}
}
