// Package api serves the JSON API under /api.
package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/SaNog2/BeMovin/internal/store"
	"github.com/SaNog2/BeMovin/internal/training"
)

const maxBodyBytes = 1 << 20 // 1 MiB

type Handler struct {
	q *store.Queries
}

func New(q *store.Queries) *Handler {
	return &Handler{q: q}
}

// Register adds the API routes to mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/trainings", h.createTraining)
	mux.HandleFunc("GET /api/trainings", h.listTrainings)
	mux.HandleFunc("GET /api/trainings/{id}", h.getTraining)
}

// trainingResponse is the API shape of a stored training.
type trainingResponse struct {
	ID        int64           `json:"id"`
	Date      string          `json:"date"`
	Title     string          `json:"title"`
	Notes     string          `json:"notes"`
	Blocks    json.RawMessage `json:"blocks"`
	CreatedAt time.Time       `json:"created_at"`
}

func toResponse(t store.Training) trainingResponse {
	return trainingResponse{
		ID:        t.ID,
		Date:      t.Date.Time.Format(training.DateLayout),
		Title:     t.Title,
		Notes:     t.Notes,
		Blocks:    t.Blocks,
		CreatedAt: t.CreatedAt.Time,
	}
}

func (h *Handler) createTraining(w http.ResponseWriter, r *http.Request) {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields() // catch typos like "zones" or "time"

	var in training.Training
	if err := dec.Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := in.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	date, _ := in.ParsedDate() // already checked by Validate
	blocks, err := json.Marshal(in.Blocks)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not encode blocks")
		return
	}

	t, err := h.q.CreateTraining(r.Context(), store.CreateTrainingParams{
		Date:   pgtype.Date{Time: date, Valid: true},
		Title:  in.Title,
		Notes:  in.Notes,
		Blocks: blocks,
	})
	if err != nil {
		log.Println("create training:", err)
		writeError(w, http.StatusInternalServerError, "could not save training")
		return
	}
	writeJSON(w, http.StatusCreated, toResponse(t))
}

func (h *Handler) listTrainings(w http.ResponseWriter, r *http.Request) {
	rows, err := h.q.ListTrainings(r.Context())
	if err != nil {
		log.Println("list trainings:", err)
		writeError(w, http.StatusInternalServerError, "could not load trainings")
		return
	}
	out := make([]trainingResponse, 0, len(rows))
	for _, t := range rows {
		out = append(out, toResponse(t))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) getTraining(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	t, err := h.q.GetTraining(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "training not found")
		return
	}
	if err != nil {
		log.Println("get training:", err)
		writeError(w, http.StatusInternalServerError, "could not load training")
		return
	}
	writeJSON(w, http.StatusOK, toResponse(t))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Println("write response:", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
