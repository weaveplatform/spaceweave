package dwg

import (
	"math"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// Hatch boundary loop flags and edge types.
const (
	loopPolyline = 2
	edgeLine     = 1
	edgeArc      = 2
	edgeEllipse  = 3
	edgeSpline   = 4
)

// hatch reads a HATCH's boundary loops into one path, a closed subpath
// each. The path is built the way the DXF reader builds it (dxf/hatch.go),
// so a drawing reads the same from either format; the two must change
// together.
func (d *decoder) hatch(o *obj, r *rec) *cad.Hatch {
	if o.ver >= v2004 {
		// Gradient fill: drawn as a plain fill, so only skipped.
		o.d.BL()
		o.d.BL()
		o.d.BD()
		o.d.BD()
		o.d.BL()
		o.d.BD()
		for range o.d.count(o.d.BL(), 4) {
			o.d.BD()
			readCMC(o.d, o.s, o.ver)
		}
		o.tv()
	}
	o.d.BD() // elevation
	h := &cad.Hatch{Common: r.common(o.d.BD3())}
	o.tv() // pattern name
	h.Solid = o.d.B()
	o.d.B() // associative
	for range o.d.count(o.d.BL(), 4) {
		if o.d.BL()&loopPolyline != 0 {
			polylineLoop(&h.Boundary, o.d)
		} else {
			edgeLoop(&h.Boundary, o.d, o.ver)
		}
		// The handles of the objects the loop was picked from.
		o.refs(o.d.BL())
	}
	return h
}

func polylineLoop(p *vector.Path, r *bitReader) {
	bulges := r.B()
	r.B() // closed: a boundary always is
	n := r.count(r.BL(), 128)
	vs := make([]cad.Vertex, 0, n)
	for range n {
		v := cad.Vertex{P: point2(r.RD2())}
		if bulges {
			v.Bulge = r.BD()
		}
		vs = append(vs, v)
	}
	cad.AddPolyline(p, vs, true)
}

func edgeLoop(p *vector.Path, r *bitReader, v ver) {
	n := r.count(r.BL(), 8)
	start := len(p.Segs)
	for range n {
		if r.err != nil {
			break
		}
		join := len(p.Segs) > start
		switch r.RC() {
		case edgeLine:
			a, b := point2(r.RD2()), point2(r.RD2())
			if join {
				p.LineTo(a)
			} else {
				p.MoveTo(a)
			}
			p.LineTo(b)
		case edgeArc:
			c, rad := point2(r.RD2()), r.BD()
			a0, a1 := r.BD(), r.BD()
			t0, t1 := sweep(a0, a1, r.B())
			p.EllipseArc(c, cad.Point{X: rad}, 1, t0, t1, join)
		case edgeEllipse:
			c, major := point2(r.RD2()), point2(r.RD2())
			ratio, a0, a1 := r.BD(), r.BD(), r.BD()
			t0, t1 := sweep(a0, a1, r.B())
			p.EllipseArc(c, major, ratio, t0, t1, join)
		case edgeSpline:
			splineEdge(p, r, v, join)
		default:
			r.fail(errShort)
		}
	}
	if len(p.Segs) > start {
		p.Close()
	}
}

// sweep orders an edge's angles in the direction it runs; a clockwise edge
// stores its angles mirrored. A copy of the DXF reader's: see there for
// why it reduces with math.Mod.
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

func splineEdge(p *vector.Path, r *bitReader, v ver, join bool) {
	s := &cad.Spline{Degree: int(r.BL())}
	rational := r.B()
	r.B() // periodic
	knots := r.count(r.BL(), 2)
	ctrl := r.count(r.BL(), 128)
	for range knots {
		s.Knots = append(s.Knots, r.BD())
	}
	for range ctrl {
		s.Control = append(s.Control, point2(r.RD2()))
		if rational {
			s.Weights = append(s.Weights, r.BD())
		}
	}
	if v >= v2010 {
		if n := r.count(r.BL(), 128); n > 0 {
			for range n {
				s.Fit = append(s.Fit, point2(r.RD2()))
			}
			r.RD2()
			r.RD2() // start and end tangents
		}
	}
	for i, q := range s.Points() {
		if i == 0 && !join {
			p.MoveTo(q)
		} else {
			p.LineTo(q)
		}
	}
}
