package web

import (
	"cmp"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/SaNog2/BeMovin/internal/auth"
	"github.com/SaNog2/BeMovin/internal/store"
	"github.com/SaNog2/BeMovin/internal/training"
)

// The training editor is a plain HTML form. Each block's fields are named
// after the block's index path, e.g. "b[1.0].kind" for the first child of
// the second top-level block. Every edit (add, remove, move) submits the
// whole form; the server rebuilds the tree, applies the edit and renders
// the editor again. No state is kept on the server between requests.

const (
	maxFormBlocks = 500

	unitMin = "min"
	unitSec = "s"
	unitM   = "m"
	unitKm  = "km"
)

// draft is the editor's raw form state. Values are kept as typed so the
// form can be shown again unchanged when something is invalid.
type draft struct {
	Title  string
	Date   string
	Notes  string
	Blocks []draftBlock
}

type draftBlock struct {
	Type training.BlockType

	// Step
	Kind  string
	Value string // duration amount, e.g. "15", "1:30", "400", "2.5"
	Unit  string // min, s, m, km
	Zone  string // "" or "1".."7"
	Notes string

	// Repeat
	Count    string
	Children []draftBlock
}

func newStep(kind training.StepKind, value, zone string) draftBlock {
	return draftBlock{Type: training.BlockStep, Kind: string(kind), Value: value, Unit: unitMin, Zone: zone}
}

func newRepeat() draftBlock {
	return draftBlock{Type: training.BlockRepeat, Count: "4", Children: []draftBlock{
		newStep(training.KindWork, "", ""),
		newStep(training.KindRecovery, "", ""),
	}}
}

// newDraft is the form shown when creating a training.
func newDraft(today time.Time) draft {
	return draft{
		Date: today.Format(training.DateLayout),
		Blocks: []draftBlock{
			newStep(training.KindWarmup, "15", "2"),
			newStep(training.KindWork, "", ""),
			newStep(training.KindCooldown, "10", "2"),
		},
	}
}

// --- Paths ---

// blockPath is a block's index path, rendered as "1.0".
type blockPath []int

func (p blockPath) String() string {
	parts := make([]string, len(p))
	for i, n := range p {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ".")
}

func (p blockPath) child(i int) blockPath {
	return append(slices.Clone(p), i)
}

func parsePath(s string) (blockPath, bool) {
	if s == "" {
		return blockPath{}, true
	}
	var p blockPath
	for part := range strings.SplitSeq(s, ".") {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 || n >= maxFormBlocks {
			return nil, false
		}
		p = append(p, n)
	}
	return p, true
}

func fieldName(p blockPath, field string) string {
	return fmt.Sprintf("b[%s].%s", p, field)
}

var fieldRe = regexp.MustCompile(`^b\[([0-9.]+)\]\.(\w+)$`)

// --- Parsing ---

// parseDraft rebuilds the editor state from submitted form values.
func parseDraft(form url.Values) (draft, error) {
	d := draft{
		Title: form.Get("title"),
		Date:  form.Get("date"),
		Notes: form.Get("notes"),
	}

	type node struct {
		path   blockPath
		fields map[string]string
	}
	nodes := map[string]*node{}
	for key, vals := range form {
		m := fieldRe.FindStringSubmatch(key)
		if m == nil || len(vals) == 0 {
			continue
		}
		p, ok := parsePath(m[1])
		if !ok || len(p) == 0 {
			return d, fmt.Errorf("invalid block path %q", m[1])
		}
		n := nodes[m[1]]
		if n == nil {
			if len(nodes) >= maxFormBlocks {
				return d, fmt.Errorf("too many blocks")
			}
			n = &node{path: p, fields: map[string]string{}}
			nodes[m[1]] = n
		}
		n.fields[m[2]] = vals[0]
	}

	// Group nodes by parent, ordered by their index.
	children := map[string][]*node{}
	for _, n := range nodes {
		parent := n.path[:len(n.path)-1].String()
		children[parent] = append(children[parent], n)
	}
	for _, list := range children {
		slices.SortFunc(list, func(a, b *node) int {
			return cmp.Compare(a.path[len(a.path)-1], b.path[len(b.path)-1])
		})
	}

	var build func(parent string, depth int) []draftBlock
	build = func(parent string, depth int) []draftBlock {
		if depth > training.MaxDepth+1 {
			return nil
		}
		var out []draftBlock
		for _, n := range children[parent] {
			f := n.fields
			b := draftBlock{
				Type:  training.BlockType(f["type"]),
				Kind:  f["kind"],
				Value: strings.TrimSpace(f["value"]),
				Unit:  f["unit"],
				Zone:  f["zone"],
				Notes: f["notes"],
				Count: strings.TrimSpace(f["count"]),
			}
			if b.Type == training.BlockRepeat {
				b.Children = build(n.path.String(), depth+1)
			}
			out = append(out, b)
		}
		return out
	}
	d.Blocks = build("", 0)
	return d, nil
}

