package cadfront

import (
	"math"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/floorgraph"
)

// Doorway widths, in metres. A gap a door's ink lies in may be as wide as a
// pair of the widest leaves with their frames. A gap with nothing in it is
// a doorway only up to a single door's width: an open-plan floor's
// walkways past a partition's end are wider (1.2 m and up for a wheelchair),
// and closing those would split open plan into rooms the drawing doesn't
// have.
const (
	maxDoorway      = 3.0
	maxPlainDoorway = 1.0
	// minDoorway is the narrowest gap taken for a doorway: a cupboard's
	// leaf.
	minDoorway = 0.3
	// doorInkLen is the least door ink, in metres, that says a gap holds a
	// door: a frame's jamb or a threshold, not a stray tag's leader.
	doorInkLen = 0.1
	// doorwayStep is how finely door ink is sampled, in metres.
	doorwayStep = 0.05
)

// Opening weights for doorways found from the gap rather than a swing.
const (
	weightDoorway = 0.6 // door ink in the gap
	weightGap     = 0.4 // nothing in it
)

// sinParallel is how far from parallel, as a sine, two pieces of one wall
// either side of a doorway may be.
var sinParallel = math.Sin(2 * math.Pi / 180)

// doorways finds the doorways no door symbol explained: a gap between a
// wall's end and the next piece of the same wall, or the wall straight
// ahead, that door ink lies in (a door drawn ajar, a sliding or folding
// door, an entrance screen's leaves: none has the quarter-circle swing the
// door finders look for), or that is no wider than a door. Each becomes an
// opening from the end across the gap, so floorgraph joins the wall
// through it and the rooms either side close; ops are the openings found
// so far, and gaps one of them already spans are left to it.
func (r *reader) doorways(ops []opening) []opening {
	walls := r.wallGrid()
	ink := r.doorInk()
	reach := r.metres(maxDoorway)
	var out []opening
	type pairKey struct{ i, j int }
	seen := map[pairKey]bool{}
	for i, w := range r.walls {
		// A column's ends are its sides, not a wall's end at a doorway.
		if w.weight == weightColumn || w.b.Sub(w.a).Len() <= 1.5*w.thick {
			continue
		}
		for end := range 2 {
			e, other := w.a, w.b
			if end == 1 {
				e, other = w.b, w.a
			}
			if r.joined(walls, i, e) {
				continue
			}
			d := unit(e.Sub(other))
			j, s, inline := r.across(walls, i, e, d, reach)
			if j < 0 {
				continue
			}
			// Closer than that, floorgraph closes the corner itself.
			if s <= math.Max(r.metres(minDoorway), 1.5*math.Max(w.thick, r.walls[j].thick)) {
				continue
			}
			if inline {
				k := pairKey{min(i, j), max(i, j)}
				if seen[k] {
					continue
				}
				seen[k] = true
			}
			far := e.Add(d.Mul(s))
			if spanned(ops, e, far) {
				continue
			}
			half := w.thick/2 + r.metres(doorFrame)/2
			kind, weight := floorgraph.Gap, weightGap
			switch {
			case ink.along(e, d, s, half) >= r.metres(doorInkLen):
				kind, weight = floorgraph.Gap, weightDoorway
			case s > r.metres(maxPlainDoorway) || !inline:
				continue
			}
			out = append(out, opening{a: e, b: far, kind: kind, weight: weight})
		}
	}
	return out
}

// joined reports whether the end e of wall i already meets another wall:
// lies on it, within its thickness.
func (r *reader) joined(walls *grid, i int, e point) bool {
	for _, k := range walls.near(e, r.metres(maxGap)) {
		w := r.walls[k]
		if k != i && distToSeg(e, w.a, w.b) <= w.thick/2+r.metres(collinear) {
			return true
		}
	}
	return false
}

