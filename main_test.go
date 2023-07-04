package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
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

	testGet(t, url+"/healthz", http.StatusOK)
	testGet(t, url+"/app", http.StatusOK)
	testGet(t, url+"/app/assets/logo.png", http.StatusOK)

	// TODO check it returns 2
	testGet(t, url+"/admin/metrics", http.StatusOK)

	// test /api/validate_chirp
	var req ValidateChirpRequest
	req = ValidateChirpRequest{"Hello!"}
	testValidateChirp(t, url, req, 200, ValidateChirpSuccess{req.Body})

	req = ValidateChirpRequest{strings.Repeat(".", 141)}
	testValidateChirp(t, url, req, 400, ValidateChirpFail{"Chirp is too long"})

	req = ValidateChirpRequest{"This is a keRfUfFle opinion I need to share with the world!"}
	testValidateChirp(t, url, req, 200, ValidateChirpSuccess{"This is a **** opinion I need to share with the world!"})
}

func testGet(t *testing.T, url string, code int) {
	resp, err := http.Get(url)
	if err != nil {
		t.Errorf(`Getting "%s": %s`, url, err)
		return
	}
	if resp.StatusCode != code {
		t.Errorf(`Getting "%s": got status %s, expected %d`, url, resp.Status, code)
		return
	}

	resp.Body.Close()
}

type ValidateChirpRequest struct {
	Body string `json:"body"`
}
type ValidateChirpSuccess struct {
	CleanedBody string `json:"cleaned_body"`
}
type ValidateChirpFail struct {
	Error string `json:"error"`
}

func testValidateChirp[T any](t *testing.T, base_url string, req ValidateChirpRequest, code int, expect T) {
	var dat []byte
	var err error
	dat, err = json.Marshal(req)
	if err != nil {
		panic("something went wrong")
	}
	resp, err := http.Post(base_url+"/api/validate_chirp", "application/json", bytes.NewReader(dat))

	if err != nil {
		t.Errorf(`Posting /api/validate_chirp: %s`, err)
		return
	}

	if resp.StatusCode != code {
		t.Errorf("Expected status code %d, got %d", code, resp.StatusCode)
	}

	got := expect

	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&got)

	if err != nil {
		t.Errorf(`Decoding response: %s`, err)
		return
	}

	if !reflect.DeepEqual(got, expect) {
		t.Errorf(`Expected %+v\nGot %+v`, expect, got)
	}
}
