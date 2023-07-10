package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	db "github.com/horriblename/go-web-server/db"
	godotenv "github.com/joho/godotenv"
)

const (
	DEFAULT_DATABASE_FILE            = "/tmp/database.json"
	DEBUG_DATABASE_FILE              = "/tmp/debug-database.json"
	gAccessTokenExpirationInSeconds  = 1 * 60 * 60       // 1 hours
	gRefreshTokenExpirationInSeconds = 60 * 24 * 60 * 60 // 60 days
	gAccessTokIssuer                 = "chirpy-access"
	gRefreshTokIssuer                = "chirpy-refresh"
)

var (
	ErrBadAuthHeader     = errors.New("bad Authorization in header")
	ErrUnauthorizedToken = errors.New("Unauthorized Token")
)

type apiConfig struct {
	fileserverHits int
	db             *db.DB
	jwtSecret      []byte
}

type serverConfig struct {
	databasePath string
	address      string
}

type genericErrorMsg struct {
	Error string `json:"error"`
}

type PostLoginParameters struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginSuccessResponse struct {
	Id           int    `json:"id"`
	Email        string `json:"email"`
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
}

type PostRefreshParameters struct {
	RefreshToken string `json:"token"`
}

type PostRefreshResponse struct {
	Token string `json:"token"`
}

var gProfanity []string = []string{"kerfuffle", "sharbert", "fornax"}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		cfg.fileserverHits += 1
		next.ServeHTTP(w, req)
	})
}

func (cfg *apiConfig) HandleMetricRequest(w http.ResponseWriter, req *http.Request) {
	content := fmt.Sprintf(`<html>
<body>
	<h1>Welcome, Chirpy Admin</h1>
	<p>Chirpy has been visited %d times!</p>
</body>
</html>`, cfg.fileserverHits)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(content))
}

func middlewareCors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	w.WriteHeader(code)
	w.Write([]byte(msg))
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	dat, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("Error marshalling JSON: %s\n", err)
	}

	w.WriteHeader(code)
	w.Header().Set("Content-Type", "application/json")
	w.Write(dat)
}

func (apiCfg apiConfig) handlePostChirp(w http.ResponseWriter, req *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}

	decoder := json.NewDecoder(req.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		// why tf are we sending internal server error??
		respondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters")
		return
	}

	if len(params.Body) > 140 {
		respBody := genericErrorMsg{
			Error: "Chirp is too long",
		}
		respondWithJSON(w, http.StatusBadRequest, respBody)
		return
	}

	filtered, err := profanityFilter(params.Body)
	if err != nil {
		respBody := genericErrorMsg{
			Error: "Internal Server Error",
		}
		respondWithJSON(w, http.StatusInternalServerError, respBody)
		return
	}

	// success response
	chirp, err := apiCfg.db.CreateChirp(filtered)
	if err != nil {
		respBody := genericErrorMsg{
			Error: "Database Error",
		}
		respondWithJSON(w, http.StatusInternalServerError, respBody)
		return
	}

	respondWithJSON(w, 201, chirp)
}

func profanityFilter(input string) (string, error) {
	var err error
	for _, word := range gProfanity {
		input, err = caseInsensitiveReplace(strings.NewReader(input), word, "****")
		if err != nil {
			return input, err
		}
	}

	return input, err
}

func caseInsensitiveReplace(input io.Reader, search, replace string) (string, error) {
	out := strings.Builder{}
	reader := bufio.NewReader(input)
	var s string
	var err error
	for err == nil {
		s, err = reader.ReadString(' ')

		if strings.EqualFold(strings.TrimRight(s, " "), search) {
			out.WriteString(replace)
			if s[len(s)-1] == ' ' {
				out.WriteByte(' ')
			}
		} else {
			out.WriteString(s)
		}
	}

	if err == io.EOF {
		err = nil
	}

	return out.String(), err
}