// across finds what the end e of wall i, heading d, faces across a gap of
// at most reach: the nearest of the next piece of the same wall (inline)
// or another wall's centreline straight ahead. j is -1 when there is
// neither; s is the gap's width.
func (r *reader) across(
	walls *grid, i int, e, d point, reach float64,
) (j int, s float64, inline bool) {
	self := r.walls[i]
	j, s = -1, math.Inf(1)
	for _, k := range walls.near(e, reach) {
		if k == i {
			continue
		}
		w := r.walls[k]
		wd := w.b.Sub(w.a)
		l := wd.Len()
		if l == 0 {
			continue
		}
		c := cross(d, wd) / l
		if math.Abs(c) <= sinParallel {
			// The next piece: its near end ahead, on this wall's line, and
			// the piece running on away from the gap.
			near, farEnd := w.a, w.b
			if dot(w.b.Sub(e), d) < dot(w.a.Sub(e), d) {
				near, farEnd = w.b, w.a
			}
			t := dot(near.Sub(e), d)
			off := math.Abs(cross(d, near.Sub(e)))
			if t > 0 && t <= reach && t < s && off <= math.Min(self.thick, w.thick)/2 &&
				dot(farEnd.Sub(near), d) > 0 {
				j, s, inline = k, t, true
			}
			continue
		}
		if math.Abs(c) < minCrossing {
			continue
		}
		t := cross(w.a.Sub(e), wd) / (c * l)
		if t <= 0 || t > reach || t >= s {
			continue
		}
		hit := e.Add(d.Mul(t))
		if u := dot(hit.Sub(w.a), wd) / l; u < -w.thick/2 || u > l+w.thick/2 {
			continue
		}
		j, s, inline = k, t, false
	}
	return j, s, inline
}

// spanned reports whether an opening found already lies across the gap
// from a to b: its middle within the gap, along it.
func spanned(ops []opening, a, b point) bool {
	d := b.Sub(a)
	l := d.Len()
	u := d.Mul(1 / l)
	for _, o := range ops {
		if o.a == o.b {
			continue
		}
		m := o.a.Add(o.b).Mul(0.5)
		t := dot(m.Sub(a), u)
		if t > 0 && t < l && math.Abs(cross(u, m.Sub(a))) <= l/2 &&
			math.Abs(cross(u, unit(o.b.Sub(o.a)))) <= parallelTol {
			return true
		}
	}
	return false
}

// inkPoints are sample points of door ink, indexed.
type inkPoints struct {
	pts  []point
	g    *grid
	step float64
}

// doorInk samples the ink of doors: loose entities on door layers and
// everything in door blocks, whether or not a door was made of them.
func (r *reader) doorInk() *inkPoints {
	ink := &inkPoints{g: newGrid(r.metres(gridCell)), step: r.metres(doorwayStep)}
	for i := range r.leaves {
		l := &r.leaves[i]
		inDoor := l.role == roleDoor && l.owner == nil && (l.node == nil || l.node.top == nil)
		if l.node != nil && l.node.top != nil {
			inDoor = l.node.top.role == roleDoor
		}
		if !inDoor {
			continue
		}
		switch l.e.(type) {
		case *cad.Line, *cad.Polyline:
			for _, sg := range r.straights([]int{i}) {
				ink.addLine(sg.a, sg.b)
			}
		case *cad.Arc, *cad.Ellipse:
			ps := samples(l.e)
			for k := 1; k < len(ps); k++ {
				ink.addLine(l.m.Apply(ps[k-1]), l.m.Apply(ps[k]))
			}
		}
	}
	return ink
}

func (k *inkPoints) addLine(a, b point) {
	n := max(1, int(math.Ceil(b.Sub(a).Len()/k.step)))
	for s := 0; s <= n; s++ {
		p := a.Add(b.Sub(a).Mul(float64(s) / float64(n)))
		k.g.add(len(k.pts), p, p)
		k.pts = append(k.pts, p)
	}
}

// along is how much door ink, in drawing units, lies in the gap that runs
// s from e along d, within half of its line.
func (k *inkPoints) along(e, d point, s, half float64) float64 {
	n := 0
	mid := e.Add(d.Mul(s / 2))
	for _, i := range k.g.near(mid, s/2+half) {
		p := k.pts[i].Sub(e)
		if t := dot(p, d); t > 0 && t < s && math.Abs(cross(d, p)) <= half {
			n++
		}
	}
	return float64(n) * k.step
}
