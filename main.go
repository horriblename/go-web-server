package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
)

type apiConfig struct {
	fileserverHits int
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
		fmt.Printf("Error marshalling JSON: %s", err)
	}

	w.WriteHeader(code)
	w.Header().Set("Content-Type", "application/json")
	w.Write(dat)
}

func handleValidateChirp(w http.ResponseWriter, req *http.Request) {
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

	type successMsg struct {
		CleanedBody string `json:"cleaned_body"`
	}

	type failMsg struct {
		Error string `json:"error"`
	}

	if len(params.Body) > 140 {
		respBody := failMsg{
			Error: "Chirp is too long",
		}
		respondWithJSON(w, http.StatusBadRequest, respBody)
		return
	}

	filtered, err := profanityFilter(params.Body)
	if err != nil {
		respBody := failMsg{
			Error: "Internal Server Error",
		}
		respondWithJSON(w, http.StatusInternalServerError, respBody)
		return
	}

	// success response
	respBody := successMsg{
		CleanedBody: filtered,
	}
	respondWithJSON(w, http.StatusOK, respBody)
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

func apiRouter() chi.Router {
	router := chi.NewRouter()
	router.Get("/healthz", handleReadinessCheck)
	router.Post("/validate_chirp", handleValidateChirp)

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

func startServer(host string) error {
	router := chi.NewRouter()

	apiCfg := apiConfig{}
	fileServer := apiCfg.middlewareMetricsInc(http.FileServer(http.Dir(".")))

	// if not using chi
	// do `router = middlewareCors(router)` after defining all endpoints
	router.Use(middlewareCors)

	router.Get("/app/*", http.StripPrefix("/app", fileServer).ServeHTTP)
	router.Get("/app", emptyPath(fileServer).ServeHTTP)
	router.Mount("/api", apiRouter())
	router.Mount("/admin", adminRouter(&apiCfg))

	server := http.Server{
		Handler: router,
		Addr:    host,
	}

	return server.ListenAndServe()
}

func main() {
	args := os.Args
	var host string
	if len(args) > 1 {
		host = args[1]
	}

	err := startServer(host)

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
