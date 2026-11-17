package rasterfront

import (
	"slices"

	"github.com/weaveplatform/spaceweave/internal/raster"
)

type axis uint8

const (
	horizontal axis = iota
	vertical
)

// piece is a straight band of wall mask on one axis, in pixel indices,
// inclusive: lo–hi along the wall, a0–a1 across it.
type piece struct {
	ax     axis
	lo, hi int
	a0, a1 int
	solid  float64 // share of the band's rectangle that is wall mask
}

func (p piece) length() int { return p.hi - p.lo + 1 }
func (p piece) thick() int  { return p.a1 - p.a0 + 1 }

// mid is the centreline across the band, in continuous pixel coordinates
// (pixel i spans i to i+1).
func (p piece) mid() float64 { return float64(p.a0+p.a1+1) / 2 }

// Band stacking tolerances. Scanned and JPEG'd plans break a wall's runs and
// rows by a pixel or two here and there; wider breaks are real (an opening is
// half a metre at least, many pixels on any plan worth tracing).
const (
	bridge   = 2 // along: ink gaps this short join the runs either side
	skipLine = 1 // across: a band survives this many lines without a run
)

// mask is a w×h boolean image viewed along one axis: lines are rows for
// horizontal walls and columns for vertical ones.
type mask struct {
	w, h int
	px   []bool
}

func (m mask) lines(ax axis) (n, length int) {
	if ax == horizontal {
		return m.h, m.w
	}
	return m.w, m.h
}

func (m mask) at(ax axis, line, i int) bool {
	if ax == horizontal {
		return m.px[line*m.w+i]
	}
	return m.px[i*m.w+line]
}

// runs is the line's runs of at least minRun px, after bridging short gaps.
func (m mask) runs(ax axis, line, minRun int, buf []raster.Run) []raster.Run {
	_, n := m.lines(ax)
	out := buf[:0]
	start, end := -1, -1
	for i := range n {
		if !m.at(ax, line, i) {
			continue
		}
		if start >= 0 && i-end-1 <= bridge {
			end = i
			continue
		}
		if start >= 0 && end-start+1 >= minRun {
			out = append(out, raster.Run{Start: start, End: end})
		}
		start, end = i, i
	}
	if start >= 0 && end-start+1 >= minRun {
		out = append(out, raster.Run{Start: start, End: end})
	}
	return out
}

// stack is a band being built line by line.
type stack struct {
	first, last  int
	starts, ends []int
}

// pieces finds the wall bands along one axis, the way raster.DetectWalls
// does, with three changes that matter on a mask of whole walls: a run joins
// a band only when it covers most of the longer of the two (raster's
// "shorter" lets a long wall's first line join the perpendicular wall it
// starts from), bands survive a missing line, and a band is kept only when it
// is longer than it is thick, since otherwise it is a slice across a
// perpendicular wall, which the other axis finds.
func (m mask) pieces(ax axis, minRun int) []piece {
	nLines, _ := m.lines(ax)
	var open, done []stack
	var buf []raster.Run
	for line := range nLines {
		buf = m.runs(ax, line, minRun, buf)
		used := make([]bool, len(buf))
		next := open[:0:0]
		for _, s := range open {
			if j := s.match(buf, used); j >= 0 {
				used[j] = true
				s.starts = append(s.starts, buf[j].Start)
				s.ends = append(s.ends, buf[j].End)
				s.last = line
				next = append(next, s)
			} else if line-s.last > skipLine {
				done = append(done, s)
			} else {
				next = append(next, s)
			}
		}
		for j, r := range buf {
			if !used[j] {
				next = append(
					next,
					stack{first: line, last: line, starts: []int{r.Start}, ends: []int{r.End}},
				)
			}
		}
		open = next
	}
	done = append(done, open...)
	out := make([]piece, 0, len(done))
	for _, s := range done {
		// Medians, not extremes: a wall's ends blur into the walls it meets.
		p := piece{ax: ax, lo: median(s.starts), hi: median(s.ends), a0: s.first, a1: s.last}
		if p.length() < p.thick() || (ax == vertical && p.length() == p.thick()) {
			// A square blob (a column) is reported once, as horizontal.
			continue
		}
		p.solid = m.solidity(p)
		out = append(out, p)
	}
	return out
}

func (s stack) match(runs []raster.Run, used []bool) int {
	ps, pe := s.starts[len(s.starts)-1], s.ends[len(s.ends)-1]
	for j, r := range runs {
		if used[j] {
			continue
		}
		overlap := min(pe, r.End) - max(ps, r.Start) + 1
		longer := max(pe-ps+1, r.Len())
		if overlap*5 >= longer*4 {
			return j
		}
	}
	return -1
}

func (m mask) solidity(p piece) float64 {
	n := 0
	for line := p.a0; line <= p.a1; line++ {
		for i := p.lo; i <= p.hi; i++ {
			if m.at(p.ax, line, i) {
				n++
			}
		}
	}
	return float64(n) / float64(p.length()*p.thick())
}

func median(v []int) int {
	s := slices.Clone(v)
	slices.Sort(s)
	return s[len(s)/2]
}

// merge joins collinear pieces that touch or overlap along their axis: one
// wall split by a stacking hiccup.
func merge(ps []piece) []piece {
	slices.SortFunc(ps, comparePieces)
	var out []piece
	for _, p := range ps {
		joined := false
		for i := len(out) - 1; i >= 0; i-- {
			q := &out[i]
			if q.ax == p.ax && abs(q.mid()-p.mid()) <= 1 && p.lo <= q.hi+1+bridge &&
				q.lo <= p.hi+1+bridge {
				n := float64(q.length()*q.thick() + p.length()*p.thick())
				q.solid = (q.solid*float64(q.length()*q.thick()) + p.solid*float64(p.length()*p.thick())) / n
				q.lo, q.hi = min(q.lo, p.lo), max(q.hi, p.hi)
				q.a0, q.a1 = min(q.a0, p.a0), max(q.a1, p.a1)
				joined = true
				break
			}
		}
		if !joined {
			out = append(out, p)
		}
	}
	slices.SortFunc(out, comparePieces)
	return out
}

func comparePieces(a, b piece) int {
	for _, d := range [...]int{int(a.ax) - int(b.ax), a.a0 - b.a0, a.lo - b.lo, a.a1 - b.a1, a.hi - b.hi} {
		if d != 0 {
			return d
		}
	}
	return 0
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
