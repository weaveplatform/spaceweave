package raster

import (
	"image"
	"slices"
	"sort"
)

// Run is a stretch of consecutive ink pixels along a scan line, in source
// pixels, end inclusive.
type Run struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Len is the run's length in pixels.
func (r Run) Len() int { return r.End - r.Start + 1 }

// MaxRuns bounds one scan's output; a scan across hatching or text would
// otherwise return hundreds of one-pixel runs and flood the model's context.
const MaxRuns = 200

// ScanRow returns up to MaxRuns ink runs along row y from x0 to x1.
func (s *Source) ScanRow(y, x0, x1 int) []Run { return s.row(y, x0, x1, MaxRuns) }

// ScanCol returns up to MaxRuns ink runs down column x from y0 to y1.
func (s *Source) ScanCol(x, y0, y1 int) []Run { return s.col(x, y0, y1, MaxRuns) }

func (s *Source) row(y, x0, x1, limit int) []Run {
	b := s.Image.Bounds()
	return scan(
		func(i int) bool { return s.Ink(i, y) },
		x0,
		x1,
		b.Min.X,
		b.Max.X,
		y >= b.Min.Y && y < b.Max.Y,
		limit,
	)
}

func (s *Source) col(x, y0, y1, limit int) []Run {
	b := s.Image.Bounds()
	return scan(
		func(i int) bool { return s.Ink(x, i) },
		y0,
		y1,
		b.Min.Y,
		b.Max.Y,
		x >= b.Min.X && x < b.Max.X,
		limit,
	)
}

// scan collects ink runs between from and to, clamped to [lo, hi); limit 0
// means no limit.
func scan(ink func(int) bool, from, to, lo, hi int, onImage bool, limit int) []Run {
	if !onImage {
		return nil
	}
	if from > to {
		from, to = to, from
	}
	from, to = max(from, lo), min(to, hi-1)
	var runs []Run
	start := -1
	for i := from; i <= to+1; i++ {
		on := i <= to && ink(i)
		switch {
		case on && start < 0:
			start = i
		case !on && start >= 0:
			runs = append(runs, Run{Start: start, End: i - 1})
			start = -1
			if len(runs) == limit {
				return runs
			}
		}
	}
	return runs
}

// Wall is a candidate wall found by DetectWalls: an axis-aligned band of ink
// with its centreline, extent and thickness in source pixels.
type Wall struct {
	Axis      string `json:"axis"` // "h" or "v"
	From      [2]int `json:"from"`
	To        [2]int `json:"to"`
	Thickness int    `json:"thickness"`
}

// WallLimits filters DetectWalls' candidates.
type WallLimits struct {
	MinThickness, MaxThickness, MinLength int
	Max                                   int // most candidates to return, longest first
}

// DetectWalls proposes axis-aligned walls inside region: bands of ink at least
// MinLength long whose thickness is within limits. It deliberately misses
// slanted walls and over-reports (furniture edges, thick text), so the model
// treats the result as candidates to confirm against the picture.
func (s *Source) DetectWalls(region image.Rectangle, lim WallLimits) ([]Wall, error) {
	r, err := s.clip(region)
	if err != nil {
		return nil, err
	}
	// Horizontal walls are runs along rows stacked down; vertical walls are the
	// same search with the axes swapped.
	horizontal := bands(
		r.Min.Y,
		r.Max.Y,
		func(y int) []Run { return longRuns(s.row(y, r.Min.X, r.Max.X-1, 0), lim.MinLength) },
		lim,
	)
	vertical := bands(
		r.Min.X,
		r.Max.X,
		func(x int) []Run { return longRuns(s.col(x, r.Min.Y, r.Max.Y-1, 0), lim.MinLength) },
		lim,
	)

	walls := make([]Wall, 0, len(horizontal)+len(vertical))
	for _, b := range horizontal {
		walls = append(
			walls,
			Wall{Axis: "h", From: [2]int{b.lo, b.mid}, To: [2]int{b.hi, b.mid}, Thickness: b.thick},
		)
	}
	for _, b := range vertical {
		walls = append(
			walls,
			Wall{Axis: "v", From: [2]int{b.mid, b.lo}, To: [2]int{b.mid, b.hi}, Thickness: b.thick},
		)
	}
	sort.SliceStable(walls, func(i, j int) bool { return length(walls[i]) > length(walls[j]) })
	if lim.Max > 0 && len(walls) > lim.Max {
		walls = walls[:lim.Max]
	}
	return walls, nil
}

func length(w Wall) int { return w.To[0] - w.From[0] + w.To[1] - w.From[1] }

// Wall detection filters an untruncated scan: MaxRuns would hide long runs
// past the cut on a busy line.
func longRuns(runs []Run, minLen int) []Run {
	return slices.DeleteFunc(runs, func(r Run) bool { return r.Len() < minLen })
}

type band struct {
	lo, hi    int // extent along the wall
	mid       int // centreline across it
	thick     int
	start     int // first line across
	starts    []int
	ends      []int
	lastAlong int // last line index that extended the band
}

// bands stacks long runs on consecutive scan lines into bands. A run joins a
// band from the previous line when they overlap by most of the shorter run.
func bands(from, to int, runsAt func(int) []Run, lim WallLimits) []band {
	var open, done []band
	for line := from; line < to; line++ {
		runs := runsAt(line)
		var next []band
		used := make([]bool, len(runs))
		for _, b := range open {
			matched := false
			for i, r := range runs {
				if used[i] || !overlaps(b, r) {
					continue
				}
				used[i] = true
				matched = true
				b.starts = append(b.starts, r.Start)
				b.ends = append(b.ends, r.End)
				b.lastAlong = line
				next = append(next, b)
				break
			}
			if !matched {
				done = append(done, b)
			}
		}
		for i, r := range runs {
			if !used[i] {
				next = append(
					next,
					band{start: line, starts: []int{r.Start}, ends: []int{r.End}, lastAlong: line},
				)
			}
		}
		open = next
	}
	done = append(done, open...)

	var out []band
	for _, b := range done {
		b.thick = b.lastAlong - b.start + 1
		if b.thick < lim.MinThickness || b.thick > lim.MaxThickness {
			continue
		}
		// Medians, not min/max: a wall's ends blur into the walls it meets.
		b.lo, b.hi = median(b.starts), median(b.ends)
		b.mid = b.start + b.thick/2
		out = append(out, b)
	}
	return out
}

func overlaps(b band, r Run) bool {
	ps, pe := b.starts[len(b.starts)-1], b.ends[len(b.ends)-1]
	overlap := min(pe, r.End) - max(ps, r.Start) + 1
	shorter := min(pe-ps+1, r.Len())
	return overlap*5 >= shorter*4
}

func median(v []int) int {
	s := slices.Clone(v)
	slices.Sort(s)
	return s[len(s)/2]
}
