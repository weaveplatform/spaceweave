package floorgraph

import (
	"math"
	"slices"
)

// outOpening is an opening placed on an output wall: a and b along the
// wall's dominant axis, a < b, in px.
type outOpening struct {
	a, b               float64
	kind, swing, hinge string
	guessed            bool
	weight             float64
}

// minOpeningM is the narrowest opening kept, metres: narrower is a gap in
// the evidence, not a door, and would round to nothing in plan.json.
const minOpeningM = 0.1

// placement counts what happened to the openings on the way out.
type placement struct {
	placed, unplaced, narrow, guessed, overlapped int
	leafed, doors                                 int
}

// roomAt finds the room containing a point: its area in m², or false.
type roomAt func(Point) (float64, bool)

// placeOpenings puts each opening that found a wall during joining on the
// output wall it lies on, clipped to the wall's span.
func placeOpenings(cs []chain, ops []opening, sx, sy float64, rooms roomAt) placement {
	var pl placement
	for _, o := range ops {
		i, ta, tb := findChain(cs, o.Opening)
		if i < 0 {
			pl.unplaced++
			continue
		}
		c := &cs[i]
		d, l := c.dir(), dist(c.from, c.to)
		eps := 1.0 / quantum
		ta, tb = clamp(ta, eps, l-eps), clamp(tb, eps, l-eps)
		if math.Abs(tb-ta) < math.Max(minTol, minOpeningM/metresPerPx(d, sx, sy)) {
			pl.narrow++
			continue
		}
		pa, pb := qp(addScaled(c.from, d, ta)), qp(addScaled(c.from, d, tb))
		ax := axis(c.from, c.to)
		out := outOpening{a: coord(pa, ax), b: coord(pb, ax), kind: o.Kind, weight: o.Weight}
		aIsA := true
		if out.a > out.b {
			out.a, out.b, aIsA = out.b, out.a, false
		}
		if !(out.a < out.b) {
			pl.unplaced++
			continue
		}
		if o.Kind == Door || o.Kind == DoubleDoor {
			pl.doors++
			if o.Leafed {
				out.swing = c.side(o.Leaf)
			}
			if out.swing != "" {
				pl.leafed++
				if o.Kind == Door {
					out.hinge = map[bool]string{true: "a", false: "b"}[o.HingeA == aIsA]
				}
			} else {
				out.swing, out.guessed = c.guessSwing(mid(pa, pb), rooms), true
				pl.guessed++
			}
		}
		c.ops = append(c.ops, out)
		pl.placed++
	}
	// Prefer measured, confident hypotheses over guessed duplicates; positional
	// ordering otherwise lets a broad, weak opening erase an observed door.
	for i := range cs {
		slices.SortStableFunc(cs[i].ops, func(x, y outOpening) int {
			if n := cmpFloat(y.weight, x.weight); n != 0 {
				return n
			}
			if x.guessed != y.guessed {
				return cmpBool(x.guessed, y.guessed)
			}
			return cmpFloat(x.a, y.a)
		})
		kept := cs[i].ops[:0]
		for _, o := range cs[i].ops {
			if slices.ContainsFunc(
				kept,
				func(k outOpening) bool { return o.a < k.b && k.a < o.b },
			) {
				pl.overlapped++
				pl.placed--
				if o.guessed {
					pl.guessed--
				}
				continue
			}
			kept = append(kept, o)
		}
		slices.SortStableFunc(kept, func(x, y outOpening) int { return cmpFloat(x.a, y.a) })
		cs[i].ops = kept
	}
	return pl
}

// findChain is the output wall an opening lies on: same direction, both ends
// within half its thickness (plus slack for the merges that moved its line),
// and the opening's middle within its span. ta and tb are the opening's ends
// as distances along the wall from its start.
func findChain(cs []chain, o Opening) (best int, ta, tb float64) {
	best = -1
	bestKey := math.Inf(1)
	ou := canonical(unit(sub(o.B, o.A)))
	for i := range cs {
		c := &cs[i]
		d := c.dir()
		if math.Abs(cross(ou, canonical(d))) > sinOpening {
			continue
		}
		n := Point{X: -d.Y, Y: d.X}
		lim := math.Max(c.t/2, minTol) + 2
		pa, pb := math.Abs(dot(sub(o.A, c.from), n)), math.Abs(dot(sub(o.B, c.from), n))
		if pa > lim || pb > lim {
			continue
		}
		a, b := dot(sub(o.A, c.from), d), dot(sub(o.B, c.from), d)
		if m := (a + b) / 2; m < 0 || m > dist(c.from, c.to) {
			continue
		}
		if pa+pb < bestKey {
			best, bestKey, ta, tb = i, pa+pb, a, b
		}
	}
	return best, ta, tb
}

// side is the compass side of the wall p lies on: N or S of a mostly
// horizontal wall, W or E of a mostly vertical one, "" on the line.
func (c *chain) side(p Point) string {
	d := c.dir()
	on := addScaled(c.from, d, dot(sub(p, c.from), d))
	if axis(c.from, c.to) == 0 {
		switch {
		case p.Y < on.Y:
			return "N"
		case p.Y > on.Y:
			return "S"
		}
		return ""
	}
	switch {
	case p.X < on.X:
		return "W"
	case p.X > on.X:
		return "E"
	}
	return ""
}

// guessSwing opens a door with no leaf found into the smaller room beside
// it (doors open into offices and toilets, not corridors), or N or E when
// neither side is a room.
func (c *chain) guessSwing(m Point, rooms roomAt) string {
	d := c.dir()
	n := Point{X: -d.Y, Y: d.X}
	off := c.t/2 + 3
	p1, p2 := addScaled(m, n, off), addScaled(m, n, -off)
	a1, ok1 := rooms(p1)
	a2, ok2 := rooms(p2)
	switch {
	case ok1 && (!ok2 || a1 <= a2):
		return c.side(p1)
	case ok2:
		return c.side(p2)
	case axis(c.from, c.to) == 0:
		return "N"
	}
	return "E"
}