// --- Edit operations ---

// applyOp applies an editor operation such as "add-step:1", "remove:1.0"
// or "up:2". The part after the colon is the target block path; for the
// add operations it is the parent ("" = top level).
func (d *draft) applyOp(op string) error {
	name, arg, _ := strings.Cut(op, ":")
	p, ok := parsePath(arg)
	if !ok {
		return fmt.Errorf("invalid path %q", arg)
	}

	switch name {
	case "add-step", "add-repeat":
		list, ok := d.listAt(p)
		if !ok {
			return fmt.Errorf("no block list at %q", arg)
		}
		if name == "add-step" {
			*list = append(*list, newStep(training.KindWork, "", ""))
		} else {
			if len(p) >= training.MaxDepth {
				return fmt.Errorf("repeats are nested too deep")
			}
			*list = append(*list, newRepeat())
		}
		return nil

	case "remove", "up", "down":
		if len(p) == 0 {
			return fmt.Errorf("missing block path")
		}
		list, ok := d.listAt(p[:len(p)-1])
		i := p[len(p)-1]
		if !ok || i >= len(*list) {
			return fmt.Errorf("no block at %q", arg)
		}
		switch name {
		case "remove":
			*list = slices.Delete(*list, i, i+1)
		case "up":
			if i > 0 {
				(*list)[i-1], (*list)[i] = (*list)[i], (*list)[i-1]
			}
		case "down":
			if i < len(*list)-1 {
				(*list)[i+1], (*list)[i] = (*list)[i], (*list)[i+1]
			}
		}
		return nil
	}
	return fmt.Errorf("unknown operation %q", name)
}

// listAt returns the block list that is the children of the block at p
// (the top-level list for an empty path).
func (d *draft) listAt(p blockPath) (*[]draftBlock, bool) {
	list := &d.Blocks
	for _, i := range p {
		if i >= len(*list) || (*list)[i].Type != training.BlockRepeat {
			return nil, false
		}
		list = &(*list)[i].Children
	}
	return list, true
}

// --- Conversion and validation ---

// formErrors maps a location to its messages. Keys are "title", "date",
// "blocks" for the top level and a block path ("1.0") for a block.
type formErrors map[string][]string

func (e formErrors) add(key, msg string) { e[key] = append(e[key], msg) }

// toTraining converts the draft into a validated training. On failure it
// returns the problems keyed for display in the form.
func (d *draft) toTraining() (training.Training, formErrors) {
	errs := formErrors{}
	// Fields that failed to parse; validation problems on the same field
	// would only repeat them.
	parseFailed := map[string]bool{}

	var convert func(blocks []draftBlock, parent blockPath) []training.Block
	convert = func(blocks []draftBlock, parent blockPath) []training.Block {
		out := make([]training.Block, len(blocks))
		for i, db := range blocks {
			p := parent.child(i)
			key := p.String()
			b := training.Block{Type: db.Type}
			switch db.Type {
			case training.BlockStep:
				b.Kind = training.StepKind(db.Kind)
				b.Notes = strings.TrimSpace(db.Notes)
				dur, msg := parseDuration(db.Value, db.Unit)
				if msg != "" {
					errs.add(key, "Duration: "+msg)
					parseFailed[key+"|duration"] = true
				}
				b.Duration = dur
				if db.Zone != "" {
					z, err := strconv.Atoi(db.Zone)
					if err != nil {
						errs.add(key, "Zone: invalid")
						parseFailed[key+"|zone"] = true
					} else {
						b.Zone = &z
					}
				}
			case training.BlockRepeat:
				n, err := strconv.Atoi(db.Count)
				if err != nil {
					errs.add(key, "Repeat count: must be a whole number")
					parseFailed[key+"|count"] = true
				}
				b.Count = n
				b.Blocks = convert(db.Children, p)
			}
			out[i] = b
		}
		return out
	}

	t := training.Training{
		Title:  strings.TrimSpace(d.Title),
		Date:   d.Date,
		Notes:  strings.TrimSpace(d.Notes),
		Blocks: convert(d.Blocks, nil),
	}

	if verr, ok := t.Validate().(*training.ValidationError); ok {
		for _, prob := range verr.Problems {
			if prob.Block == nil {
				errs.add(prob.Field, prob.Msg)
				continue
			}
			key := blockPath(prob.Block).String()
			if parseFailed[key+"|"+prob.Field] {
				continue
			}
			label := fieldLabel(prob.Field)
			if label != "" {
				label += ": "
			}
			errs.add(key, label+prob.Msg)
		}
	}
	if len(errs) > 0 {
		return t, errs
	}
	return t, nil
}

