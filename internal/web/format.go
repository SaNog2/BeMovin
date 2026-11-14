package web

import (
	"fmt"
	"github.com/a-h/templ"
	"strconv"
	"strings"
	"time"

	"github.com/SaNog2/BeMovin/internal/training"
)

// formatTime renders seconds as "45 s", "15 min", "1 h 30 min" or "4:30".
func formatTime(s int) string {
	switch {
	case s < 60:
		return fmt.Sprintf("%d s", s)
	case s%60 != 0 && s < 3600:
		return fmt.Sprintf("%d:%02d", s/60, s%60)
	case s%60 != 0:
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
	case s < 3600:
		return fmt.Sprintf("%d min", s/60)
	case s%3600 == 0:
		return fmt.Sprintf("%d h", s/3600)
	default:
		return fmt.Sprintf("%d h %d min", s/3600, s/60%60)
	}
}

// formatDistance renders meters as "400 m" or "1.5 km".
func formatDistance(m int) string {
	if m < 1000 {
		return fmt.Sprintf("%d m", m)
	}
	km := strconv.FormatFloat(float64(m)/1000, 'f', 2, 64)
	km = strings.TrimRight(strings.TrimRight(km, "0"), ".")
	return km + " km"
}

func formatDuration(d *training.Duration) string {
	switch {
	case d == nil:
		return ""
	case d.TimeS != nil:
		return formatTime(*d.TimeS)
	case d.DistanceM != nil:
		return formatDistance(*d.DistanceM)
	}
	return ""
}

func formatSummary(s training.Summary) string {
	var parts []string
	if s.TimeS > 0 {
		parts = append(parts, formatTime(s.TimeS))
	}
	if s.DistanceM > 0 {
		parts = append(parts, formatDistance(s.DistanceM))
	}
	return strings.Join(parts, " + ")
}

func formatDate(t time.Time) string {
	return t.Format("Mon 2 Jan 2006")
}

func kindLabel(k training.StepKind) string {
	switch k {
	case training.KindWarmup:
		return "Warm-up"
	case training.KindWork:
		return "Work"
	case training.KindRecovery:
		return "Recovery"
	case training.KindCooldown:
		return "Cool-down"
	}
	return string(k)
}

func trainingURL(id int64) string {
	return fmt.Sprintf("/trainings/%d", id)
}

var (
	stepKinds = []training.StepKind{training.KindWarmup, training.KindWork, training.KindRecovery, training.KindCooldown}
	units     = []string{unitMin, unitSec, unitM, unitKm}
)

// draftZone is the zone typed in the editor, 0 when none or invalid.
func draftZone(z string) int {
	n, err := strconv.Atoi(z)
	if err != nil || n < training.MinZone || n > training.MaxZone {
		return 0
	}
	return n
}

func zoneCls(z int) string {
	if z == 0 {
		return "zone-none"
	}
	return fmt.Sprintf("zone-%d", z)
}

func grow(n int) templ.SafeCSS {
	return templ.SafeCSS(fmt.Sprintf("flex-grow:%d", n))
}

func pageTitle(d weekPageData) string {
	if d.Session != nil && d.Focus {
		return d.Session.Title
	}
	return "Week of " + d.Week.Label
}

func chartLabel(s sessionView) string {
	return fmt.Sprintf("Intensity over time, %s, %d sets", formatSummary(s.Summary), len(s.Sets))
}
