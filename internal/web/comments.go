package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/SaNog2/BeMovin/internal/auth"
	"github.com/SaNog2/BeMovin/internal/store"
)

// Comments are a thread under each training. The admin and the viewer can
// both post, and delete their own comments (the admin can delete any).

const maxCommentLen = 2000

type commentView struct {
	ID        int64
	Author    string
	Body      string
	When      string // "13 Nov, 14:05"
	Mine      bool
	CanDelete bool
}

func commentsURL(trainingID int64) string {
	return fmt.Sprintf("/trainings/%d/comments", trainingID)
}

func deleteCommentURL(trainingID, commentID int64) string {
	return fmt.Sprintf("/trainings/%d/comments/%d/delete", trainingID, commentID)
}

func newCommentViews(rows []store.Comment, me string, isAdmin bool) []commentView {
	out := make([]commentView, len(rows))
	for i, c := range rows {
		mine := c.Author == me
		out[i] = commentView{
			ID:        c.ID,
			Author:    c.Author,
			Body:      c.Body,
			When:      c.CreatedAt.Time.In(time.Local).Format("2 Jan, 15:04"),
			Mine:      mine,
			CanDelete: mine || isAdmin,
		}
	}
	return out
}

// commentInitial is the letter shown in the author's avatar.
func commentInitial(author string) string {
	r, _ := utf8.DecodeRuneInString(author)
	if r == utf8.RuneError {
		return "?"
	}
	return strings.ToUpper(string(r))
}

func (h *Handler) createComment(w http.ResponseWriter, r *http.Request) {
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
	body, msg := parseComment(r.PostForm.Get("body"))
	if msg != "" {
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	_, err = h.q.CreateComment(r.Context(), store.CreateCommentParams{
		TrainingID: id,
		Author:     auth.UserName(r.Context()),
		Body:       body,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		h.notFound(w, r)
		return
	}
	if err != nil {
		h.serverError(w, r, "create comment", err)
		return
	}
	backToTraining(w, r, id, "comments")
}

func (h *Handler) deleteComment(w http.ResponseWriter, r *http.Request) {
	id, err1 := strconv.ParseInt(r.PathValue("id"), 10, 64)
	cid, err2 := strconv.ParseInt(r.PathValue("cid"), 10, 64)
	if err1 != nil || err2 != nil {
		h.notFound(w, r)
		return
	}
	n, err := h.q.DeleteComment(r.Context(), store.DeleteCommentParams{
		ID:         cid,
		TrainingID: id,
		Author:     auth.UserName(r.Context()),
		AnyAuthor:  auth.IsAdmin(r.Context()),
	})
	if err != nil {
		h.serverError(w, r, "delete comment", err)
		return
	}
	if n == 0 {
		h.notFound(w, r)
		return
	}
	backToTraining(w, r, id, "comments")
}

// parseComment trims the body and checks it isn't empty or too long. The
// form enforces both, so a problem means a bad request.
func parseComment(body string) (string, string) {
	body = strings.TrimSpace(body)
	switch {
	case body == "":
		return "", "comment is empty"
	case utf8.RuneCountInString(body) > maxCommentLen:
		return "", fmt.Sprintf("comment is too long (max %d characters)", maxCommentLen)
	}
	return body, ""
}

// backToTraining sends the browser back to the training page after a
// post, optionally scrolled to an anchor.
func backToTraining(w http.ResponseWriter, r *http.Request, id int64, anchor string) {
	target := trainingURL(id)
	if anchor != "" {
		target += "#" + anchor
	}
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Location", target)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}
