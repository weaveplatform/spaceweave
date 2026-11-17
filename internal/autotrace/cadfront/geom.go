package cadfront

import (
	"math"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

type point = vector.Point

func dot(a, b point) float64   { return a.X*b.X + a.Y*b.Y }
func cross(a, b point) float64 { return a.X*b.Y - a.Y*b.X }

func unit(v point) point {
	if l := v.Len(); l > 0 {
		return v.Mul(1 / l)
	}
	return point{}
}

// inverse undoes m; ok is false when m squashes the plane flat.
func inverse(m vector.Affine) (vector.Affine, bool) {
	det := m.A*m.D - m.B*m.C
	if math.Abs(det) < 1e-300 {
		return vector.Affine{}, false
	}
	a, b, c, d := m.D/det, -m.B/det, -m.C/det, m.A/det
	return vector.Affine{A: a, B: b, C: c, D: d, E: -(a*m.E + c*m.F), F: -(b*m.E + d*m.F)}, true
}

// arcPoint is the point at angle a on a circle.
func arcPoint(c point, r, a float64) point {
	s, co := math.Sincos(a)
	return point{X: c.X + r*co, Y: c.Y + r*s}
}

// segment is a straight piece of drawing in drawing coordinates, with the
// leaf it came from.
type segment struct {
	a, b point
	leaf int
}

// bulgeArc is the arc a polyline bulge draws from a to b, in the
// polyline's own coordinates: centre, radius and counter-clockwise start
// and sweep (negative for clockwise). The geometry is the same as
// cad.AddPolyline's.
func bulgeArc(a, b point, bulge float64) (c point, r, start, sweep float64) {
	chord := b.Sub(a)
	l := chord.Len()
	left := point{X: -chord.Y, Y: chord.X}.Mul(1 / l)
	c = a.Add(chord.Mul(0.5)).Add(left.Mul(l * (1 - bulge*bulge) / (4 * bulge)))
	return c, a.Sub(c).Len(), math.Atan2(a.Y-c.Y, a.X-c.X), 4 * math.Atan(bulge)
}

// straightBulge is how small a bulge must be to count as a straight segment.
const straightBulge = 1e-6

// polylineSegments are a polyline's straight segments; closed polylines
// include the closing edge.
func polylineSegments(p *cad.Polyline) [][2]point {
	n := len(p.Vertices)
	edges := n - 1
	if p.Closed && n > 2 {
		edges = n
	}
	out := make([][2]point, 0, max(edges, 0))
	for i := range edges {
		a, b := p.Vertices[i], p.Vertices[(i+1)%n]
		if math.Abs(a.Bulge) < straightBulge && a.P != b.P {
			out = append(out, [2]point{a.P, b.P})
		}
	}
	return out
}

// samples are points on an entity, in its own coordinates, enough to find
// its extent: ends and corners, and a few points round each curve.
func samples(e cad.Entity) []point {
	switch e := e.(type) {
	case *cad.Line:
		return []point{e.A, e.B}
	case *cad.Circle:
		return curve(func(t float64) point { return arcPoint(e.Center, e.Radius, t) }, 0, 2*math.Pi)
	case *cad.Arc:
		at := func(t float64) point { return arcPoint(e.Center, e.Radius, t) }
		return curve(at, e.Start, ccwEnd(e.Start, e.End))
	case *cad.Ellipse:
		minor := point{X: -e.Major.Y, Y: e.Major.X}.Mul(e.Ratio)
		return curve(func(t float64) point {
			s, c := math.Sincos(t)
			return e.Center.Add(e.Major.Mul(c)).Add(minor.Mul(s))
		}, e.Start, ccwEnd(e.Start, e.End))
	case *cad.Polyline:
		return polylineSamples(e)
	case *cad.Spline:
		return e.Points()
	case *cad.Solid:
		return e.Corners[:]
	case *cad.Hatch:
		return pathPoints(&e.Boundary)
	}
	return nil
}

func polylineSamples(p *cad.Polyline) []point {
	out := make([]point, 0, 2*len(p.Vertices))
	for i, v := range p.Vertices {
		out = append(out, v.P)
		if math.Abs(v.Bulge) < straightBulge || (i == len(p.Vertices)-1 && !p.Closed) {
			continue
		}
		next := p.Vertices[(i+1)%len(p.Vertices)].P
		if next == v.P {
			continue
		}
		c, r, start, sweep := bulgeArc(v.P, next, v.Bulge)
		at := func(t float64) point { return arcPoint(c, r, t) }
		out = append(out, curve(at, start, start+sweep)...)
	}
	return out
}

// ccwEnd is an arc's end angle unwound to lie after its start.
func ccwEnd(start, end float64) float64 {
	for end <= start {
		end += 2 * math.Pi
	}
	return end
}

// curveSamples is enough points on a curve for its extent to within a few
// per cent of its size, which is all extents are used for.
const curveSamples = 16

func curve(at func(float64) point, t0, t1 float64) []point {
	out := make([]point, curveSamples+1)
	for i := range out {
		out[i] = at(t0 + (t1-t0)*float64(i)/curveSamples)
	}
	return out
}

// pathPoints are a path's points, cubics by their ends and controls,
// whose hull holds the curve.
func pathPoints(p *vector.Path) []point {
	var out []point
	for _, s := range p.Segs {
		switch s.Op {
		case vector.MoveTo, vector.LineTo:
			out = append(out, s.P[0])
		case vector.CubicTo:
			out = append(out, s.P[0], s.P[1], s.P[2])
		}
	}
	return out
}

// rings flattens a path into closed rings for point-in-polygon tests.
func rings(p *vector.Path, m vector.Affine) [][]point {
	var out [][]point
	var cur []point
	pen := point{}
	end := func() {
		if len(cur) > 2 {
			out = append(out, cur)
		}
		cur = nil
	}
	for _, s := range p.Segs {
		switch s.Op {
		case vector.MoveTo:
			end()
			pen = s.P[0]
			cur = []point{m.Apply(pen)}
		case vector.LineTo:
			pen = s.P[0]
			cur = append(cur, m.Apply(pen))
		case vector.CubicTo:
			p0 := pen
			for i := 1; i <= 8; i++ {
				t := float64(i) / 8
				u := 1 - t
				q := p0.Mul(u * u * u).Add(s.P[0].Mul(3 * u * u * t)).
					Add(s.P[1].Mul(3 * u * t * t)).Add(s.P[2].Mul(t * t * t))
				cur = append(cur, m.Apply(q))
			}
			pen = s.P[2]
		case vector.Close:
			end()
		}
	}
	end()
	return out
}

// inside is the even-odd rule over every ring, so holes count as outside.
func inside(rs [][]point, p point) bool {
	in := false
	for _, r := range rs {
		j := len(r) - 1
		for i := range r {
			a, b := r[i], r[j]
			if (a.Y > p.Y) != (b.Y > p.Y) && p.X < (b.X-a.X)*(p.Y-a.Y)/(b.Y-a.Y)+a.X {
				in = !in
			}
			j = i
		}
	}
	return in
}

// imageAngle is the direction of drawing vector v in the image, in degrees
// clockwise from right (0 right, 90 down), as plan.Item's angles are.
func imageAngle(view vector.Affine, v point) float64 {
	a := math.Atan2(view.B*v.X+view.D*v.Y, view.A*v.X+view.C*v.Y) * 180 / math.Pi
	a = math.Round(math.Mod(a+360, 360)*100) / 100
	if a >= 360 {
		a = 0
	}
	return a
}
