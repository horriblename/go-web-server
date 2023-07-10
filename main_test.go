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
	"github.com/joho/godotenv"
)

var gNoCheck *struct{} = nil

// no I don't know how to mock servers properly shut up
func TestServer(t *testing.T) {
	url := "localhost:9000"

	_ = os.Remove(DEBUG_DATABASE_FILE)

	assertOk := func(err error) {
		if err != nil {
			t.Errorf("%s", err)
			panic("")
		}
	}

	serverErr := make(chan error, 1)
	serverCfg := serverConfig{
		address:      url,
		databasePath: DEBUG_DATABASE_FILE,
	}

	godotenv.Load()
	jwtSecret := os.Getenv("JWT_SECRET")
	go func() {
		serverErr <- startServer(serverCfg, []byte(jwtSecret))
	}()

	time.Sleep(500 * time.Millisecond)

	url = "http://" + url

	assertOk(testHttpRequestString("GET", nil, url+"/api/healthz", nil, http.StatusOK, "OK"))
	assertOk(testHttpRequest("GET", nil, url+"/app", nil, http.StatusOK, gNoCheck))
	assertOk(testHttpRequest("GET", nil, url+"/app/assets/logo.png", nil, http.StatusOK, gNoCheck))

	// TODO check it returns 2
	assertOk(testHttpRequest("GET", nil, url+"/admin/metrics", nil, http.StatusOK, gNoCheck))

	// test /api/chirp

	var req PostChirpRequest
	chirps_url := url + "/api/chirps"
	assertOk(testHttpRequest("GET", nil, chirps_url, nil, http.StatusOK, gNoCheck))

	req = PostChirpRequest{"Hello!"}
	exp1 := "Hello!"
	assertOk(testHttpRequest("POST", nil, chirps_url, req, 201, &db.Chirp{Id: 1, Body: exp1}))

	req = PostChirpRequest{strings.Repeat(".", 141)}
	assertOk(testHttpRequest("POST", nil, chirps_url, req, 400, &genericFailMessage{"Chirp is too long"}))

	req = PostChirpRequest{"This is a keRfUfFle opinion I need to share with the world!"}
	exp2 := "This is a **** opinion I need to share with the world!"
	assertOk(testHttpRequest("POST", nil, chirps_url, req, 201, &db.Chirp{Id: 2, Body: exp2}))

	expect := []db.Chirp{
		{Id: 1, Body: exp1},
		{Id: 2, Body: exp2},
	}
	assertOk(testHttpRequest("GET", nil, chirps_url, nil, http.StatusOK, &expect))

	// get chirp by id
	assertOk(testHttpRequest("GET", nil, url+"/api/chirps/1", nil, http.StatusOK, &expect[0]))
	// TODO check returned result
	assertOk(testHttpRequest("GET", nil, url+"/api/chirps/100", nil, http.StatusNotFound, gNoCheck))

	users_url := url + "/api/users"
	email1 := "x@ymail.com"
	pw1 := "04234"
	req_user := PostUserRequest{email1, pw1}
	// Register User 1
	assertOk(testHttpRequest("POST", nil, users_url, req_user, 201, &db.UserDTO{Id: 1, Email: email1}))

	email2 := "abc@nomail.com"
	pw2 := "10293"
	req_user = PostUserRequest{email2, pw2}
	// Register User 2
	assertOk(testHttpRequest("POST", nil, users_url, req_user, 201, &db.UserDTO{Id: 2, Email: email2}))

	login_url := url + "/api/login"
	req_login := PostUserRequest{email1, pw1}
	var login_resp *LoginSuccessResponse
	var err error
	// Login User 1
	login_resp, err = testHttpWithResponse[LoginSuccessResponse]("POST", nil, login_url, req_login, 200)
	assertOk(err)
	token1 := login_resp.Token

	req_login = PostUserRequest{email2, pw2}
	// Login User 2
	login_resp, err = testHttpWithResponse[LoginSuccessResponse]("POST", nil, login_url, req_login, 200)
	assertOk(err)

	token2 := login_resp.Token

	req_login = PostUserRequest{email2, "wrong password"}
	// Login User With Wrong Password
	assertOk(testHttpRequestString("POST", nil, login_url, req_login, http.StatusUnauthorized, "Unauthorized"))

	type userValidation struct {
		Email string `json:"email"`
	}

	header := map[string]string{
		"Authorization": "Bearer " + token1,
	}
	req_put_users := PostUserRequest{
		Email:    email1,
		Password: "043234",
	}
	// PUT /api/users: change password
	login_resp, err = testHttpWithResponse[LoginSuccessResponse]("PUT", header, users_url, req_put_users, 200)
	assertOk(err)

	header = map[string]string{
		"Authorization": "Bearer " + token2,
	}
	req_put_users = PostUserRequest{
		Email:    "new@email.com",
		Password: pw2,
	}
	// PUT /api/users: change email
	login_resp, err = testHttpWithResponse[LoginSuccessResponse]("PUT", header, users_url, req_put_users, 200)
	assertOk(err)
}

func testHttpRequestString(method string, headers map[string]string, url string, req any, code int, expect string) error {
	resp, err := sendHttpRequest(method, headers, url, req, code)
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

func sendHttpRequest(method string, header map[string]string, url string, req any, code int) (*http.Response, error) {
	var dat []byte
	var err error
	dat, err = json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf(`json.Marshal: %w`, err)
	}
	httpReq, err := http.NewRequest(method, url, bytes.NewReader(dat))
	if err != nil {
		return nil, fmt.Errorf(`http.NewRequest: %w`, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for key, value := range header {
		httpReq.Header.Set(key, value)
	}
	resp, err := (&http.Client{}).Do(httpReq)

	if err != nil {
		return nil, fmt.Errorf(`Posting to %s: %w`, url, err)
	}

	if resp.StatusCode != code {
		// debug
		if content, err := io.ReadAll(resp.Body); err == nil {
			defer fmt.Printf("body: %s\n", string(content))
		} else {
			defer fmt.Printf("error reading response body\n")
		}
		return nil, fmt.Errorf("Expected status code %d, got %d", code, resp.StatusCode)
	}

	return resp, nil
}

func testHttpRequest[T any](method string, headers map[string]string, url string, req any, code int, expect *T) error {
	resp, err := sendHttpRequest(method, headers, url, req, code)
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

func testHttpWithResponse[T any](method string, headers map[string]string, url string, req any, code int) (*T, error) {
	resp, err := sendHttpRequest(method, headers, url, req, code)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var got T

	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&got)

	if err != nil {
		return nil, fmt.Errorf(`Decoding response: %w`, err)
	}

	return &got, nil
}
