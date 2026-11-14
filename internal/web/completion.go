package web

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/SaNog2/BeMovin/internal/store"
)

// Both the admin and the viewer can mark a training as done, rate how it
// felt and leave a note.

const maxCompletionNote = 2000

// feelings are the labels of the 1-5 "how did it feel" scale.
var feelings = [...]string{1: "Awful", 2: "Tough", 3: "OK", 4: "Good", 5: "Great"}

func feelingLabel(f int) string {
	if f < 1 || f >= len(feelings) {
		return ""
	}
	return feelings[f]
}

func completionURL(id int64) string {
	return fmt.Sprintf("/trainings/%d/completion", id)
}

// viewerWritableRe matches the paths the viewer may post to: marking a
// training as done, and posting or deleting comments.
var viewerWritableRe = regexp.MustCompile(`^/trainings/[0-9]+/(completion|comments|comments/[0-9]+/delete)$`)

// ViewerCanWrite reports whether the viewer may make the (non-GET)
// request r. Pass it to auth.Require.
func ViewerCanWrite(r *http.Request) bool {
	return r.Method == http.MethodPost && viewerWritableRe.MatchString(r.URL.Path)
}

// saveCompletion marks a training as done (updating the feeling and note
// if it already was), or as not done with action=undo.
func (h *Handler) saveCompletion(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		h.notFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	var n int64
	if r.PostForm.Get("action") == "undo" {
		n, err = h.q.UncompleteTraining(r.Context(), id)
	} else {
		params, msg := parseCompletion(id, r.PostForm.Get("feeling"), r.PostForm.Get("note"))
		if msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		n, err = h.q.CompleteTraining(r.Context(), params)
	}
	if err != nil {
		h.serverError(w, r, "save completion", err)
		return
	}
	if n == 0 {
		h.notFound(w, r)
		return
	}

	backToTraining(w, r, id, "")
}

// parseCompletion validates the submitted feeling ("" or 1-5) and note.
// The form only offers valid values, so a problem means a bad request.
func parseCompletion(id int64, feeling, note string) (store.CompleteTrainingParams, string) {
	p := store.CompleteTrainingParams{ID: id, CompletionNote: strings.TrimSpace(note)}
	if feeling != "" {
		f, err := strconv.Atoi(feeling)
		if err != nil || feelingLabel(f) == "" {
			return p, "invalid feeling"
		}
		p.Feeling = pgtype.Int2{Int16: int16(f), Valid: true}
	}
	if utf8.RuneCountInString(p.CompletionNote) > maxCompletionNote {
		return p, fmt.Sprintf("note is too long (max %d characters)", maxCompletionNote)
	}
	return p, ""
}
