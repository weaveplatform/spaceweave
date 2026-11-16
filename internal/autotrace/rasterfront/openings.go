package rasterfront

import (
	"math"
	"slices"

	"github.com/weaveplatform/spaceweave/internal/floorgraph"
)

// wallLine is a run of collinear wall pieces: one wall line of the plan,
// which openings interrupt.
type wallLine struct {
	ax     axis
	a0, a1 int // the lines across it that the profile reads
	mid    float64
	pieces []piece // sorted along
}

// lines groups pieces whose centrelines agree within half the thinner one's
// thickness. Half the thicker would let a column sitting against an
// exterior wall join the wall's line and drag its centreline off the wall.
func lines(ps []piece) []wallLine {
	sorted := slices.Clone(ps)
	slices.SortFunc(sorted, func(a, b piece) int {
		if a.ax != b.ax {
			return int(a.ax) - int(b.ax)
		}
		return cmpFloat(a.mid(), b.mid())
	})
	var out []wallLine
	var weight float64
	for _, p := range sorted {
		if n := len(out); n > 0 {
			l := &out[n-1]
			thinnest := slices.MinFunc(l.pieces, func(a, b piece) int { return a.thick() - b.thick() }).
				thick()
			if l.ax == p.ax && abs(l.mid-p.mid()) <= float64(max(1, min(thinnest, p.thick())))/2 {
				w := float64(p.length())
				l.mid = (l.mid*weight + p.mid()*w) / (weight + w)
				weight += w
				l.pieces = append(l.pieces, p)
				l.a0, l.a1 = min(l.a0, p.a0), max(l.a1, p.a1)
				continue
			}
		}
		weight = float64(p.length())
		out = append(out, wallLine{ax: p.ax, a0: p.a0, a1: p.a1, mid: p.mid(), pieces: []piece{p}})
	}
	for i := range out {
		slices.SortFunc(out[i].pieces, func(a, b piece) int { return a.lo - b.lo })
	}
	return out
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// Profile states along a wall line.
const (
	_        = iota // paper: nothing crosses the band
	profThin        // hairline ink crosses the wall's band here
	profWall        // wall mask crosses it
)

// gapLimits bound the openings looked for, px along the wall.
type gapLimits struct {
	minGap, maxGap, minWindow float64
	// squashes are the ratios of px across to px along, for the same
	// length on the building, that door arcs are looked for at: the known
	// one, or a spread when the scale is unknown, since published plans are
	// often stretched on one axis by a few per cent up to about 15.
	squashes []float64
}

// Classification thresholds for the stretch between two wall jambs.
const (
	// glazed is the share of a gap's length that must carry hairline ink
	// inside the wall's band for it to read as glazing: the lines run the
	// whole way, but noise may eat a pixel or two.
	glazed = 0.85
	// openInk is the most of a door gap that may carry ink: a door's leaf and
	// arc cross the band only at its two ends.
	openInk = 0.5
	// arcHit is the share of samples along a candidate swing arc that must
	// find ink for the gap to be called a door.
	arcHit = 0.8
)

type gap struct {
	g0, g1 int // the non-wall stretch, inclusive
	ink    float64
	window bool
	// touches says a reported wall piece ends at the gap; jambs counts
	// the ends where a piece of this wall line ends, however short, and
	// alike says there are two and they are about as thick.
	touches bool
	jambs   int
	alike   bool
	// free says neither jamb is inside a crossing wall.
	free bool
	ok   bool
}

// openings finds windows, doors and plain gaps along one wall line, and the
// stubs of wall they hang from that were too short to report as pieces.
func (r *reader) openings(l wallLine, lim gapLimits) ([]floorgraph.Opening, []floorgraph.Segment) {
	gaps, prof := r.gaps(l, lim)
	var out []floorgraph.Opening
	var stubs []floorgraph.Segment
	done := map[int]bool{}
	for _, g := range gaps {
		if !g.ok {
			continue
		}
		width := float64(g.g1 - g.g0 + 1)
		a, b := l.point(float64(g.g0), 0), l.point(float64(g.g1+1), 0)
		var o floorgraph.Opening
		switch {
		case g.window:
			o = floorgraph.Opening{A: a, B: b, Kind: floorgraph.Window, Weight: 0.8 * g.ink}
		case g.ink <= openInk && width >= lim.minGap && width <= lim.maxGap:
			// A door symbol vouches for a gap on its own. A bare gap needs
			// the one wall carrying on past both jambs: between a wall end
			// and a crossing wall, between two junctions, or between a wall
			// and a column, is as likely the open side of a room or a
			// corridor.
			if o = r.door(l, g, lim); !o.Leafed && (!g.alike || !g.free) {
				continue
			}
		default:
			continue
		}
		out = append(out, o)
		for _, s := range [2][2]int{stretch(prof, g.g0-1, -1), stretch(prof, g.g1+1, 1)} {
			if !done[s[0]] && !covered(l.pieces, s) {
				done[s[0]] = true
				stubs = append(stubs, l.stub(s))
			}
		}
	}
	return out, stubs
}

// stretch is the run of wall along a profile from i stepping by dir, as
// first and last index.
func stretch(prof []uint8, i, dir int) [2]int {
	j := i
	for j+dir >= 0 && j+dir < len(prof) && prof[j+dir] == profWall {
		j += dir
	}
	return [2]int{min(i, j), max(i, j)}
}

func covered(ps []piece, s [2]int) bool {
	for _, p := range ps {
		if p.lo <= s[1] && p.hi >= s[0] {
			return true
		}
	}
	return false
}

// stub is a jamb too short to have been reported as a wall piece (a mullion
// between panes, a corner seen end-on): floorgraph needs wall either side of
// an opening to hang it on. The weight is low because the stub is evidence
// only through the opening.
func (l wallLine) stub(s [2]int) floorgraph.Segment {
	thick := l.pieces[0].thick()
	for _, p := range l.pieces {
		thick = min(thick, p.thick())
	}
	return floorgraph.Segment{
		A:         l.point(float64(s[0]), 0),
		B:         l.point(float64(s[1]+1), 0),
		Thickness: float64(thick),
		Weight:    0.3,
	}
}

// gaps lists the stretches of l between two pieces of wall, with what
// crosses them.
func (r *reader) gaps(l wallLine, lim gapLimits) ([]gap, []uint8) {
	prof, offs := r.profile(l)
	same := r.collinear(l)
	half := float64(l.a1-l.a0+1) / 2
	var gaps []gap
	for i := 0; i < len(prof); {
		if prof[i] == profWall {
			i++
			continue
		}
		j := i
		inked, off := 0, 0.0
		for j < len(prof) && prof[j] != profWall {
			if prof[j] == profThin {
				inked++
				off += offs[j]
			}
			j++
		}
		// Only stretches with wall at both ends are openings; one running
		// off the image is just the end of a wall.
		if i > 0 && j < len(prof) {
			g := gap{g0: i, g1: j - 1, ink: float64(inked) / float64(j-i)}
			// Glazing sits symmetrically in its wall. Hairlines across a
			// gap but off to one side belong to something else: a parallel
			// wall's window, or furniture along the wall.
			centred := inked > 0 && abs(off/float64(inked)) <= max(1, half/2)
			g.window = g.ink >= glazed && centred && float64(j-i) >= lim.minWindow
			lo, hi := jambs(l.pieces, g)
			g.touches = lo > 0 || hi > 0
			lo, hi = jambs(same, g)
			g.jambs = min(lo, 1) + min(hi, 1)
			g.alike = g.jambs == 2 && 2*max(lo, hi) <= 3*min(lo, hi)
			g.free = !r.junction(l, i-1) && !r.junction(l, j)
			gaps = append(gaps, g)
		}
		i = j
	}
	// A run of windows split by mullions belongs to the wall if any of them
	// meets a piece of it: the mullions themselves may be too small to have
	// stacked into a piece. Glazing needs only the one jamb; a door needs a
	// reported wall piece.
	for i := range gaps {
		gaps[i].ok = gaps[i].touches || (gaps[i].window && gaps[i].jambs > 0)
	}
	chained := func(i, j int) bool {
		return gaps[i].window && gaps[j].window && gaps[j].ok && mullion(gaps[i], gaps[j], l)
	}
	for i := 1; i < len(gaps); i++ {
		gaps[i].ok = gaps[i].ok || chained(i, i-1)
	}
	for i := len(gaps) - 2; i >= 0; i-- {
		gaps[i].ok = gaps[i].ok || chained(i, i+1)
	}
	return gaps, prof
}

// junction says the wall at s along l lies inside a crossing wall.
func (r *reader) junction(l wallLine, s int) bool {
	for _, q := range r.pieces {
		if q.ax != l.ax && s >= q.a0-1 && s <= q.a1+1 && l.mid >= float64(q.lo-1) &&
			l.mid <= float64(q.hi+2) {
			return true
		}
	}
	return false
}

// mullion says the wall between two neighbouring gaps is short enough to be
// a mullion between panes rather than a stretch of wall in its own right.
func mullion(a, b gap, l wallLine) bool {
	between := max(a.g0, b.g0) - min(a.g1, b.g1) - 1
	return between <= 2*(l.a1-l.a0+1)
}

// jambs is the thickness of the thickest of ps ending at each end of g, or
// 0 where none does.
func jambs(ps []piece, g gap) (lo, hi int) {
	const slack = 2
	for _, p := range ps {
		if abs(float64(p.hi+1-g.g0)) <= slack {
			lo = max(lo, p.thick())
		}
		if abs(float64(p.lo-(g.g1+1))) <= slack {
			hi = max(hi, p.thick())
		}
	}
	return lo, hi
}

// collinear is every piece on l's line, including those too short to
// report.
func (r *reader) collinear(l wallLine) []piece {
	var out []piece
	for _, p := range r.pieces {
		if p.ax == l.ax && abs(p.mid()-l.mid) <= float64(l.a1-l.a0+1)/2 {
			out = append(out, p)
		}
	}
	return out
}

// profile reads the wall line's band across its whole length: for each
// position along it, whether wall, hairline ink or nothing crosses the band,
// and for hairlines their mean offset from the centreline.
func (r *reader) profile(l wallLine) ([]uint8, []float64) {
	_, n := r.wall.lines(l.ax)
	prof, offs := make([]uint8, n), make([]float64, n)
	for i := range n {
		sum, k := 0.0, 0
		for a := l.a0; a <= l.a1; a++ {
			if r.wall.at(l.ax, a, i) {
				prof[i] = profWall
				break
			}
			if r.ink.at(l.ax, a, i) {
				prof[i] = profThin
				sum += float64(a) + 0.5 - l.mid
				k++
			}
		}
		if prof[i] == profThin {
			offs[i] = sum / float64(k)
		}
	}
	return prof, offs
}

// point is the image point at s along the wall line and off across it.
func (l wallLine) point(s, off float64) floorgraph.Point {
	if l.ax == horizontal {
		return floorgraph.Point{X: s, Y: l.mid + off}
	}
	return floorgraph.Point{X: l.mid + off, Y: s}
}

// door decides what fills a door-sized gap by looking for the quarter-circle
// swing a door symbol draws: one arc as wide as the gap for a single leaf,
// two half-width arcs on the same side for a pair. Without one it is a plain
// gap; floorgraph decides about leaves the arc test can't see.
func (r *reader) door(l wallLine, g gap, lim gapLimits) floorgraph.Opening {
	s0, s1 := float64(g.g0), float64(g.g1+1)
	w := s1 - s0
	o := floorgraph.Opening{A: l.point(s0, 0), B: l.point(s1, 0), Kind: floorgraph.Gap, Weight: 0.5}
	best := 0.0
	for _, sq := range lim.squashes {
		for _, side := range [2]float64{-1, 1} {
			for _, hingeA := range [2]bool{true, false} {
				hinge, dir := s0, 1.0
				if !hingeA {
					hinge, dir = s1, -1
				}
				if f := r.arc(l, hinge, dir, side, w, sq); f >= arcHit && f > best {
					best = f
					o.Kind, o.HingeA, o.Leafed, o.Weight = floorgraph.Door, hingeA, true, 0.9
					o.Leaf = arcPoint(l, hinge, dir, side, w, sq, math.Pi/4)
				}
			}
			left := r.arc(l, s0, 1, side, w/2, sq)
			right := r.arc(l, s1, -1, side, w/2, sq)
			if f := min(left, right); f >= arcHit && f > best {
				best = f
				o.Kind, o.HingeA, o.Leafed, o.Weight = floorgraph.DoubleDoor, false, true, 0.9
				o.Leaf = arcPoint(l, s0, 1, side, w/2, sq, math.Pi/4)
			}
		}
	}
	return o
}

// arc is the share of points on a quarter circle about the hinge, from the
// closed leaf (along the wall, towards dir) to the open one (across, towards
// side), that have ink within two pixels. The ends are skipped: there the arc
// meets the leaf and the wall, which are inked anyway.
func (r *reader) arc(l wallLine, hinge, dir, side, radius, squash float64) float64 {
	n := max(8, int(radius*math.Pi/4))
	hits := 0
	for k := range n {
		th := (10 + 70*float64(k)/float64(n-1)) * math.Pi / 180
		p := arcPoint(l, hinge, dir, side, radius, squash, th)
		if r.inkNear(int(math.Floor(p.X)), int(math.Floor(p.Y)), 2) {
			hits++
		}
	}
	return float64(hits) / float64(n)
}

func arcPoint(l wallLine, hinge, dir, side, radius, squash, th float64) floorgraph.Point {
	return l.point(hinge+dir*radius*math.Cos(th), side*radius*squash*math.Sin(th))
}

func (r *reader) inkNear(x, y, d int) bool {
	for yy := max(y-d, 0); yy <= min(y+d, r.h-1); yy++ {
		for xx := max(x-d, 0); xx <= min(x+d, r.w-1); xx++ {
			if r.ink.px[yy*r.w+xx] {
				return true
			}
		}
	}
	return false
}
