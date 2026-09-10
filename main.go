package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	s := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	mux.HandleFunc("/", handlePage)

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Hitted Server")
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})

	log.Fatal(s.ListenAndServe())
}

func handlePage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(200)
	const page = `<html>
<head></head>
<body>
	<p> Hello from Docker! I'm a Go server. </p>
</body>
</html>
`
	w.Write([]byte(page))
}