func fieldLabel(field string) string {
	switch field {
	case "duration":
		return "Duration"
	case "zone":
		return "Zone"
	case "count":
		return "Repeat count"
	case "kind":
		return "Type"
	case "blocks":
		return "Steps"
	}
	return ""
}

// parseDuration converts the editor's amount and unit into a duration.
// Minutes accept "15", "1.5" or "1:30"; kilometers accept "2.5".
func parseDuration(value, unit string) (*training.Duration, string) {
	if value == "" {
		return nil, "required"
	}
	var amount float64
	if unit == unitMin && strings.Contains(value, ":") {
		m, s, _ := strings.Cut(value, ":")
		mi, err1 := strconv.Atoi(m)
		si, err2 := strconv.Atoi(s)
		if err1 != nil || err2 != nil || mi < 0 || si < 0 || si >= 60 {
			return nil, `use minutes like "15", "1.5" or "1:30"`
		}
		amount = float64(mi) + float64(si)/60
	} else {
		f, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return nil, "must be a number"
		}
		amount = f
	}

	var factor float64
	switch unit {
	case unitMin:
		factor = 60
	case unitSec, unitM:
		factor = 1
	case unitKm:
		factor = 1000
	default:
		return nil, "invalid unit"
	}
	n := math.Round(amount * factor)
	if n <= 0 {
		return nil, "must be greater than 0"
	}
	if n > 1e7 {
		return nil, "too large"
	}
	v := int(n)
	if unit == unitMin || unit == unitSec {
		return &training.Duration{TimeS: &v}, ""
	}
	return &training.Duration{DistanceM: &v}, ""
}

// --- Handlers ---

func (h *Handler) newTrainingPage(w http.ResponseWriter, r *http.Request) {
	if !auth.IsAdmin(r.Context()) {
		h.notFound(w, r)
		return
	}
	d := newDraft(today())
	if date := r.URL.Query().Get("date"); date != "" {
		if _, err := time.Parse(training.DateLayout, date); err == nil {
			d.Date = date
		}
	}
	render(w, r, http.StatusOK, newTrainingPage(d, nil))
}

// createTraining handles every submit of the editor form. With an "op"
// value it applies that edit and shows the form again; without one it
// saves the training.
func (h *Handler) createTraining(w http.ResponseWriter, r *http.Request) {
	if !auth.IsAdmin(r.Context()) {
		h.notFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	d, err := parseDraft(r.PostForm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if op := r.PostForm.Get("op"); op != "" {
		if err := d.applyOp(op); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		render(w, r, http.StatusOK, newTrainingPage(d, nil))
		return
	}

	t, errs := d.toTraining()
	if errs != nil {
		// 200, not 422: htmx doesn't swap error responses by default.
		render(w, r, http.StatusOK, newTrainingPage(d, errs))
		return
	}

	date, _ := t.ParsedDate()
	blocks, err := json.Marshal(t.Blocks)
	if err != nil {
		h.serverError(w, r, "encode blocks", err)
		return
	}
	row, err := h.q.CreateTraining(r.Context(), store.CreateTrainingParams{
		Date:   pgtype.Date{Time: date, Valid: true},
		Title:  t.Title,
		Notes:  t.Notes,
		Blocks: blocks,
	})
	if err != nil {
		h.serverError(w, r, "create training", err)
		return
	}

	target := trainingURL(row.ID)
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Location", target)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}
