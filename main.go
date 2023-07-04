package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type apiConfig struct {
	fileserverHits int
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		cfg.fileserverHits += 1
		next.ServeHTTP(w, req)
	})
}

func (cfg *apiConfig) HandleMetricRequest(w http.ResponseWriter, req *http.Request) {
	w.Write([]byte("Hits: " + strconv.Itoa(cfg.fileserverHits)))
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

func apiRouter(cfg *apiConfig) chi.Router {
	router := chi.NewRouter()
	router.Get("/healthz", handleReadinessCheck)
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
	router.Mount("/api", apiRouter(&apiCfg))

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
