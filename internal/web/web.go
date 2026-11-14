// Package web serves the server-rendered HTML pages (templ + htmx).
package web

import (
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/a-h/templ"
	"github.com/jackc/pgx/v5"

	"github.com/SaNog2/BeMovin/internal/auth"
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

	mux.HandleFunc("GET /{$}", h.listPage)
	mux.HandleFunc("GET /trainings/{id}", h.detailPage)
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
	}, nil
}

func (h *Handler) listPage(w http.ResponseWriter, r *http.Request) {
	rows, err := h.q.ListTrainings(r.Context())
	if err != nil {
		h.serverError(w, r, "list trainings", err)
		return
	}

	// Rows come newest first. Upcoming trainings read better soonest
	// first, past ones most recent first.
	today := time.Now().UTC().Truncate(24 * time.Hour)
	var upcoming, past []trainingView
	for _, row := range rows {
		v, err := toView(row)
		if err != nil {
			h.serverError(w, r, "decode training", err)
			return
		}
		if v.Date.Before(today) {
			past = append(past, v)
		} else {
			upcoming = append(upcoming, v)
		}
	}
	slices.Reverse(upcoming)

	render(w, r, http.StatusOK, listPage(upcoming, past, auth.IsAdmin(r.Context())))
}

func (h *Handler) detailPage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		h.notFound(w, r)
		return
	}
	row, err := h.q.GetTraining(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		h.notFound(w, r)
		return
	}
	if err != nil {
		h.serverError(w, r, "get training", err)
		return
	}
	v, err := toView(row)
	if err != nil {
		h.serverError(w, r, "decode training", err)
		return
	}
	render(w, r, http.StatusOK, detailPage(v))
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
