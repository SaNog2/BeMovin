// Package web serves the server-rendered HTML pages (templ + htmx).
package web

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/a-h/templ"

	"github.com/SaNog2/BeMovin/internal/store"
	"github.com/SaNog2/BeMovin/internal/training"
)

//go:embed static
var staticFS embed.FS

type Handler struct {
	q *store.Queries
}

func New(q *store.Queries) *Handler {
	return &Handler{q: q}
}

// Register adds the HTML and static routes to mux.
func (h *Handler) Register(mux *http.ServeMux) {
	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))

	mux.HandleFunc("GET /{$}", h.homePage)
	mux.HandleFunc("GET /weeks/{date}", h.weekPage)
	mux.HandleFunc("GET /trainings/new", h.newTrainingPage)
	mux.HandleFunc("POST /trainings/new", h.createTraining)
	mux.HandleFunc("GET /trainings/{id}", h.trainingPage)
	// The viewer can post to these too (see ViewerCanWrite).
	mux.HandleFunc("POST /trainings/{id}/completion", h.saveCompletion)
	mux.HandleFunc("POST /trainings/{id}/comments", h.createComment)
	mux.HandleFunc("POST /trainings/{id}/comments/{cid}/delete", h.deleteComment)
	mux.HandleFunc("/", h.notFound)
}

// trainingView is a stored training decoded for rendering.
type trainingView struct {
	ID      int64
	Date    time.Time
	Title   string
	Notes   string
	Blocks  []training.Block
	Summary training.Summary

	Done           bool
	CompletedAt    time.Time
	Feeling        int // 0 = not rated, else 1-5
	CompletionNote string
}

func toView(t store.Training) (trainingView, error) {
	var blocks []training.Block
	if err := json.Unmarshal(t.Blocks, &blocks); err != nil {
		return trainingView{}, err
	}
	return trainingView{
		ID:      t.ID,
		Date:    t.Date.Time,
		Title:   t.Title,
		Notes:   t.Notes,
		Blocks:  blocks,
		Summary: training.Summarize(blocks),

		Done:           t.CompletedAt.Valid,
		CompletedAt:    t.CompletedAt.Time,
		Feeling:        int(t.Feeling.Int16),
		CompletionNote: t.CompletionNote,
	}, nil
}

func (h *Handler) notFound(w http.ResponseWriter, r *http.Request) {
	render(w, r, http.StatusNotFound, messagePage("Not found", "This page doesn't exist."))
}

func (h *Handler) serverError(w http.ResponseWriter, r *http.Request, op string, err error) {
	log.Printf("%s: %v", op, err)
	render(w, r, http.StatusInternalServerError, messagePage("Something went wrong", "Please try again later."))
}

func render(w http.ResponseWriter, r *http.Request, status int, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := c.Render(r.Context(), w); err != nil {
		log.Println("render:", err)
	}
}