func (cfg *apiConfig) handlePostLogin(w http.ResponseWriter, req *http.Request) {
	var params = PostLoginParameters{}
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&params)
	if err != nil {
		fmt.Printf("decoding json: %s\n", err)
		respondWithError(w, http.StatusBadRequest, "Couldn't decode parameters")
		return
	}

	user, err := cfg.db.ValidateUser(params.Email, params.Password)
	if err == db.ErrWrongPassword {
		fmt.Printf("user %s failed password check\n", params.Email)
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if err != nil {
		fmt.Printf("validating user: %s\n", err)
		respondWithError(w, http.StatusInternalServerError, "Database Error")
		return
	}

	if user == nil {
		fmt.Printf("BUG: this should be unreachable\n")
		respondWithError(w, http.StatusInternalServerError, "Internal Error")
		return
	}

	accessTokClaims := jwt.RegisteredClaims{
		Issuer:    gAccessTokIssuer,
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(gAccessTokenExpirationInSeconds) * time.Second)),
		Subject:   strconv.Itoa(user.Id),
	}
	refreshTokClaims := jwt.RegisteredClaims{
		Issuer:    gRefreshTokIssuer,
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(gRefreshTokenExpirationInSeconds) * time.Second)),
		Subject:   strconv.Itoa(user.Id),
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessTokClaims)
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshTokClaims)

	accessTokStr, err := accessToken.SignedString(cfg.jwtSecret)
	if err != nil {
		fmt.Printf("signing JWT token: %s\n", err)
		respondWithError(w, http.StatusInternalServerError, "Internal Error")
		return
	}

	refreshTokStr, err := refreshToken.SignedString(cfg.jwtSecret)

	resp := LoginSuccessResponse{
		Id:           user.Id,
		Email:        user.Email,
		Token:        accessTokStr,
		RefreshToken: refreshTokStr,
	}

	respondWithJSON(w, http.StatusOK, resp)
}

func (cfg *apiConfig) handleGetChirps(w http.ResponseWriter, req *http.Request) {
	chirps, err := cfg.db.GetChirps()
	if err != nil {
		fmt.Printf("Getting chirps from DB: %s\n", err)
		respBody := genericErrorMsg{
			Error: "Database Error",
		}
		respondWithJSON(w, http.StatusInternalServerError, respBody)
		return
	}

	respondWithJSON(w, http.StatusOK, chirps)
}

func (cfg *apiConfig) handleGetChirpByID(w http.ResponseWriter, req *http.Request) {
	chirpID := req.Context().Value("chirpID")

	chirps, err := cfg.db.GetChirps()
	if err != nil {
		fmt.Printf("Getting chirps from DB: %s\n", err)
		respBody := struct {
			Error string `json:"error"`
		}{
			Error: "Database Error",
		}
		respondWithJSON(w, http.StatusInternalServerError, respBody)
		return
	}

	// could be optimised but idc
	for _, chirp := range chirps {
		if chirp.Id == chirpID {
			respondWithJSON(w, http.StatusOK, chirp)
			return
		}
	}

	respondWithError(w, http.StatusNotFound, "Chirp not found")
}

// func (cfg *apiConfig) handleGetUsers(w http.ResponseWriter, req *http.Request) {
// 	users, err := cfg.db.GetUsers()
// 	if err != nil {
// 		fmt.Printf("Getting chirps from DB: %s\n", err)
// 		respBody := struct {
// 			Error string `json:"error"`
// 		}{
// 			Error: "Database Error",
// 		}
// 		respondWithJSON(w, http.StatusInternalServerError, respBody)
// 		return
// 	}
//
// 	respondWithJSON(w, http.StatusOK, users)
// }

func (cfg *apiConfig) handlePostUsers(w http.ResponseWriter, req *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	decoder := json.NewDecoder(req.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters")
		return
	}

	user, err := cfg.db.CreateUser(params.Email, params.Password)
	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError, "Database Error")
		return
	}
	respondWithJSON(w, 201, user)
}

