// Package training defines the structure of a planned running workout.
//
// A workout is a tree of blocks. A block is either a step (a single
// effort with a duration) or a repeat (a group of blocks done N times).
// Repeats can be nested, e.g. 3 x (4 x 200m + 200m jog).
package training

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

const (
	MinZone  = 1
	MaxZone  = 7
	MaxDepth = 5 // max nesting of repeats

	DateLayout = "2006-01-02"
)

type BlockType string

const (
	BlockStep   BlockType = "step"
	BlockRepeat BlockType = "repeat"
)

type StepKind string

const (
	KindWarmup   StepKind = "warmup"
	KindWork     StepKind = "work"
	KindRecovery StepKind = "recovery"
	KindCooldown StepKind = "cooldown"
)

// Training is a planned workout as sent by the client.
type Training struct {
	Date   string  `json:"date"` // YYYY-MM-DD
	Title  string  `json:"title"`
	Notes  string  `json:"notes,omitempty"`
	Blocks []Block `json:"blocks"`
}

// Block is either a step or a repeat. Only the fields matching Type are used.
type Block struct {
	Type BlockType `json:"type"`

	// Step fields.
	Kind     StepKind  `json:"kind,omitempty"`
	Duration *Duration `json:"duration,omitempty"`
	Zone     *int      `json:"zone,omitempty"` // 1-7, optional
	Notes    string    `json:"notes,omitempty"`

	// Repeat fields.
	Count  int     `json:"count,omitempty"`
	Blocks []Block `json:"blocks,omitempty"`
}

// Duration is either a time or a distance; exactly one must be set.
type Duration struct {
	TimeS     *int `json:"time_s,omitempty"`
	DistanceM *int `json:"distance_m,omitempty"`
}

// ParsedDate returns the training date.
func (t *Training) ParsedDate() (time.Time, error) {
	return time.Parse(DateLayout, t.Date)
}

// Problem is one validation failure. Block is the index path of the block
// it belongs to (nil for training-level fields), e.g. [1 0] is the first
// child of the second top-level block. Field is the field name ("title",
// "duration", ...) or empty when the problem concerns the block as a whole.
type Problem struct {
	Block []int
	Field string
	Msg   string
}

// Location renders the problem's position, e.g. "blocks[1].blocks[0].duration".
func (p Problem) Location() string {
	if p.Block == nil {
		return p.Field
	}
	var b strings.Builder
	for i, idx := range p.Block {
		if i > 0 {
			b.WriteString(".")
		}
		fmt.Fprintf(&b, "blocks[%d]", idx)
	}
	if p.Field != "" {
		b.WriteString(".")
		b.WriteString(p.Field)
	}
	return b.String()
}

// ValidationError lists every problem found in a training.
type ValidationError struct {
	Problems []Problem
}

func (e *ValidationError) Error() string {
	lines := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		lines[i] = p.Location() + ": " + p.Msg
	}
	return strings.Join(lines, "\n")
}

// Validate checks the whole training. It returns a *ValidationError
// listing all problems, or nil.
func (t *Training) Validate() error {
	var v validator
	if strings.TrimSpace(t.Title) == "" {
		v.add(nil, "title", "required")
	}
	if _, err := t.ParsedDate(); err != nil {
		v.add(nil, "date", fmt.Sprintf("must be YYYY-MM-DD, got %q", t.Date))
	}
	v.blocks(t.Blocks, nil)
	if len(v.problems) == 0 {
		return nil
	}
	return &ValidationError{Problems: v.problems}
}

type validator struct {
	problems []Problem
}

func (v *validator) add(path []int, field, msg string) {
	v.problems = append(v.problems, Problem{Block: path, Field: field, Msg: msg})
}

// blocks validates the children of the block at parent (nil = top level).
func (v *validator) blocks(blocks []Block, parent []int) {
	if len(blocks) == 0 {
		v.add(parent, "blocks", "at least one block is required")
		return
	}
	for i := range blocks {
		v.block(&blocks[i], append(slices.Clone(parent), i))
	}
}

func (v *validator) block(b *Block, path []int) {
	switch b.Type {
	case BlockStep:
		switch b.Kind {
		case KindWarmup, KindWork, KindRecovery, KindCooldown:
		default:
			v.add(path, "kind", fmt.Sprintf("must be one of warmup, work, recovery, cooldown, got %q", b.Kind))
		}
		if b.Duration == nil {
			v.add(path, "duration", "required")
		} else if msg := b.Duration.problem(); msg != "" {
			v.add(path, "duration", msg)
		}
		if b.Zone != nil && (*b.Zone < MinZone || *b.Zone > MaxZone) {
			v.add(path, "zone", fmt.Sprintf("must be between %d and %d, got %d", MinZone, MaxZone, *b.Zone))
		}
		if b.Count != 0 || len(b.Blocks) > 0 {
			v.add(path, "", "a step cannot have count or blocks")
		}

	case BlockRepeat:
		if len(path) > MaxDepth {
			v.add(path, "", fmt.Sprintf("repeats are nested too deep (max %d)", MaxDepth))
			return
		}
		if b.Count < 1 {
			v.add(path, "count", "must be at least 1")
		}
		if b.Kind != "" || b.Duration != nil || b.Zone != nil {
			v.add(path, "", "a repeat cannot have kind, duration or zone")
		}
		v.blocks(b.Blocks, path)

	default:
		v.add(path, "type", fmt.Sprintf("must be %q or %q, got %q", BlockStep, BlockRepeat, b.Type))
	}
}

func (d *Duration) problem() string {
	switch {
	case d.TimeS != nil && d.DistanceM != nil:
		return "set either time_s or distance_m, not both"
	case d.TimeS != nil:
		if *d.TimeS <= 0 {
			return "time_s must be positive"
		}
	case d.DistanceM != nil:
		if *d.DistanceM <= 0 {
			return "distance_m must be positive"
		}
	default:
		return "time_s or distance_m is required"
	}
	return ""
}
