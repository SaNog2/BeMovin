package web

import (
	"net/url"
	"reflect"
	"slices"
	"testing"

	"github.com/SaNog2/BeMovin/internal/training"
)

// sampleForm is warmup, 2 × (work, recovery), cooldown.
func sampleForm() url.Values {
	return url.Values{
		"title": {" Intervals "}, "date": {"2026-11-20"}, "notes": {""},
		"b[0].type": {"step"}, "b[0].kind": {"warmup"}, "b[0].value": {"15"}, "b[0].unit": {"min"}, "b[0].zone": {"2"},
		"b[1].type": {"repeat"}, "b[1].count": {"2"},
		"b[1.0].type": {"step"}, "b[1.0].kind": {"work"}, "b[1.0].value": {"400"}, "b[1.0].unit": {"m"}, "b[1.0].zone": {"5"}, "b[1.0].notes": {"fast"},
		"b[1.1].type": {"step"}, "b[1.1].kind": {"recovery"}, "b[1.1].value": {"1:30"}, "b[1.1].unit": {"min"}, "b[1.1].zone": {""},
		"b[2].type": {"step"}, "b[2].kind": {"cooldown"}, "b[2].value": {"10"}, "b[2].unit": {"min"}, "b[2].zone": {"2"},
		// Indexes above 9 must sort numerically, not as strings.
		"b[10].type": {"step"}, "b[10].kind": {"cooldown"}, "b[10].value": {"2.5"}, "b[10].unit": {"km"}, "b[10].zone": {""},
	}
}

func ptr(n int) *int { return &n }

func TestParseDraftAndConvert(t *testing.T) {
	d, err := parseDraft(sampleForm())
	if err != nil {
		t.Fatal(err)
	}
	kinds := []string{}
	for _, b := range d.Blocks {
		kinds = append(kinds, b.Kind)
	}
	if want := []string{"warmup", "", "cooldown", "cooldown"}; !slices.Equal(kinds, want) {
		t.Fatalf("block order %v, want %v", kinds, want)
	}

	tr, errs := d.toTraining()
	if errs != nil {
		t.Fatalf("unexpected errors: %v", errs)
	}
	want := training.Training{
		Title: "Intervals", Date: "2026-11-20",
		Blocks: []training.Block{
			{Type: "step", Kind: "warmup", Duration: &training.Duration{TimeS: ptr(900)}, Zone: ptr(2)},
			{Type: "repeat", Count: 2, Blocks: []training.Block{
				{Type: "step", Kind: "work", Duration: &training.Duration{DistanceM: ptr(400)}, Zone: ptr(5), Notes: "fast"},
				{Type: "step", Kind: "recovery", Duration: &training.Duration{TimeS: ptr(90)}},
			}},
			{Type: "step", Kind: "cooldown", Duration: &training.Duration{TimeS: ptr(600)}, Zone: ptr(2)},
			{Type: "step", Kind: "cooldown", Duration: &training.Duration{DistanceM: ptr(2500)}},
		},
	}
	if !reflect.DeepEqual(tr, want) {
		t.Fatalf("got %+v\nwant %+v", tr, want)
	}
}

func TestParseDraftRejectsBadPaths(t *testing.T) {
	for _, key := range []string{"b[1..2].type", "b[.1].type", "b[1.].type", "b[99999].type"} {
		if _, err := parseDraft(url.Values{key: {"step"}}); err == nil {
			t.Errorf("%s: want error", key)
		}
	}
	// Names that don't look like block fields are ignored.
	d, err := parseDraft(url.Values{"b[].type": {"step"}, "b[-1].type": {"step"}, "other": {"x"}})
	if err != nil || len(d.Blocks) != 0 {
		t.Errorf("got %+v, %v; want empty draft", d, err)
	}
}

