package main

import (
	"fmt"
	"net/http"
	"os"
)

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

func handleNotFound(w http.ResponseWriter, req *http.Request) {
	http.NotFound(w, req)
}

func handleRoot(w http.ResponseWriter, req *http.Request) {

}

func main() {
	args := os.Args
	var host string
	if len(args) > 1 {
		host = args[1]
	}

	mux := http.NewServeMux()
	// mux.HandleFunc("", handleNotFound)
	mux.Handle("/", http.FileServer(http.Dir(".")))
	corsMux := middlewareCors(mux)

	server := http.Server{
		Handler: corsMux,
		Addr:    host,
	}

	err := server.ListenAndServe()

	if err != http.ErrServerClosed {
		fmt.Printf("%s\n", err)
		os.Exit(1)
	}
}
