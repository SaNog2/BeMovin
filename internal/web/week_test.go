package web

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/SaNog2/BeMovin/internal/training"
)

func day(s string) time.Time {
	t, err := time.Parse(training.DateLayout, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestMondayOf(t *testing.T) {
	for in, want := range map[string]string{
		"2026-11-16": "2026-11-16", // Monday
		"2026-11-19": "2026-11-16",
		"2026-11-22": "2026-11-16", // Sunday
		"2026-11-23": "2026-11-23",
	} {
		if got := mondayOf(day(in)).Format(training.DateLayout); got != want {
			t.Errorf("mondayOf(%s) = %s, want %s", in, got, want)
		}
	}
}

func TestWeekLabel(t *testing.T) {
	for in, want := range map[string]string{
		"2026-11-16": "16–22 Nov",
		"2026-11-30": "30 Nov – 6 Dec",
		"2026-12-28": "28 Dec 2026 – 3 Jan 2027",
	} {
		if got := weekLabel(day(in)); got != want {
			t.Errorf("weekLabel(%s) = %q, want %q", in, got, want)
		}
	}
}

func TestDefaultSelection(t *testing.T) {
	views := []trainingView{{ID: 1, Date: day("2026-11-16")}, {ID: 2, Date: day("2026-11-18")}, {ID: 3, Date: day("2026-11-21")}}
	for today, want := range map[string]int64{
		"2026-11-15": 1, // before the week: first
		"2026-11-18": 2, // a training today
		"2026-11-19": 3, // next one
		"2026-11-23": 3, // week is over: last
	} {
		if got := defaultSelection(views, day(today)); got != want {
			t.Errorf("today %s: got %d, want %d", today, got, want)
		}
	}
	if got := defaultSelection(nil, day("2026-11-18")); got != 0 {
		t.Errorf("empty week: got %d", got)
	}
}

func blocks(t *testing.T, s string) []training.Block {
	t.Helper()
	var b []training.Block
	if err := json.Unmarshal([]byte(s), &b); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSessionView(t *testing.T) {
	v := trainingView{ID: 1, Title: "Track", Date: day("2026-11-17"), Blocks: blocks(t, `[
		{"type":"step","kind":"warmup","duration":{"time_s":600},"zone":1},
		{"type":"repeat","count":3,"blocks":[
			{"type":"step","kind":"work","duration":{"distance_m":400},"zone":5,"notes":"Relaxed"},
			{"type":"step","kind":"recovery","duration":{"time_s":90},"zone":1}]},
		{"type":"step","kind":"cooldown","duration":{"time_s":480}}]`)}
	s := newSessionView(v)

	if len(s.Bars) != 1+3*2+1 {
		t.Fatalf("got %d bars, want 8", len(s.Bars))
	}
	// 400 m at the Z5 estimate of 285 s/km = 114 s.
	if s.Bars[1].Grow != 114 || s.Bars[1].Zone != 5 || s.Bars[1].Set != 2 {
		t.Errorf("first work bar = %+v", s.Bars[1])
	}
	if !s.Estimated {
		t.Error("want Estimated with a distance step")
	}
	if s.Selected != 2 {
		t.Errorf("selected set %d, want the hardest (2)", s.Selected)
	}
	if !reflect.DeepEqual(s.Zones, []int{1, 5}) {
		t.Errorf("zones %v", s.Zones)
	}

	set := s.Sets[1]
	if set.Name != "3 × 400 m" || set.Structure != "Each round: 400 m Z5 + 1:30 Z1" || set.Zone != 5 {
		t.Errorf("repeat set = %+v", set)
	}
	wantExtras := []extraView{{"Recovery", "1:30 Z1"}, {"Note", "Relaxed"}}
	if !reflect.DeepEqual(set.Extras, wantExtras) {
		t.Errorf("extras %v, want %v", set.Extras, wantExtras)
	}
	if set.DurLabel != "4:30 + 1.2 km" {
		t.Errorf("repeat duration %q", set.DurLabel)
	}
	if s.Sets[0].Structure != "10 min continuous" || s.Sets[2].Zone != 0 {
		t.Errorf("step sets = %+v / %+v", s.Sets[0], s.Sets[2])
	}

	last := s.Ticks[len(s.Ticks)-1]
	if last.Label != "~28′" || last.Left != 100 || s.Ticks[0].Label != "0′" {
		t.Errorf("ticks %+v", s.Ticks)
	}
}

func TestRepeatNameNested(t *testing.T) {
	b := blocks(t, `[{"type":"repeat","count":3,"blocks":[
		{"type":"repeat","count":2,"blocks":[{"type":"step","kind":"work","duration":{"time_s":60}}]},
		{"type":"step","kind":"recovery","duration":{"time_s":180}}]}]`)[0]
	if got := repeatName(b); got != "3 × (2 × 1 min)" {
		t.Errorf("got %q", got)
	}
}

func TestBuildWeek(t *testing.T) {
	short := trainingView{ID: 1, Title: "Easy", Date: day("2026-11-16"), Blocks: blocks(t, `[{"type":"step","kind":"work","duration":{"time_s":1800}}]`)}
	long := trainingView{ID: 2, Title: "Long", Date: day("2026-11-21"), Blocks: blocks(t, `[{"type":"step","kind":"work","duration":{"time_s":3600}}]`)}
	w := buildWeek(day("2026-11-16"), []trainingView{short, long}, 2, day("2026-11-18"))

	if len(w.Days) != 7 || w.Label != "16–22 Nov" {
		t.Fatalf("week = %+v", w)
	}
	if w.PrevURL != "/weeks/2026-11-09" || w.NextURL != "/weeks/2026-11-23" {
		t.Errorf("nav %s %s", w.PrevURL, w.NextURL)
	}
	if !w.Days[2].IsToday || len(w.Days[2].Rows) != 0 {
		t.Errorf("wednesday = %+v", w.Days[2])
	}
	mon, sat := w.Days[0].Rows[0], w.Days[5].Rows[0]
	if mon.StripW != 50 || sat.StripW != 100 || mon.Selected || !sat.Selected {
		t.Errorf("rows %+v / %+v", mon, sat)
	}
}

func TestTicksDoNotCrowdTheEnd(t *testing.T) {
	var labels []string
	for _, tk := range ticks(47*60, false) {
		labels = append(labels, tk.Label)
	}
	if want := []string{"0′", "10′", "20′", "30′", "47′"}; !reflect.DeepEqual(labels, want) {
		t.Errorf("got %v, want %v", labels, want)
	}
}