func TestApplyOp(t *testing.T) {
	d, _ := parseDraft(sampleForm())
	kinds := func() []string {
		var out []string
		for _, b := range d.Blocks {
			out = append(out, string(b.Type)+":"+b.Kind)
		}
		return out
	}

	must := func(op string) {
		t.Helper()
		if err := d.applyOp(op); err != nil {
			t.Fatalf("%s: %v", op, err)
		}
	}

	must("down:0")
	if got := kinds(); got[0] != "repeat:" || got[1] != "step:warmup" {
		t.Fatalf("after down:0 got %v", got)
	}
	must("up:1")
	must("remove:3")
	if len(d.Blocks) != 3 {
		t.Fatalf("after remove got %d blocks", len(d.Blocks))
	}
	must("add-step:1")
	if n := len(d.Blocks[1].Children); n != 3 {
		t.Fatalf("add-step:1 gave %d children, want 3", n)
	}
	must("add-repeat:")
	if last := d.Blocks[len(d.Blocks)-1]; last.Type != training.BlockRepeat || len(last.Children) != 2 {
		t.Fatalf("add-repeat gave %+v", last)
	}
	must("up:0") // no-op at the top
	must("down:3")

	for _, bad := range []string{"add-step:0", "remove:", "remove:9", "explode:0", "up:x"} {
		if err := d.applyOp(bad); err == nil {
			t.Errorf("%s: want error", bad)
		}
	}
}

func TestApplyOpMaxDepth(t *testing.T) {
	d := draft{}
	if err := d.applyOp("add-repeat:"); err != nil {
		t.Fatal(err)
	}
	path := "0"
	for i := 1; i < training.MaxDepth; i++ {
		if err := d.applyOp("add-repeat:" + path); err != nil {
			t.Fatalf("depth %d: %v", i, err)
		}
		path += ".2" // the new repeat lands after the default work + recovery
	}
	if err := d.applyOp("add-repeat:" + path); err == nil {
		t.Fatal("want nesting error")
	}
}

func TestToTrainingErrors(t *testing.T) {
	f := sampleForm()
	f.Set("title", "  ")
	f.Set("b[0].value", "abc")
	f.Set("b[1].count", "0")
	f.Set("b[1.1].value", "")
	f.Set("b[2].value", "-3")
	d, _ := parseDraft(f)
	_, errs := d.toTraining()
	want := formErrors{
		"title": {"required"},
		"0":     {"Duration: must be a number"},
		"1":     {"Repeat count: must be at least 1"},
		"1.1":   {"Duration: required"},
		"2":     {"Duration: must be greater than 0"},
	}
	if !reflect.DeepEqual(errs, want) {
		t.Fatalf("got %v\nwant %v", errs, want)
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		value, unit string
		timeS, m    int
		msg         string
	}{
		{"15", "min", 900, 0, ""},
		{"1.5", "min", 90, 0, ""},
		{"1,5", "min", 90, 0, ""},
		{"1:30", "min", 90, 0, ""},
		{"45", "s", 45, 0, ""},
		{"400", "m", 0, 400, ""},
		{"2.5", "km", 0, 2500, ""},
		{"", "min", 0, 0, "required"},
		{"1:75", "min", 0, 0, `use minutes like "15", "1.5" or "1:30"`},
		{"0", "km", 0, 0, "must be greater than 0"},
		{"NaN", "km", 0, 0, "must be a number"},
		{"5", "parsecs", 0, 0, "invalid unit"},
	}
	for _, tt := range tests {
		d, msg := parseDuration(tt.value, tt.unit)
		if msg != tt.msg {
			t.Errorf("%q %s: msg %q, want %q", tt.value, tt.unit, msg, tt.msg)
			continue
		}
		if msg != "" {
			continue
		}
		if (tt.timeS != 0) != (d.TimeS != nil) || (d.TimeS != nil && *d.TimeS != tt.timeS) ||
			(tt.m != 0) != (d.DistanceM != nil) || (d.DistanceM != nil && *d.DistanceM != tt.m) {
			t.Errorf("%q %s: got %+v", tt.value, tt.unit, d)
		}
	}
}
