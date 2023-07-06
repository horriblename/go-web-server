package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/horriblename/go-web-server/db"
)

// no I don't know how to mock servers properly shut up
func TestServer(t *testing.T) {
	url := "localhost:9000"

	_ = os.Remove(gDatabasePath)

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- startServer(url)
	}()

	time.Sleep(500 * time.Millisecond)

	url = "http://" + url

	testGetString(t, url+"/api/healthz", http.StatusOK, "OK")
	testGet(t, url+"/app", http.StatusOK)
	testGet(t, url+"/app/assets/logo.png", http.StatusOK)

	// TODO check it returns 2
	testGet(t, url+"/admin/metrics", http.StatusOK)

	// test /api/chirp

	var req PostChirpRequest
	chirps_url := url + "/api/chirps"
	testGet(t, chirps_url, http.StatusOK)

	req = PostChirpRequest{"Hello!"}
	exp1 := "Hello!"
	testPost(t, chirps_url, req, 201, db.Chirp{Id: 1, Body: exp1})

	req = PostChirpRequest{strings.Repeat(".", 141)}
	testPost(t, chirps_url, req, 400, genericFailMessage{"Chirp is too long"})

	req = PostChirpRequest{"This is a keRfUfFle opinion I need to share with the world!"}
	exp2 := "This is a **** opinion I need to share with the world!"
	testPost(t, chirps_url, req, 201, db.Chirp{Id: 2, Body: exp2})

	expect := []db.Chirp{
		{Id: 1, Body: exp1},
		{Id: 2, Body: exp2},
	}
	testGetJson(t, chirps_url, http.StatusOK, expect)

	// get chirp by id
	testGetJson(t, url+"/api/chirps/1", http.StatusOK, expect[0])
	// TODO check returned result
	testGet(t, url+"/api/chirps/100", http.StatusNotFound)

	users_url := url + "/api/users"
	exp1 = "x@ymail.com"
	req_user := PostUserRequest{exp1}
	testPost(t, users_url, req_user, 201, db.User{Id: 1, Email: exp1})

	exp2 = "abc@nomail.com"
	req_user = PostUserRequest{exp2}
	testPost(t, users_url, req_user, 201, db.User{Id: 2, Email: exp2})
}

// The returned body must be `Close`d after use
func httpGet(url string, code int) (body io.ReadCloser, err error) {
	resp, err := http.Get(url)
	if err != nil {
		resp.Body.Close()
		return nil, err
	}

	if resp.StatusCode != code {
		resp.Body.Close()
		return nil, errors.New(fmt.Sprintf(`got status %s, expected %d`, resp.Status, code))
	}

	return resp.Body, nil
}

func testGet(t *testing.T, url string, code int) {
	body, err := httpGet(url, code)
	if err != nil {
		t.Errorf("Getting: %s", err)
		return
	}
	body.Close()
}

func testGetString(t *testing.T, url string, code int, expect string) {
	body, err := httpGet(url, code)
	if err != nil {
		t.Errorf("Getting: %s", err)
		return
	}
	defer body.Close()

	got, err := io.ReadAll(body)
	if err != nil {
		t.Errorf("Reading response body: %s", err)
		return
	}
	if string(got) != expect {
		t.Errorf(`Expected %s, got %s`, expect, got)
		return
	}
}

func testGetJson[T any](t *testing.T, url string, code int, expect T) {
	body, err := httpGet(url, code)
	if err != nil {
		t.Errorf("Getting: %s", err)
		return
	}
	defer body.Close()

	var got T
	decoder := json.NewDecoder(body)
	err = decoder.Decode(&got)

	if err != nil {
		t.Errorf(`Decoding json: %s`, err)
		return
	}

	if !reflect.DeepEqual(got, expect) {
		t.Errorf(`Expected %+v\nGot %+v`, expect, got)
	}
}

type PostChirpRequest struct {
	Body string `json:"body"`
}
type genericFailMessage struct {
	Error string `json:"error"`
}
type PostUserRequest struct {
	Email string `json:"email"`
}

func testPost[T, U any](t *testing.T, url string, req U, code int, expect T) {
	var dat []byte
	var err error
	dat, err = json.Marshal(req)
	if err != nil {
		panic("something went wrong")
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(dat))

	if err != nil {
		t.Errorf(`Posting /api/chirp: %s`, err)
		return
	}

	if resp.StatusCode != code {
		t.Errorf("Expected status code %d, got %d", code, resp.StatusCode)
		return
	}

	var got T

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
