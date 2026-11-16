package cad

import (
	"math"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/vector"
)

// cubicAt evaluates a cubic Bézier from p0 with control points and end in s.
func cubicAt(p0 Point, s vector.Seg, t float64) Point {
	u := 1 - t
	a, b, c, d := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
	return Point{
		X: a*p0.X + b*s.P[0].X + c*s.P[1].X + d*s.P[2].X,
		Y: a*p0.Y + b*s.P[0].Y + c*s.P[1].Y + d*s.P[2].Y,
	}
}

// samples flattens a path into points along it, so tests can ask where the
// drawn curve actually runs rather than where its control points sit.
func samples(p *vector.Path) []Point {
	var out []Point
	var cur Point
	for _, s := range p.Segs {
		switch s.Op {
		case vector.MoveTo, vector.LineTo:
			cur = s.P[0]
			out = append(out, cur)
		case vector.CubicTo:
			for i := 1; i <= 16; i++ {
				out = append(out, cubicAt(cur, s, float64(i)/16))
			}
			cur = s.P[2]
		case vector.Close:
		}
	}
	return out
}

func TestBulgeSemicircle(t *testing.T) {
	a, b := Point{X: 0, Y: 0}, Point{X: 2, Y: 0}
	tests := []struct {
		bulge float64
		// A positive bulge turns counter-clockwise from a to b, which for a
		// chord running along +X carries the arc below it: its centre is to
		// the chord's left, the arc to its right.
		far Point
	}{
		{bulge: 1, far: Point{X: 1, Y: -1}},
		{bulge: -1, far: Point{X: 1, Y: 1}},
	}
	for _, tt := range tests {
		p := &vector.Path{}
		p.MoveTo(a)
		bulgeTo(p, a, b, tt.bulge)
		pts := samples(p)
		centre := Point{X: 1, Y: 0}
		reached := false
		for _, q := range pts {
			if r := q.Sub(centre).Len(); math.Abs(r-1) > 1e-3 {
				t.Errorf("bulge %v: point %v is %v from the centre, want 1", tt.bulge, q, r)
			}
			if q.Y*tt.far.Y < -eps {
				t.Errorf("bulge %v: point %v is on the wrong side of the chord", tt.bulge, q)
			}
			reached = reached || nearPt(q, tt.far)
		}
		if !reached {
			t.Errorf("bulge %v: arc never reaches %v", tt.bulge, tt.far)
		}
		if last := p.Segs[len(p.Segs)-1]; last.Op != vector.CubicTo || last.P[2] != b {
			t.Errorf(
				"bulge %v: last segment %+v, want a curve ending exactly on %v",
				tt.bulge,
				last,
				b,
			)
		}
	}
}

func TestBulgeQuarter(t *testing.T) {
	// tan(22.5°) is the bulge of a 90° arc; from (1,0) to (0,1) turning
	// counter-clockwise that is the unit circle's first quadrant.
	a, b := Point{X: 1, Y: 0}, Point{X: 0, Y: 1}
	p := &vector.Path{}
	p.MoveTo(a)
	bulgeTo(p, a, b, math.Tan(math.Pi/8))
	for _, q := range samples(p) {
		if r := q.Len(); math.Abs(r-1) > 1e-3 {
			t.Errorf("point %v is %v from the origin, want 1", q, r)
		}
		if q.X < -eps || q.Y < -eps {
			t.Errorf("point %v left the first quadrant", q)
		}
	}
}

func TestBulgeStraight(t *testing.T) {
	a, b := Point{X: 0, Y: 0}, Point{X: 3, Y: 4}
	for _, bulge := range []float64{0, 1e-12} {
		p := &vector.Path{}
		p.MoveTo(a)
		bulgeTo(p, a, b, bulge)
		if len(p.Segs) != 2 || p.Segs[1].Op != vector.LineTo || p.Segs[1].P[0] != b {
			t.Errorf("bulge %v: segs %+v, want one straight line to %v", bulge, p.Segs, b)
		}
	}
	// A bulge on a zero-length chord has no circle; it must not divide by
	// zero into NaN points.
	p := &vector.Path{}
	p.MoveTo(a)
	bulgeTo(p, a, a, 1)
	if len(p.Segs) != 2 || p.Segs[1].Op != vector.LineTo || p.Segs[1].P[0] != a {
		t.Errorf("zero chord: segs %+v, want a line to the same point", p.Segs)
	}
}

func TestAddPolyline(t *testing.T) {
	vs := []Vertex{{P: Point{X: 0, Y: 0}}, {P: Point{X: 4, Y: 0}}, {P: Point{X: 4, Y: 3}}}

	open := &vector.Path{}
	AddPolyline(open, vs, false)
	wantOps := []vector.Op{vector.MoveTo, vector.LineTo, vector.LineTo}
	checkOps(t, "open", open, wantOps)

	closed := &vector.Path{}
	AddPolyline(closed, vs, true)
	checkOps(
		t,
		"closed",
		closed,
		[]vector.Op{vector.MoveTo, vector.LineTo, vector.LineTo, vector.LineTo, vector.Close},
	)
	// The closing segment is drawn explicitly so a bulge on the last vertex
	// has somewhere to go.
	if got := closed.Segs[3].P[0]; got != vs[0].P {
		t.Errorf("closing segment ends at %v, want %v", got, vs[0].P)
	}

	// A bulge on the last vertex of a closed polyline curves the closing
	// segment; open, it is ignored.
	bulged := []Vertex{{P: Point{X: 0, Y: 0}}, {P: Point{X: 2, Y: 0}, Bulge: 1}}
	cp := &vector.Path{}
	AddPolyline(cp, bulged, true)
	hasCurve := false
	for _, s := range cp.Segs {
		hasCurve = hasCurve || s.Op == vector.CubicTo
	}
	if !hasCurve {
		t.Error("closing bulge drew no arc")
	}
	op := &vector.Path{}
	AddPolyline(op, bulged, false)
	checkOps(t, "open with trailing bulge", op, []vector.Op{vector.MoveTo, vector.LineTo})

	empty := &vector.Path{}
	AddPolyline(empty, nil, true)
	if len(empty.Segs) != 0 {
		t.Errorf("no vertices drew %+v", empty.Segs)
	}
}

func checkOps(t *testing.T, name string, p *vector.Path, want []vector.Op) {
	t.Helper()
	if len(p.Segs) != len(want) {
		t.Fatalf("%s: %d segments %+v, want ops %v", name, len(p.Segs), p.Segs, want)
	}
	for i, s := range p.Segs {
		if s.Op != want[i] {
			t.Errorf("%s: segment %d op %v, want %v", name, i, s.Op, want[i])
		}
	}
}
