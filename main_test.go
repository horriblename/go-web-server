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
	accToken1 := login_resp.Token
	refreshToken1 := login_resp.RefreshToken

	req_login = PostUserRequest{email2, pw2}
	// Login User 2
	login_resp, err = testHttpWithResponse[LoginSuccessResponse]("POST", nil, login_url, req_login, 200)
	assertOk(err)

	accToken2 := login_resp.Token
	// refreshToken2 := login_resp.RefreshToken

	req_login = PostUserRequest{email2, "wrong password"}
	// Login User With Wrong Password
	assertOk(testHttpRequestString("POST", nil, login_url, req_login, http.StatusUnauthorized, "Unauthorized"))

	// test /api/chirp
	var req_post_chirp PostChirpRequest
	chirps_url := url + "/api/chirps"
	assertOk(testHttpRequest("GET", nil, chirps_url, nil, http.StatusOK, gNoCheck))

	req_post_chirp = PostChirpRequest{"Hello!"}
	header := newAuthenticatedHeader(accToken1)
	chirp1 := db.Chirp{Id: 1, AuthorID: 1, Body: "Hello!"}
	assertOk(testHttpRequest("POST", header, chirps_url, req_post_chirp, 201, &chirp1))

	req_post_chirp = PostChirpRequest{strings.Repeat(".", 141)}
	header = newAuthenticatedHeader(accToken1)
	assertOk(testHttpRequest("POST", header, chirps_url, req_post_chirp, 400, &genericFailMessage{"Chirp is too long"}))

	req_post_chirp = PostChirpRequest{"This is a keRfUfFle opinion I need to share with the world!"}
	chirp2 := db.Chirp{Id: 2, AuthorID: 2, Body: "This is a **** opinion I need to share with the world!"}
	header = newAuthenticatedHeader(accToken2)
	assertOk(testHttpRequest("POST", header, chirps_url, req_post_chirp, 201, &chirp2))

	req_post_chirp = PostChirpRequest{"Posting without logging in"}
	testHttpRequest("POST", nil, chirps_url, req_post_chirp, http.StatusUnauthorized, gNoCheck)

	expect := []db.Chirp{chirp1, chirp2}
	assertOk(testHttpRequest("GET", nil, chirps_url, nil, http.StatusOK, &expect))

	// get chirp by id
	assertOk(testHttpRequest("GET", nil, url+"/api/chirps/1", nil, http.StatusOK, &expect[0]))
	// TODO check returned result
	assertOk(testHttpRequest("GET", nil, url+"/api/chirps/100", nil, http.StatusNotFound, gNoCheck))

	header = newAuthenticatedHeader(accToken1)
	pw1 = "043234"
	req_put_users := PostUserRequest{
		Email:    email1,
		Password: pw1,
	}
	// PUT /api/users: change password
	_, err = testHttpWithResponse[LoginSuccessResponse]("PUT", header, users_url, req_put_users, 200)
	assertOk(err)

	header = newAuthenticatedHeader(accToken2)
	email2 = "new@email.com"
	req_put_users = PostUserRequest{
		Email:    email2,
		Password: pw2,
	}
	// PUT /api/users: change email
	_, err = testHttpWithResponse[LoginSuccessResponse]("PUT", header, users_url, req_put_users, 200)
	assertOk(err)

	refresh_url := url + "/api/refresh"
	empty_req := struct{}{}
	header = newAuthenticatedHeader(refreshToken1)
	// refresh token
	_, err = testHttpWithResponse[PostRefreshResponse]("POST", header, refresh_url, empty_req, 200)
	assertOk(err)

	header = newAuthenticatedHeader(accToken2)
	// refresh token reject wrong token
	assertOk(testHttpRequest("POST", header, refresh_url, empty_req, http.StatusUnauthorized, gNoCheck))

	revoke_url := url + "/api/revoke"
	header = newAuthenticatedHeader(refreshToken1)
	// revoke refresh token of user 1
	assertOk(testHttpRequest("POST", header, revoke_url, empty_req, http.StatusOK, gNoCheck))
	assertOk(testHttpRequest("POST", header, refresh_url, empty_req, http.StatusUnauthorized, gNoCheck))

	// DELETE /api/chirps/{id}
	header = newAuthenticatedHeader(accToken1)
	assertOk(testHttpRequest("DELETE", header, chirps_url+"/1", struct{}{}, http.StatusOK, gNoCheck))

	header = newAuthenticatedHeader(accToken1)
	assertOk(testHttpRequest("DELETE", header, chirps_url+"/1", struct{}{}, http.StatusNotFound, gNoCheck))
	header = newAuthenticatedHeader(accToken1)
	assertOk(testHttpRequest("DELETE", header, chirps_url+"/2", struct{}{}, 403, gNoCheck))

	polka_webhooks_url := url + "/api/polka/webhooks"
	webhook_req := PostPolkaWebhooksParameters{
		Event: "user.upgraded",
		Data: struct {
			UserID int `json:"user_id"`
		}{2},
	}
	req_login = PostUserRequest{email2, pw2}
	// POST /api/polka/webhooks
	assertOk(testHttpRequest("POST", nil, polka_webhooks_url, webhook_req, 200, gNoCheck))
	resp, err := testHttpWithResponse[LoginSuccessResponse]("POST", nil, login_url, req_login, 200)
	assertOk(err)
	if !resp.IsChirpyRed {
		t.Errorf("expected user to have chirpy red")
	}

	webhook_req = PostPolkaWebhooksParameters{
		Event: "user.upgraded",
		Data: struct {
			UserID int `json:"user_id"`
		}{100},
	}
	// POST /api/polka/webhooks send non-existent user id
	assertOk(testHttpRequest("POST", nil, polka_webhooks_url, webhook_req, 404, gNoCheck))
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

func newAuthenticatedHeader(jwt_token string) map[string]string {
	return map[string]string{
		"Authorization": "Bearer " + jwt_token,
	}
}