func (cfg *apiConfig) handlePutUserById(w http.ResponseWriter, req *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	var params parameters
	token, err := validateJWT(w, req, cfg.jwtSecret)
	if err != nil {
		return
	}

	if iss, err := token.Claims.GetIssuer(); err != nil {
		respondWithError(w, http.StatusBadRequest, "Missing Issuer in Token")
		return
	} else if iss != gAccessTokIssuer {
		respondWithError(w, http.StatusUnauthorized, "Wrong Issuer")
		return
	}

	userIDStr, err := token.Claims.GetSubject()
	if err != nil {
		// TODO: log?
		respondWithError(w, http.StatusBadRequest, "Missing Subject in Token")
		return
	}

	decoder := json.NewDecoder(req.Body)
	err = decoder.Decode(&params)
	if err != nil {
		fmt.Printf("decoding json: %s\n", err)
		respondWithError(w, http.StatusBadRequest, "Couldn't decode parameters")
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Token Subject is not an ID")
		return
	}

	updatedUser, err := cfg.db.UpdateUser(userID, params.Email, params.Password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database Error")
		return
	}

	respondWithJSON(w, http.StatusOK, updatedUser)
}

func (cfg *apiConfig) handlePostRefresh(w http.ResponseWriter, req *http.Request) {
	token, err := validateJWT(w, req, cfg.jwtSecret)
	if err != nil {
		return
	}

	auth := req.Header.Get("Authorization")

	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		respondWithError(w, http.StatusBadRequest, `Malformed "Authorization" in Header`)
		return
	}
	tokStr := strings.TrimPrefix(auth, prefix)
	if err := cfg.db.CheckTokenRevocation(tokStr); err != nil {
		respondWithError(w, http.StatusUnauthorized, "Token Revoked or Database Error")
		return
	}

	if iss, err := token.Claims.GetIssuer(); err != nil {
		respondWithError(w, http.StatusBadRequest, "Token Missing Issuer")
		return
	} else if iss != gRefreshTokIssuer {
		fmt.Printf("request refresh: wrong issuer %s\n", iss)
		respondWithError(w, http.StatusUnauthorized, "Wrong Issuer")
		return
	}

	subject, err := token.Claims.GetSubject()
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Token Missing Subject")
		return
	}

	accessTokClaims := jwt.RegisteredClaims{
		Issuer:    gAccessTokIssuer,
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(gAccessTokenExpirationInSeconds) * time.Second)),
		Subject:   subject,
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessTokClaims)

	accessTokStr, err := accessToken.SignedString(cfg.jwtSecret)
	if err != nil {
		fmt.Printf("signing JWT token: %s\n", err)
		respondWithError(w, http.StatusInternalServerError, "Internal Error")
		return
	}

	resp := PostRefreshResponse{
		Token: accessTokStr,
	}
	respondWithJSON(w, http.StatusOK, resp)
}

func (cfg *apiConfig) handlePostRevoke(w http.ResponseWriter, req *http.Request) {
	token, err := validateJWT(w, req, cfg.jwtSecret)
	if err != nil {
		return
	}

	if iss, err := token.Claims.GetIssuer(); err != nil {
		respondWithError(w, http.StatusBadRequest, "Token Missing Issuer")
		return
	} else if iss != gRefreshTokIssuer {
		fmt.Printf("request refresh: wrong issuer %s\n", iss)
		respondWithError(w, http.StatusUnauthorized, "Wrong Issuer")
		return
	}

	auth := req.Header.Get("Authorization")

	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		respondWithError(w, http.StatusBadRequest, `Malformed "Authorization" in Header`)
		return
	}
	tokStr := strings.TrimPrefix(auth, prefix)

	err = cfg.db.AddTokenRevocation(tokStr)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database Error")
	}

	respondWithJSON(w, http.StatusOK, struct{}{})
}

func chirpCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		chirpIDStr := chi.URLParam(req, "chirpID")
		chirpID, err := strconv.Atoi(chirpIDStr)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Expected an ID")
			return
		}

		ctx := context.WithValue(req.Context(), "chirpID", chirpID)
		next.ServeHTTP(w, req.WithContext(ctx))
	})
}

