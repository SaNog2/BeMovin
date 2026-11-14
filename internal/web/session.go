package web

import (
	"fmt"
	"math"
	"strings"

	"github.com/SaNog2/BeMovin/internal/training"
)

// The session view turns a workout tree into what the page draws: an
// intensity chart (one bar per step, width = time, height = zone) and one
// card per top-level block ("set").

// estPaceSPerKm is a rough pace per zone, used only to give distance-based
// steps a width in the time-based chart. Index 0 is "no zone".
var estPaceSPerKm = [training.MaxZone + 1]int{360, 390, 360, 330, 305, 285, 270, 255}

type sessionView struct {
	trainingView
	Sets      []setView
	Bars      []barView
	Ticks     []tickView
	Zones     []int // zones used, for the legend
	Estimated bool  // some widths come from estimated paces
	Selected  int   // set selected by default (1-based)
	Comments  []commentView
}

type setView struct {
	Num       int // 1-based
	Name      string
	Structure string
	Zone      int // 0 = none; for a repeat, the hardest zone inside
	DurLabel  string
	Extras    []extraView
	Grow      int // seconds, for the bracket under the chart
}

type extraView struct {
	Label, Text string
}

type barView struct {
	Grow int // seconds
	Zone int // 0 = none
	Set  int // 1-based set number
}

type tickView struct {
	Left  float64 // percent
	Label string
	Align string // start, center, end
}

func newSessionView(t trainingView) sessionView {
	s := sessionView{trainingView: t}
	total := 0
	for i, b := range t.Blocks {
		num := i + 1
		var bars []barView
		s.Estimated = flatten(b, num, &bars) || s.Estimated
		secs := 0
		for _, bar := range bars {
			secs += bar.Grow
		}
		total += secs
		s.Bars = append(s.Bars, bars...)
		s.Sets = append(s.Sets, newSetView(b, num, secs))
	}

	// Default selection: the hardest set, like a coach would point at.
	for _, set := range s.Sets {
		if s.Selected == 0 || set.Zone > s.Sets[s.Selected-1].Zone {
			s.Selected = set.Num
		}
	}

	used := map[int]bool{}
	for _, bar := range s.Bars {
		used[bar.Zone] = true
	}
	for z := 1; z <= training.MaxZone; z++ {
		if used[z] {
			s.Zones = append(s.Zones, z)
		}
	}

	s.Ticks = ticks(total, s.Estimated)
	return s
}

// flatten appends one bar per step of b, with repeats expanded. It
// reports whether any width was estimated from a distance.
func flatten(b training.Block, set int, out *[]barView) (estimated bool) {
	switch b.Type {
	case training.BlockStep:
		secs, est := stepSeconds(b)
		*out = append(*out, barView{Grow: secs, Zone: zoneOf(b.Zone), Set: set})
		return est
	case training.BlockRepeat:
		var inner []barView
		for _, c := range b.Blocks {
			estimated = flatten(c, set, &inner) || estimated
		}
		for range b.Count {
			*out = append(*out, inner...)
		}
	}
	return estimated
}

func stepSeconds(b training.Block) (secs int, estimated bool) {
	if b.Duration == nil {
		return 0, false
	}
	if b.Duration.TimeS != nil {
		return *b.Duration.TimeS, false
	}
	if b.Duration.DistanceM != nil {
		pace := estPaceSPerKm[zoneOf(b.Zone)]
		return int(math.Round(float64(*b.Duration.DistanceM) * float64(pace) / 1000)), true
	}
	return 0, false
}

func zoneOf(z *int) int {
	if z == nil {
		return 0
	}
	return *z
}

func maxZone(b training.Block) int {
	if b.Type == training.BlockStep {
		return zoneOf(b.Zone)
	}
	m := 0
	for _, c := range b.Blocks {
		m = max(m, maxZone(c))
	}
	return m
}

func newSetView(b training.Block, num, secs int) setView {
	s := setView{
		Num:      num,
		Zone:     maxZone(b),
		DurLabel: formatSummary(training.Summarize([]training.Block{b})),
		Grow:     secs,
	}
	if b.Type == training.BlockStep {
		s.Name = kindLabel(b.Kind)
		s.Structure = formatDuration(b.Duration)
		if b.Duration != nil && b.Duration.TimeS != nil {
			s.Structure += " continuous"
		}
		if b.Notes != "" {
			s.Extras = append(s.Extras, extraView{"Note", b.Notes})
		}
		return s
	}

	s.Name = repeatName(b)
	s.Structure = repeatStructure(b)
	for _, c := range b.Blocks {
		if c.Type == training.BlockStep && c.Kind == training.KindRecovery {
			s.Extras = append(s.Extras, extraView{"Recovery", stepShort(c)})
		}
	}
	for _, c := range b.Blocks {
		if c.Type == training.BlockStep && c.Notes != "" {
			s.Extras = append(s.Extras, extraView{"Note", c.Notes})
		}
	}
	return s
}

// repeatName is e.g. "5 × 3 min" or "3 × (2 × 400 m)": the count and the
// first non-recovery block inside.
func repeatName(b training.Block) string {
	main := b.Blocks[0]
	for _, c := range b.Blocks {
		if c.Type == training.BlockRepeat || c.Kind != training.KindRecovery {
			main = c
			break
		}
	}
	inner := formatDuration(main.Duration)
	if main.Type == training.BlockRepeat {
		inner = "(" + repeatName(main) + ")"
	}
	return fmt.Sprintf("%d × %s", b.Count, inner)
}

// repeatStructure lists what one round contains, e.g. "400 m Z5 + 1:30 Z1".
func repeatStructure(b training.Block) string {
	parts := make([]string, 0, len(b.Blocks))
	for _, c := range b.Blocks {
		if c.Type == training.BlockRepeat {
			parts = append(parts, "("+repeatName(c)+")")
		} else {
			parts = append(parts, stepShort(c))
		}
	}
	return "Each round: " + strings.Join(parts, " + ")
}

func stepShort(b training.Block) string {
	s := formatDuration(b.Duration)
	if b.Zone != nil {
		s += fmt.Sprintf(" Z%d", *b.Zone)
	}
	return s
}

// ticks returns minute marks along the chart's time axis.
func ticks(totalS int, estimated bool) []tickView {
	if totalS <= 0 {
		return nil
	}
	totalMin := float64(totalS) / 60
	step := 10.0
	switch {
	case totalMin > 120:
		step = 30
	case totalMin > 60:
		step = 20
	case totalMin <= 20:
		step = 5
	}
	prefix := ""
	if estimated {
		prefix = "~"
	}
	var out []tickView
	for m := 0.0; m < totalMin-step*0.75; m += step {
		align := "center"
		if m == 0 {
			align = "start"
		}
		out = append(out, tickView{Left: m / totalMin * 100, Label: fmt.Sprintf("%s%.0f′", prefix, m), Align: align})
	}
	out[0].Label = "0′"
	return append(out, tickView{Left: 100, Label: fmt.Sprintf("%s%.0f′", prefix, math.Round(totalMin)), Align: "end"})
}

func zoneName(z int) string {
	if z == 0 {
		return "No zone"
	}
	return fmt.Sprintf("Zone %d", z)
}
