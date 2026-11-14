package training

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

const valid = `{
  "title": "6x400m intervals",
  "date": "2026-11-15",
  "blocks": [
    {"type": "step", "kind": "warmup", "duration": {"time_s": 900}, "zone": 2},
    {"type": "repeat", "count": 3, "blocks": [
      {"type": "repeat", "count": 2, "blocks": [
        {"type": "step", "kind": "work", "duration": {"distance_m": 400}, "zone": 5},
        {"type": "step", "kind": "recovery", "duration": {"time_s": 90}, "zone": 1}
      ]},
      {"type": "step", "kind": "recovery", "duration": {"time_s": 180}}
    ]},
    {"type": "step", "kind": "cooldown", "duration": {"time_s": 600}}
  ]
}`

func parse(t *testing.T, s string) Training {
	t.Helper()
	var tr Training
	if err := json.Unmarshal([]byte(s), &tr); err != nil {
		t.Fatal(err)
	}
	return tr
}

func TestValidateOK(t *testing.T) {
	tr := parse(t, valid)
	if err := tr.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateErrors(t *testing.T) {
	tests := []struct {
		name, json, want string
	}{
		{"missing title", `{"date":"2026-11-15","blocks":[{"type":"step","kind":"work","duration":{"time_s":60}}]}`, "title: required"},
		{"bad date", `{"title":"x","date":"15/11/2026","blocks":[{"type":"step","kind":"work","duration":{"time_s":60}}]}`, "date: must be YYYY-MM-DD"},
		{"no blocks", `{"title":"x","date":"2026-11-15","blocks":[]}`, "blocks: at least one block"},
		{"unknown type", `{"title":"x","date":"2026-11-15","blocks":[{"type":"run"}]}`, `blocks[0].type: must be`},
		{"bad kind", `{"title":"x","date":"2026-11-15","blocks":[{"type":"step","kind":"sprint","duration":{"time_s":60}}]}`, "blocks[0].kind: must be one of"},
		{"no duration", `{"title":"x","date":"2026-11-15","blocks":[{"type":"step","kind":"work"}]}`, "blocks[0].duration: required"},
		{"both durations", `{"title":"x","date":"2026-11-15","blocks":[{"type":"step","kind":"work","duration":{"time_s":60,"distance_m":400}}]}`, "not both"},
		{"negative time", `{"title":"x","date":"2026-11-15","blocks":[{"type":"step","kind":"work","duration":{"time_s":-1}}]}`, "must be positive"},
		{"zone too high", `{"title":"x","date":"2026-11-15","blocks":[{"type":"step","kind":"work","duration":{"time_s":60},"zone":8}]}`, "blocks[0].zone: must be between 1 and 7"},
		{"zone zero", `{"title":"x","date":"2026-11-15","blocks":[{"type":"step","kind":"work","duration":{"time_s":60},"zone":0}]}`, "blocks[0].zone: must be between 1 and 7"},
		{"repeat count 0", `{"title":"x","date":"2026-11-15","blocks":[{"type":"repeat","count":0,"blocks":[{"type":"step","kind":"work","duration":{"time_s":60}}]}]}`, "blocks[0].count: must be at least 1"},
		{"empty repeat", `{"title":"x","date":"2026-11-15","blocks":[{"type":"repeat","count":2,"blocks":[]}]}`, "blocks[0].blocks: at least one block"},
		{"repeat with duration", `{"title":"x","date":"2026-11-15","blocks":[{"type":"repeat","count":2,"duration":{"time_s":60},"blocks":[{"type":"step","kind":"work","duration":{"time_s":60}}]}]}`, "a repeat cannot have"},
		{"nested error path", `{"title":"x","date":"2026-11-15","blocks":[{"type":"repeat","count":2,"blocks":[{"type":"step","kind":"work","duration":{}}]}]}`, "blocks[0].blocks[0].duration: time_s or distance_m is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := parse(t, tt.json)
			err := tr.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want error containing %q", err, tt.want)
			}
		})
	}
}

func TestValidateMaxDepth(t *testing.T) {
	inner := `{"type":"step","kind":"work","duration":{"time_s":60}}`
	for i := 0; i <= MaxDepth; i++ {
		inner = `{"type":"repeat","count":2,"blocks":[` + inner + `]}`
	}
	tr := parse(t, `{"title":"x","date":"2026-11-15","blocks":[`+inner+`]}`)
	if err := tr.Validate(); err == nil || !strings.Contains(err.Error(), "nested too deep") {
		t.Fatalf("got %v, want nesting error", err)
	}
}

func TestSummarize(t *testing.T) {
	tr := parse(t, valid)
	// warmup 900 + 3*(2*(400m + 90s) + 180s) + cooldown 600
	got := Summarize(tr.Blocks)
	want := Summary{TimeS: 900 + 3*(2*90+180) + 600, DistanceM: 3 * 2 * 400}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestValidateProblems(t *testing.T) {
	tr := parse(t, `{"title":"","date":"2026-11-15","blocks":[
		{"type":"step","kind":"work","duration":{"time_s":60}},
		{"type":"repeat","count":2,"blocks":[{"type":"step","kind":"work","duration":{}}]}]}`)
	var ve *ValidationError
	if !errors.As(tr.Validate(), &ve) {
		t.Fatal("want *ValidationError")
	}
	want := []Problem{
		{Block: nil, Field: "title", Msg: "required"},
		{Block: []int{1, 0}, Field: "duration", Msg: "time_s or distance_m is required"},
	}
	if !reflect.DeepEqual(ve.Problems, want) {
		t.Fatalf("got %+v, want %+v", ve.Problems, want)
	}
}