func apiRouter(cfg *apiConfig) chi.Router {
	router := chi.NewRouter()
	router.Get("/healthz", handleReadinessCheck)
	router.Post("/login", cfg.handlePostLogin)
	router.Route("/chirps", func(r chi.Router) {
		r.Get("/", cfg.handleGetChirps)
		r.Post("/", cfg.handlePostChirp)
		r.With(chirpCtx).Get("/{chirpID}", cfg.handleGetChirpByID)
	})
	router.Route("/users", func(r chi.Router) {
		r.Post("/", cfg.handlePostUsers)
		r.Put("/", cfg.handlePutUserById)
	})
	router.Post("/refresh", cfg.handlePostRefresh)
	router.Post("/revoke", cfg.handlePostRevoke)

	return router
}

func adminRouter(cfg *apiConfig) chi.Router {
	router := chi.NewRouter()

	router.Get("/metrics", cfg.HandleMetricRequest)

	return router
}

func handleReadinessCheck(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("charset", "utf-8")
	w.Write([]byte("OK"))
}

func startServer(serverCfg serverConfig, jwtSecret []byte) error {
	router := chi.NewRouter()

	db, err := db.New(serverCfg.databasePath)
	if err != nil {
		panic(fmt.Sprintf("Creating DB: %s", err))
	}

	apiCfg := apiConfig{db: db, jwtSecret: jwtSecret}
	fileServer := apiCfg.middlewareMetricsInc(http.FileServer(http.Dir(".")))

	// if not using chi
	// do `router = middlewareCors(router)` after defining all endpoints
	router.Use(middlewareCors)

	router.Get("/app/*", http.StripPrefix("/app", fileServer).ServeHTTP)
	router.Get("/app", emptyPath(fileServer).ServeHTTP)
	router.Mount("/api", apiRouter(&apiCfg))
	router.Mount("/admin", adminRouter(&apiCfg))

	server := http.Server{
		Handler: router,
		Addr:    serverCfg.address,
	}

	return server.ListenAndServe()
}

func main() {
	dbg := flag.Bool("debug", false, "Enable debug mode")
	flag.Parse()
	host := flag.Arg(0)

	godotenv.Load()

	serverCfg := serverConfig{
		databasePath: DEFAULT_DATABASE_FILE,
		address:      host,
	}

	if *dbg {
		serverCfg.databasePath = DEBUG_DATABASE_FILE
		_ = os.Remove(serverCfg.databasePath)
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		fmt.Printf("empty JWT_SECRET!\n")
		os.Exit(1)
	}

	err := startServer(serverCfg, []byte(jwtSecret))

	if err != http.ErrServerClosed {
		fmt.Printf("%s\n", err)
		os.Exit(1)
	}
}

// -------
// Helpers
// -------
type rootPath struct {
	next http.Handler
}

func (h *rootPath) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	req.URL.Path = "/"
	h.next.ServeHTTP(w, req)
}

func emptyPath(next http.Handler) http.Handler {
	return &rootPath{
		next: next,
	}
}

// validates the autheticity of a JWT token. If an error occured, w will already be written to and should not be used further
//
// TODO: having w "sometimes" being written to is kinda confusing, should change that
func validateJWT(w http.ResponseWriter, req *http.Request, secret []byte) (*jwt.Token, error) {
	// header format:
	//	  Authorization: Bearer <token>
	auth := req.Header.Get("Authorization")

	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		respondWithError(w, http.StatusBadRequest, `Malformed "Authorization" in Header`)
		return nil, ErrBadAuthHeader
	}
	tokStr := strings.TrimPrefix(auth, prefix)
	claims := jwt.RegisteredClaims{}

	token, err := jwt.ParseWithClaims(tokStr, &claims, func(token *jwt.Token) (interface{}, error) {
		return secret, nil
	})
	if err != nil {
		if err == jwt.ErrSignatureInvalid {
			respondWithError(w, http.StatusUnauthorized, "Unauthorized")
			return nil, ErrUnauthorizedToken
		}
		respondWithError(w, http.StatusBadRequest, "Bad Request")
		return nil, fmt.Errorf("parsing JWT token: %w\n", err)
	}

	if !token.Valid {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return nil, ErrUnauthorizedToken
	}

	return token, nil
}
