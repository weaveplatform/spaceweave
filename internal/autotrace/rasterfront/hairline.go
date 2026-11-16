package rasterfront

import "slices"

// Hairline partitions (glazed screens, WC cubicles, lift shafts drawn with
// the same pen as the furniture) can't be told from furniture by width. What
// tells them apart is that a partition runs from wall to wall: a desk's
// edge, a door leaf or a dimension line ends in the room. A straight
// hairline, or a row of them broken by door-sized gaps, is taken as a wall
// when both its ends meet a wall, and partitions meeting partitions count
// once those are walls.
const (
	// hairMinLen is the shortest hairline partition, in ordinary partition
	// thicknesses: about half a metre.
	hairMinLen = 5
	// hairReach is how far, px, past a hairline's end a wall may stop and
	// still be the one it meets; hairSide how far to either side.
	hairReach = 3
	hairSide  = 2
	// hairRoom is the least clear floor, in ordinary partition
	// thicknesses, a partition needs on both sides: about 0.8 m, a WC
	// cubicle. A cabinet's front edge runs wall to wall too, but with the
	// wall it stands against a cabinet's depth behind it.
	hairRoom = 8
)

// chainOf is a row of collinear hairline pieces.
type chainOf struct {
	pieces      []piece
	ok, glazing bool
}

// hairlines finds hairline partitions, adds them to the wall mask and to
// r.pieces, and returns them as pieces to report.
func (r *reader) hairlines(thinnest, tRef int, kept []piece, maxGap float64) []piece {
	thin := mask{w: r.w, h: r.h, px: make([]bool, len(r.ink.px))}
	for i, v := range r.ink.px {
		thin.px[i] = v && !r.wall.px[i]
	}
	minLen := max(hairMinLen*tRef, 4*thinnest)
	var cands []piece
	for _, ax := range [2]axis{horizontal, vertical} {
		cands = append(cands, thin.pieces(ax, minLen)...)
	}
	cands = merge(cands)
	walls := lines(kept)
	cands = slices.DeleteFunc(cands, func(p piece) bool {
		return p.length() < minLen || p.thick() >= thinnest || p.solid < minSolid ||
			alongWall(p, walls)
	})
	cands = slices.DeleteFunc(cands, func(p piece) bool {
		return r.wall.clearance(p, -1, hairRoom*tRef) < hairRoom*tRef ||
			r.wall.clearance(p, 1, hairRoom*tRef) < hairRoom*tRef
	})
	chains := chainUp(cands, maxGap, r.wall)
	// Two or more hairlines side by side, close enough to share a wall's
	// band, are a window's panes or a thin element drawn double, not a
	// partition.
	for i := range chains {
		for j := range chains {
			if i != j && sideBySide(chains[i], chains[j], thinnest) {
				chains[i].glazing = true
			}
		}
	}
	accepted := mask{w: r.w, h: r.h, px: make([]bool, len(r.ink.px))}
	// Partitions meeting walls at both ends first; then those meeting a
	// wall at one end and one of those at the other, such as a cubicle's
	// side wall meeting its front. Hairlines touching only each other are
	// furniture.
	for pass := range 2 {
		var took []int
		for i := range chains {
			c := &chains[i]
			if c.ok || c.glazing {
				continue
			}
			first, last := c.pieces[0], c.pieces[len(c.pieces)-1]
			solidLo, lo, glazedLo := r.anchored(first, first.lo, -1, accepted)
			solidHi, hi, glazedHi := r.anchored(last, last.hi, 1, accepted)
			if glazedLo || glazedHi {
				c.glazing = true
				continue
			}
			if solidLo && solidHi || pass == 1 && lo && hi && (solidLo || solidHi) {
				took = append(took, i)
			}
		}
		for _, i := range took {
			chains[i].ok = true
			for _, p := range chains[i].pieces {
				paint(accepted, p)
			}
		}
	}
	var out []piece
	for _, c := range chains {
		if c.ok {
			out = append(out, c.pieces...)
		}
	}
	for _, p := range out {
		paint(r.wall, p)
	}
	r.pieces = append(r.pieces, out...)
	slices.SortFunc(r.pieces, comparePieces)
	return out
}

// alongWall says p lies in, or against the face of, a wall line it runs
// along: glazing in a window, or a counter's edge against the wall.
func alongWall(p piece, ls []wallLine) bool {
	for _, l := range ls {
		if l.ax != p.ax {
			continue
		}
		lo, hi := l.pieces[0].lo, l.pieces[0].hi
		for _, q := range l.pieces {
			lo, hi = min(lo, q.lo), max(hi, q.hi)
		}
		half := float64(l.a1-l.a0+1) / 2
		if abs(p.mid()-l.mid) <= half+hairReach && p.lo <= hi && p.hi >= lo {
			return true
		}
	}
	return false
}

