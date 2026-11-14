// Package training defines the structure of a planned running workout.
//
// A workout is a tree of blocks. A block is either a step (a single
// effort with a duration) or a repeat (a group of blocks done N times).
// Repeats can be nested, e.g. 3 x (4 x 200m + 200m jog).
package training

import (
	"errors"
	"fmt"
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

// Validate checks the whole training and returns all problems found.
func (t *Training) Validate() error {
	var errs []error
	if strings.TrimSpace(t.Title) == "" {
		errs = append(errs, errors.New("title is required"))
	}
	if _, err := t.ParsedDate(); err != nil {
		errs = append(errs, fmt.Errorf("date must be YYYY-MM-DD, got %q", t.Date))
	}
	errs = append(errs, validateBlocks(t.Blocks, "blocks", 0)...)
	return errors.Join(errs...)
}

func validateBlocks(blocks []Block, path string, depth int) []error {
	if len(blocks) == 0 {
		return []error{fmt.Errorf("%s: at least one block is required", path)}
	}
	var errs []error
	for i := range blocks {
		errs = append(errs, blocks[i].validate(fmt.Sprintf("%s[%d]", path, i), depth)...)
	}
	return errs
}

func (b *Block) validate(path string, depth int) []error {
	var errs []error
	add := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf(path+": "+format, args...))
	}

	switch b.Type {
	case BlockStep:
		switch b.Kind {
		case KindWarmup, KindWork, KindRecovery, KindCooldown:
		default:
			add("kind must be one of warmup, work, recovery, cooldown, got %q", b.Kind)
		}
		if b.Duration == nil {
			add("duration is required")
		} else {
			errs = append(errs, b.Duration.validate(path+".duration")...)
		}
		if b.Zone != nil && (*b.Zone < MinZone || *b.Zone > MaxZone) {
			add("zone must be between %d and %d, got %d", MinZone, MaxZone, *b.Zone)
		}
		if b.Count != 0 || len(b.Blocks) > 0 {
			add("a step cannot have count or blocks")
		}

	case BlockRepeat:
		if depth >= MaxDepth {
			add("repeats are nested too deep (max %d)", MaxDepth)
			return errs
		}
		if b.Count < 1 {
			add("count must be at least 1")
		}
		if b.Kind != "" || b.Duration != nil || b.Zone != nil {
			add("a repeat cannot have kind, duration or zone")
		}
		errs = append(errs, validateBlocks(b.Blocks, path+".blocks", depth+1)...)

	default:
		add("type must be %q or %q, got %q", BlockStep, BlockRepeat, b.Type)
	}
	return errs
}

func (d *Duration) validate(path string) []error {
	switch {
	case d.TimeS != nil && d.DistanceM != nil:
		return []error{fmt.Errorf("%s: set either time_s or distance_m, not both", path)}
	case d.TimeS != nil:
		if *d.TimeS <= 0 {
			return []error{fmt.Errorf("%s: time_s must be positive", path)}
		}
	case d.DistanceM != nil:
		if *d.DistanceM <= 0 {
			return []error{fmt.Errorf("%s: distance_m must be positive", path)}
		}
	default:
		return []error{fmt.Errorf("%s: time_s or distance_m is required", path)}
	}
	return nil
}
