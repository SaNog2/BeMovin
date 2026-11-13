package main

import (
	"context"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/SaNog2/BeMovin/internal/config"
)

func main() {
	cfg := config.Load()
	conn, err := pgx.Connect(context.Background(), cfg.DBURL)
	if err != nil {
		log.Fatal("Error could not connect to Database")
	}
	defer conn.Close(context.Background())

	mux := http.NewServeMux()
	s := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	mux.HandleFunc("/", handlePage)

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
