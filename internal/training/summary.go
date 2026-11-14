package training

// Summary is the total planned volume of a list of blocks, with repeats
// multiplied out. Time-based and distance-based steps are summed
// separately since we can't convert between them without a pace.
type Summary struct {
	TimeS     int
	DistanceM int
}

// Summarize returns the total time and distance of blocks.
func Summarize(blocks []Block) Summary {
	var s Summary
	for _, b := range blocks {
		switch b.Type {
		case BlockStep:
			if b.Duration == nil {
				continue
			}
			if b.Duration.TimeS != nil {
				s.TimeS += *b.Duration.TimeS
			}
			if b.Duration.DistanceM != nil {
				s.DistanceM += *b.Duration.DistanceM
			}
		case BlockRepeat:
			inner := Summarize(b.Blocks)
			s.TimeS += inner.TimeS * b.Count
			s.DistanceM += inner.DistanceM * b.Count
		}
	}
	return s
}
