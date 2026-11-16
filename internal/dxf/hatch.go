package dxf

import (
	"math"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// cursor walks a hatch's pairs, whose meaning depends on their order.
type cursor struct {
	g fields
	i int
}

// want returns the next pair if it has the code, and moves past it;
// otherwise it stays put, so a missing optional group doesn't throw the
// rest out of step.
func (c *cursor) want(code int) (pair, bool) {
	if c.i < len(c.g) && c.g[c.i].code == code {
		c.i++
		return c.g[c.i-1], true
	}
	return pair{}, false
}

func (c *cursor) float(code int) float64 {
	p, _ := c.want(code)
	return p.float()
}

func (c *cursor) int(code int) int {
	p, _ := c.want(code)
	return p.int()
}

func (c *cursor) point(x int) cad.Point {
	return cad.Point{X: c.float(x), Y: c.float(x + 10)}
}

// count reads a count and caps it at what the remaining pairs could hold,
// so a corrupt count can't make us allocate or loop for ever.
func (c *cursor) count(code, pairsEach int) int {
	n := c.int(code)
	return max(0, min(n, (len(c.g)-c.i)/max(pairsEach, 1)))
}

// Hatch boundary loop flags and edge types.
const (
	loopPolyline = 2
	edgeLine     = 1
	edgeArc      = 2
	edgeEllipse  = 3
	edgeSpline   = 4
)

// hatchBoundary reads the boundary loops into p, one closed subpath each.
func hatchBoundary(p *vector.Path, g fields) {
	c := &cursor{g: g}
	for c.i < len(g) && g[c.i].code != 91 {
		c.i++
	}
	loops := c.count(91, 2)
	for range loops {
		flags := c.int(92)
		if flags&loopPolyline != 0 {
			polylineLoop(p, c)
		} else {
			edgeLoop(p, c)
		}
		// Handles of the objects the boundary was picked from.
		for range c.count(97, 1) {
			c.want(330)
		}
	}
}

func polylineLoop(p *vector.Path, c *cursor) {
	bulges := c.int(72) != 0
	c.int(73) // closed: a boundary always is
	n := c.count(93, 2)
	vs := make([]cad.Vertex, 0, n)
	for range n {
		v := cad.Vertex{P: c.point(10)}
		if bulges {
			v.Bulge = c.float(42)
		}
		vs = append(vs, v)
	}
	cad.AddPolyline(p, vs, true)
}

func edgeLoop(p *vector.Path, c *cursor) {
	n := c.count(93, 3)
	start := len(p.Segs)
	for range n {
		join := len(p.Segs) > start
		switch c.int(72) {
		case edgeLine:
			a, b := c.point(10), c.point(11)
			if join {
				p.LineTo(a)
			} else {
				p.MoveTo(a)
			}
			p.LineTo(b)
		case edgeArc:
			ctr, r := c.point(10), c.float(40)
			t0, t1 := sweep(deg(c.float(50)), deg(c.float(51)), c.int(73) != 0)
			p.EllipseArc(ctr, cad.Point{X: r}, 1, t0, t1, join)
		case edgeEllipse:
			ctr, major := c.point(10), c.point(11)
			ratio := c.float(40)
			t0, t1 := sweep(deg(c.float(50)), deg(c.float(51)), c.int(73) != 0)
			p.EllipseArc(ctr, major, ratio, t0, t1, join)
		case edgeSpline:
			splineEdge(p, c, join)
		}
	}
	if len(p.Segs) > start {
		p.Close()
	}
}

// sweep orders an edge's angles in the direction it runs. A clockwise edge
// stores its angles mirrored, measured the other way round.
//
// The sweep is reduced with math.Mod rather than by stepping 2π at a time:
// at an angle like 1e300, adding 2π changes nothing and the loop would never
// end. Equal angles make a full turn; a non-finite one draws nothing.
func sweep(a0, a1 float64, ccw bool) (float64, float64) {
	if !ccw {
		a0, a1 = -a0, -a1
	}
	d := a1 - a0
	if !ccw {
		d = -d
	}
	d = math.Mod(d, 2*math.Pi)
	if math.IsNaN(d) || math.IsInf(a0, 0) {
		return a0, a0
	}
	if d <= 0 {
		d += 2 * math.Pi
	}
	if !ccw {
		return a0, a0 - d
	}
	return a0, a0 + d
}

func splineEdge(p *vector.Path, c *cursor, join bool) {
	s := &cad.Spline{Degree: c.int(94)}
	rational := c.int(73) != 0
	c.int(74) // periodic
	knots := c.count(95, 1)
	ctrl := c.count(96, 2)
	for range knots {
		s.Knots = append(s.Knots, c.float(40))
	}
	for range ctrl {
		s.Control = append(s.Control, c.point(10))
		if rational {
			s.Weights = append(s.Weights, c.float(42))
		}
	}
	// Fit data (97, then 11/21 pairs) was added to spline edges in R2010.
	// Before that the next 97 is the loop's count of source objects, which
	// hatchBoundary reads, so take it here only when fit points follow it
	// (a zero is the same either way).
	if c.i < len(c.g) && c.g[c.i].code == 97 &&
		(c.g[c.i].int() == 0 || (c.i+1 < len(c.g) && c.g[c.i+1].code == 11)) {
		for range c.count(97, 2) {
			s.Fit = append(s.Fit, c.point(11))
		}
	}
	// Start and end tangents, when present.
	c.want(12)
	c.want(22)
	c.want(13)
	c.want(23)
	pts := s.Points()
	for i, q := range pts {
		if i == 0 && !join {
			p.MoveTo(q)
		} else {
			p.LineTo(q)
		}
	}
}