// sideBySide says chains a and b run the same way within a wall's
// thickness of each other, overlapping along it.
func sideBySide(a, b chainOf, thinnest int) bool {
	p, q := a.pieces[0], b.pieces[0]
	if p.ax != q.ax || abs(p.mid()-q.mid()) > float64(2*thinnest) {
		return false
	}
	alo, ahi := p.lo, a.pieces[len(a.pieces)-1].hi
	blo, bhi := q.lo, b.pieces[len(b.pieces)-1].hi
	return min(ahi, bhi)-max(alo, blo) > (ahi-alo)/2
}

// chainUp groups collinear pieces into rows whose gaps are at most maxGap
// and cross no wall: a door's width, not two rooms' furniture lined up
// either side of the wall between them.
func chainUp(ps []piece, maxGap float64, wall mask) []chainOf {
	sorted := slices.Clone(ps)
	slices.SortFunc(sorted, func(a, b piece) int {
		if a.ax != b.ax {
			return int(a.ax) - int(b.ax)
		}
		if c := cmpFloat(a.mid(), b.mid()); c != 0 {
			return c
		}
		return a.lo - b.lo
	})
	var out []chainOf
	for _, p := range sorted {
		if n := len(out); n > 0 {
			c := &out[n-1]
			q := c.pieces[len(c.pieces)-1]
			if q.ax == p.ax && abs(q.mid()-p.mid()) <= 1 && float64(p.lo-q.hi-1) <= maxGap &&
				!wall.crosses(q.ax, int(q.mid()), q.hi+1, p.lo-1) {
				c.pieces = append(c.pieces, p)
				continue
			}
		}
		out = append(out, chainOf{pieces: []piece{p}})
	}
	return out
}

// anchored says what lies just past the end of p at s, looking in
// direction dir along it: a wall (solid), an accepted partition (ok), or a
// wall running the same way as p with p's line in its band, off its centre
// (glazing): p runs into it end-on, as glazing does between a window's
// jambs. A glazed screen carrying on from a solid wall is on its centre.
func (r *reader) anchored(p piece, s, dir int, accepted mask) (solid, ok, glazing bool) {
	l := wallLine{ax: p.ax, mid: p.mid()}
	for k := 0; k <= hairReach; k++ {
		for off := -hairSide; off <= hairSide; off++ {
			pt := l.point(float64(s+dir*k)+0.5, float64(off))
			x, y := int(pt.X), int(pt.Y)
			if x < 0 || y < 0 || x >= r.w || y >= r.h {
				continue
			}
			i := y*r.w + x
			if r.wall.px[i] {
				if q, on := r.endOn(p, x, y); on && abs(q.mid()-p.mid()) > 1 {
					return false, false, true
				}
				solid = true
			}
			ok = ok || accepted.px[i]
		}
	}
	return solid, ok || solid, false
}

// endOn says the wall pixel (x, y) belongs to a piece running the same way
// as p whose band holds p's line.
func (r *reader) endOn(p piece, x, y int) (piece, bool) {
	for _, q := range r.pieces {
		b := q.rect()
		if q.ax == p.ax && x >= b[0] && x <= b[2] && y >= b[1] && y <= b[3] &&
			abs(q.mid()-p.mid()) <= float64(q.thick())/2+1 {
			return q, true
		}
	}
	return piece{}, false
}

// clearance is the median distance, px, from p's side towards dir to the
// next ink in m, at five points along it, up to limit.
func (m mask) clearance(p piece, dir, limit int) int {
	var ds []int
	for k := 1; k <= 9; k += 2 {
		s := p.lo + k*p.length()/10
		from := p.a1 + 1
		if dir < 0 {
			from = p.a0 - 1
		}
		d := 0
		for a := from; d < limit; a += dir {
			n, _ := m.lines(p.ax)
			if a < 0 || a >= n || m.at(p.ax, a, s) {
				break
			}
			d++
		}
		ds = append(ds, d)
	}
	slices.Sort(ds)
	return ds[len(ds)/2]
}

// crosses says m has a pixel on line between along positions s0 and s1.
func (m mask) crosses(ax axis, line, s0, s1 int) bool {
	n, _ := m.lines(ax)
	if line < 0 || line >= n {
		return false
	}
	for s := s0; s <= s1; s++ {
		if m.at(ax, line, s) {
			return true
		}
	}
	return false
}

// paint sets p's rectangle in m.
func paint(m mask, p piece) {
	b := p.rect()
	for y := max(b[1], 0); y <= min(b[3], m.h-1); y++ {
		for x := max(b[0], 0); x <= min(b[2], m.w-1); x++ {
			m.px[y*m.w+x] = true
		}
	}
}
