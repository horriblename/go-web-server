package main

import (
	"io"
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

	var resp *http.Response
	var err error
	resp, err = http.Get(url + "/app")
	if err != nil {
		t.Errorf(`Getting "/app": %s`, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf(`Getting "/app": got status %s`, resp.Status)
	}
	resp.Body.Close()

	resp, err = http.Get(url + "/app/assets/logo.png")
	if err != nil {
		t.Errorf(`Getting "/app/assets/logo.png": %s`, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf(`Getting "/app/assets/logo.png": got status %s`, resp.Status)
	}
	resp.Body.Close()

	// Test metric middleware
	resp, err = http.Get(url + "/admin/metrics")
	if err != nil {
		t.Errorf(`Getting "/admin/metrics": %s`, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf(`Getting "/admin/metrics": got status %s`, resp.Status)
	} else {
		_, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Errorf(`Reading Response Body of "/admin/metrics": %s`, err)
		}
		// if string(buf) != "Hits: 2" {
		// 	t.Errorf(`Expected 2 from "/admin/metrics", got %s`, string(buf))
		// }
	}
	resp.Body.Close()
}
