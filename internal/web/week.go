package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/SaNog2/BeMovin/internal/auth"
	"github.com/SaNog2/BeMovin/internal/store"
	"github.com/SaNog2/BeMovin/internal/training"
)

// The main screen is a week: the list of days on the left and the
// selected training on the right. On a phone only one of the two shows:
// the list on a week URL, the training (with a back button) on a training
// URL.

type weekView struct {
	Start   time.Time // Monday
	Label   string    // "16–22 Nov"
	URL     string
	PrevURL string
	NextURL string
	Days    []dayView
}

type dayView struct {
	Dow, Date string
	ISO       string // YYYY-MM-DD
	IsToday   bool
	Rows      []rowView
}

type rowView struct {
	ID       int64
	URL      string
	Title    string
	DurLabel string
	Bars     []barView
	StripW   float64 // percent of the longest training this week
	Selected bool
}

// weekPageData is everything the week page renders.
type weekPageData struct {
	Week    weekView
	Session *sessionView // nil: nothing to show on the right
	// Focus is true on a training URL: phones show the training, not the list.
	Focus   bool
	IsAdmin bool
}

func mondayOf(t time.Time) time.Time {
	d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	offset := (int(d.Weekday()) + 6) % 7 // Monday = 0
	return d.AddDate(0, 0, -offset)
}

func weekURL(monday time.Time) string {
	return "/weeks/" + monday.Format(training.DateLayout)
}

func weekLabel(start time.Time) string {
	end := start.AddDate(0, 0, 6)
	switch {
	case start.Year() != end.Year():
		return start.Format("2 Jan 2006") + " – " + end.Format("2 Jan 2006")
	case start.Month() != end.Month():
		return start.Format("2 Jan") + " – " + end.Format("2 Jan")
	default:
		return fmt.Sprintf("%d–%s", start.Day(), end.Format("2 Jan"))
	}
}

// today is the current date in the server's time zone (set TZ).
func today() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func (h *Handler) homePage(w http.ResponseWriter, r *http.Request) {
	h.renderWeek(w, r, mondayOf(today()), 0)
}

func (h *Handler) weekPage(w http.ResponseWriter, r *http.Request) {
	d, err := time.Parse(training.DateLayout, r.PathValue("date"))
	if err != nil {
		h.notFound(w, r)
		return
	}
	monday := mondayOf(d)
	if !monday.Equal(d) {
		http.Redirect(w, r, weekURL(monday), http.StatusSeeOther)
		return
	}
	h.renderWeek(w, r, monday, 0)
}

func (h *Handler) trainingPage(w http.ResponseWriter, r *http.Request) {
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
	h.renderWeek(w, r, mondayOf(row.Date.Time), id)
}

// renderWeek renders the week starting at monday. selected is the training
// to show on the right; 0 picks one (see defaultSelection).
func (h *Handler) renderWeek(w http.ResponseWriter, r *http.Request, monday time.Time, selected int64) {
	rows, err := h.q.ListTrainingsBetween(r.Context(), weekRange(monday))
	if err != nil {
		h.serverError(w, r, "list week", err)
		return
	}
	views := make([]trainingView, 0, len(rows))
	for _, row := range rows {
		v, err := toView(row)
		if err != nil {
			h.serverError(w, r, "decode training", err)
			return
		}
		views = append(views, v)
	}

	data := weekPageData{Focus: selected != 0, IsAdmin: auth.IsAdmin(r.Context())}
	if selected == 0 {
		selected = defaultSelection(views, today())
	}
	data.Week = buildWeek(monday, views, selected, today())
	for _, v := range views {
		if v.ID == selected {
			s := newSessionView(v)
			data.Session = &s
		}
	}
	render(w, r, http.StatusOK, weekPage(data))
}

// defaultSelection picks the training to show when none was chosen: the
// first one today or later, else the last one of the week.
func defaultSelection(views []trainingView, today time.Time) int64 {
	if len(views) == 0 {
		return 0
	}
	for _, v := range views {
		if !v.Date.Before(today) {
			return v.ID
		}
	}
	return views[len(views)-1].ID
}

func buildWeek(monday time.Time, views []trainingView, selected int64, today time.Time) weekView {
	w := weekView{
		Start:   monday,
		Label:   weekLabel(monday),
		URL:     weekURL(monday),
		PrevURL: weekURL(monday.AddDate(0, 0, -7)),
		NextURL: weekURL(monday.AddDate(0, 0, 7)),
	}

	longest := 0
	totals := map[int64]int{}
	bars := map[int64][]barView{}
	for _, v := range views {
		var bs []barView
		for i, b := range v.Blocks {
			flatten(b, i+1, &bs)
		}
		sum := 0
		for _, b := range bs {
			sum += b.Grow
		}
		totals[v.ID], bars[v.ID] = sum, bs
		longest = max(longest, sum)
	}

	for i := range 7 {
		day := monday.AddDate(0, 0, i)
		d := dayView{Dow: day.Format("Mon"), Date: day.Format("2"), ISO: day.Format(training.DateLayout), IsToday: day.Equal(today)}
		for _, v := range views {
			if !v.Date.Equal(day) {
				continue
			}
			strip := 0.0
			if longest > 0 {
				strip = float64(totals[v.ID]) / float64(longest) * 100
			}
			d.Rows = append(d.Rows, rowView{
				ID:       v.ID,
				URL:      trainingURL(v.ID),
				Title:    v.Title,
				DurLabel: formatSummary(v.Summary),
				Bars:     bars[v.ID],
				StripW:   strip,
				Selected: v.ID == selected,
			})
		}
		w.Days = append(w.Days, d)
	}
	return w
}

func pgDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}

func weekRange(monday time.Time) store.ListTrainingsBetweenParams {
	return store.ListTrainingsBetweenParams{StartDate: pgDate(monday), EndDate: pgDate(monday.AddDate(0, 0, 7))}
}
