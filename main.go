package main

import (
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	s := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})

	log.Fatal(s.ListenAndServe())
}
