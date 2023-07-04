package main

import (
	"net/http"
	"testing"
	"time"
)

// no I don't know how to mock servers properly shut up
func TestServer(t *testing.T) {
	url := "localhost:9000"

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- startServer(url)
	}()

	time.Sleep(500 * time.Millisecond)

	url = "http://" + url
	_, err := http.Get(url + "/app")
	if err != nil {
		t.Errorf(`Getting "/app": %s`, err)
	}

	_, err = http.Get(url + "/app/assets/logo.png")
	if err != nil {
		t.Errorf(`Getting "/app/assets/logo.png": %s`, err)
	}
}
