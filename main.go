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

func main() {
	args := os.Args
	var host string
	if len(args) > 1 {
		host = args[1]
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleNotFound)
	corsMux := middlewareCors(mux)

	server := http.Server{
		Handler: corsMux,
		Addr:    host,
	}

	err := server.ListenAndServe()

	fmt.Printf("%s\n", err)
}
