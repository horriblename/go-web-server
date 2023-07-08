package main

import (
	"bytes"
	"encoding/json"
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

var gNoCheck *struct{} = nil

// no I don't know how to mock servers properly shut up
func TestServer(t *testing.T) {
	url := "localhost:9000"

	_ = os.Remove(DEBUG_DATABASE_FILE)

	assertOk := func(err error) {
		if err != nil {
			t.Errorf("%s", err)
		}
	}

	serverErr := make(chan error, 1)
	serverCfg := serverConfig{
		address:      url,
		databasePath: DEBUG_DATABASE_FILE,
	}
	go func() {
		serverErr <- startServer(serverCfg)
	}()

	time.Sleep(500 * time.Millisecond)

	url = "http://" + url

	assertOk(testHttpRequestString("GET", url+"/api/healthz", nil, http.StatusOK, "OK"))
	assertOk(testHttpRequest("GET", url+"/app", nil, http.StatusOK, gNoCheck))
	assertOk(testHttpRequest("GET", url+"/app/assets/logo.png", nil, http.StatusOK, gNoCheck))

	// TODO check it returns 2
	assertOk(testHttpRequest("GET", url+"/admin/metrics", nil, http.StatusOK, gNoCheck))

	// test /api/chirp

	var req PostChirpRequest
	chirps_url := url + "/api/chirps"
	assertOk(testHttpRequest("GET", chirps_url, nil, http.StatusOK, gNoCheck))

	req = PostChirpRequest{"Hello!"}
	exp1 := "Hello!"
	assertOk(testHttpRequest("POST", chirps_url, req, 201, &db.Chirp{Id: 1, Body: exp1}))

	req = PostChirpRequest{strings.Repeat(".", 141)}
	assertOk(testHttpRequest("POST", chirps_url, req, 400, &genericFailMessage{"Chirp is too long"}))

	req = PostChirpRequest{"This is a keRfUfFle opinion I need to share with the world!"}
	exp2 := "This is a **** opinion I need to share with the world!"
	assertOk(testHttpRequest("POST", chirps_url, req, 201, &db.Chirp{Id: 2, Body: exp2}))

	expect := []db.Chirp{
		{Id: 1, Body: exp1},
		{Id: 2, Body: exp2},
	}
	assertOk(testHttpRequest("GET", chirps_url, nil, http.StatusOK, &expect))

	// get chirp by id
	assertOk(testHttpRequest("GET", url+"/api/chirps/1", nil, http.StatusOK, &expect[0]))
	// TODO check returned result
	assertOk(testHttpRequest("GET", url+"/api/chirps/100", nil, http.StatusNotFound, gNoCheck))

	users_url := url + "/api/users"
	exp1 = "x@ymail.com"
	pw1 := "04234"
	req_user := PostUserRequest{exp1, pw1}
	assertOk(testHttpRequest("POST", users_url, req_user, 201, &db.UserDTO{Id: 1, Email: exp1}))

	exp2 = "abc@nomail.com"
	pw2 := "10293"
	req_user = PostUserRequest{exp2, pw2}
	assertOk(testHttpRequest("POST", users_url, req_user, 201, &db.UserDTO{Id: 2, Email: exp2}))

	login_url := url + "/api/login"
	req_login := PostUserRequest{exp1, pw1}
	assertOk(testHttpRequest("POST", login_url, req_login, 200, &db.UserDTO{Id: 1, Email: exp1}))

	req_login = PostUserRequest{exp2, pw2}
	assertOk(testHttpRequest("POST", login_url, req_login, 200, &db.UserDTO{Id: 2, Email: exp2}))

	req_login = PostUserRequest{exp2, "wrong password"}
	var nocheck *struct{} = nil
	assertOk(testHttpRequest("POST", login_url, req_login, http.StatusUnauthorized, nocheck))
}

func testHttpRequestString(method string, url string, req any, code int, expect string) error {
	resp, err := sendHttpRequest(method, url, req, code)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	got, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Reading response body: %s", err)
	}
	if string(got) != expect {
		return fmt.Errorf(`Expected %s, got %s`, expect, got)
	}

	return nil
}

type PostChirpRequest struct {
	Body string `json:"body"`
}
type genericFailMessage struct {
	Error string `json:"error"`
}
type PostUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func sendHttpRequest(method string, url string, req any, code int) (*http.Response, error) {
	var dat []byte
	var err error
	dat, err = json.Marshal(req)
	if err != nil {
		panic("something went wrong")
	}
	httpReq, err := http.NewRequest(method, url, bytes.NewReader(dat))
	if err != nil {
		return nil, fmt.Errorf(`http.NewRequest: %s`, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{}).Do(httpReq)

	if err != nil {
		return nil, fmt.Errorf(`Posting to %s: %w`, url, err)
	}

	if resp.StatusCode != code {
		return nil, fmt.Errorf("Expected status code %d, got %d", code, resp.StatusCode)
	}

	return resp, nil
}

func testHttpRequest[T any](method string, url string, req any, code int, expect *T) error {
	resp, err := sendHttpRequest(method, url, req, code)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if expect == nil {
		return nil
	}

	var got T

	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&got)

	if err != nil {
		return fmt.Errorf(`Decoding response: %w`, err)
	}

	if !reflect.DeepEqual(got, *expect) {
		return fmt.Errorf(`Expected %+v\nGot %+v`, *expect, got)
	}

	return nil
}
