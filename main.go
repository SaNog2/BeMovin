package main

import (
	"context"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/SaNog2/BeMovin/db"
	"github.com/SaNog2/BeMovin/internal/config"
	"github.com/SaNog2/BeMovin/internal/store"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.DBURL)
	if err != nil {
		log.Fatal("Error could not connect to Database: ", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatal("Error could not reach Database: ", err)
	}

	if err := migrate(pool); err != nil {
		log.Fatal("Error running migrations: ", err)
	}

	srv := &server{q: store.New(pool)}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handlePage)
	mux.HandleFunc("POST /trainings", srv.createTraining)
	mux.HandleFunc("GET /trainings", srv.listTrainings)
	mux.HandleFunc("GET /trainings/{id}", srv.getTraining)

	s := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: requireAuth(cfg, mux),
	}
	log.Println("Listening on :" + cfg.Port)
	log.Fatal(s.ListenAndServe())
}

func migrate(pool *pgxpool.Pool) error {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	goose.SetBaseFS(db.Migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(sqlDB, "migrations")
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
