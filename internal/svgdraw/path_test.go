package svgdraw

import (
	"math"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/vector"
)

const eps = 1e-6

func near(a, b float64) bool { return math.Abs(a-b) < eps }

func nearPt(a, b vector.Point) bool { return near(a.X, b.X) && near(a.Y, b.Y) }

func pt(x, y float64) vector.Point { return vector.Point{X: x, Y: y} }

// seg is a compact expectation: the op and the points that op uses.
type seg struct {
	op  vector.Op
	pts []vector.Point
}

func mv(x, y float64) seg { return seg{op: vector.MoveTo, pts: []vector.Point{pt(x, y)}} }
func ln(x, y float64) seg { return seg{op: vector.LineTo, pts: []vector.Point{pt(x, y)}} }
func cb(x1, y1, x2, y2, x, y float64) seg {
	return seg{op: vector.CubicTo, pts: []vector.Point{pt(x1, y1), pt(x2, y2), pt(x, y)}}
}

var cl = seg{op: vector.Close}

func checkSegs(t *testing.T, got []vector.Seg, want []seg) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d segments %v, want %d", len(got), got, len(want))
	}
	for i, w := range want {
		if got[i].Op != w.op {
			t.Fatalf("segment %d: op %d, want %d (%v)", i, got[i].Op, w.op, got)
		}
		for j, p := range w.pts {
			if !nearPt(got[i].P[j], p) {
				t.Fatalf("segment %d point %d: %v, want %v", i, j, got[i].P[j], p)
			}
		}
	}
}

func TestParsePathCommands(t *testing.T) {
	const third, twoThirds = 10.0 / 3, 20.0 / 3
	cases := []struct {
		name, d string
		want    []seg
	}{
		{"moveto", "M10 20", []seg{mv(10, 20)}},
		{"relative first moveto is absolute", "m10 20", []seg{mv(10, 20)}},
		{"implicit lineto after M", "M10 20 30 40", []seg{mv(10, 20), ln(30, 40)}},
		{
			"implicit lineto after m is relative",
			"m10 20 5 5 1 1",
			[]seg{mv(10, 20), ln(15, 25), ln(16, 26)},
		},
		{"lineto", "M0 0 L10 0 l5 5", []seg{mv(0, 0), ln(10, 0), ln(15, 5)}},
		{"repeated lineto", "M0 0 L1 1 2 2", []seg{mv(0, 0), ln(1, 1), ln(2, 2)}},
		{"horizontal", "M1 2 H10 h5", []seg{mv(1, 2), ln(10, 2), ln(15, 2)}},
		{"repeated horizontal", "M0 0 H1 2 3", []seg{mv(0, 0), ln(1, 0), ln(2, 0), ln(3, 0)}},
		{"vertical", "M1 2 V10 v5", []seg{mv(1, 2), ln(1, 10), ln(1, 15)}},
		{
			"cubic",
			"M0 0 C1 1 2 2 3 3 c1 1 2 2 3 3",
			[]seg{mv(0, 0), cb(1, 1, 2, 2, 3, 3), cb(4, 4, 5, 5, 6, 6)},
		},
		{
			"smooth cubic reflects the previous control point",
			"M0 0 C0 10 10 10 10 0 S20 -10 20 0",
			[]seg{mv(0, 0), cb(0, 10, 10, 10, 10, 0), cb(10, -10, 20, -10, 20, 0)},
		},
		{
			"smooth cubic chains its own reflection",
			"M0 0 C0 10 10 10 10 0 s10 -10 10 0 s10 10 10 0",
			[]seg{
				mv(0, 0),
				cb(0, 10, 10, 10, 10, 0),
				cb(10, -10, 20, -10, 20, 0),
				cb(20, 10, 30, 10, 30, 0),
			},
		},
		// Without a cubic before it, S's first control point is the current point.
		{"smooth cubic alone", "M0 0 S10 10 20 0", []seg{mv(0, 0), cb(0, 0, 10, 10, 20, 0)}},
		{
			"smooth cubic after quadratic",
			"M0 0 Q5 5 10 0 S15 5 20 0",
			[]seg{mv(0, 0), cb(third, third, 10-third, third, 10, 0), cb(10, 0, 15, 5, 20, 0)},
		},
		{
			"quadratic",
			"M0 0 Q10 10 20 0",
			[]seg{mv(0, 0), cb(twoThirds, twoThirds, 40.0/3, twoThirds, 20, 0)},
		},
		{
			"relative quadratic",
			"M10 0 q10 10 20 0",
			[]seg{mv(10, 0), cb(10+twoThirds, twoThirds, 10+40.0/3, twoThirds, 30, 0)},
		},
		{
			"smooth quadratic reflects",
			"M0 0 Q10 10 20 0 T40 0",
			[]seg{
				mv(0, 0),
				cb(twoThirds, twoThirds, 40.0/3, twoThirds, 20, 0),
				cb(20+twoThirds, -twoThirds, 40-twoThirds, -twoThirds, 40, 0),
			},
		},
		{
			"smooth quadratic chains",
			"M0 0 Q5 5 10 0 t10 0 t10 0",
			[]seg{
				mv(0, 0), cb(third, third, 10-third, third, 10, 0),
				cb(10+third, -third, 20-third, -third, 20, 0),
				cb(20+third, third, 30-third, third, 30, 0),
			},
		},
		{"smooth quadratic alone is straight", "M0 0 T9 0", []seg{mv(0, 0), cb(0, 0, 3, 0, 9, 0)}},
		{
			"smooth quadratic after cubic",
			"M0 0 C1 1 2 2 3 3 T6 3",
			[]seg{mv(0, 0), cb(1, 1, 2, 2, 3, 3), cb(3, 3, 4, 3, 6, 3)},
		},
		{
			"close returns to the subpath start",
			"M0 0 L10 0 L10 10 Z m1 1 l1 0",
			[]seg{mv(0, 0), ln(10, 0), ln(10, 10), cl, mv(1, 1), ln(2, 1)},
		},
		{"close then relative line", "M5 5 L10 5 z l1 1", []seg{mv(5, 5), ln(10, 5), cl, ln(6, 6)}},
		{"compact numbers", "M1.5.5L-1-2", []seg{mv(1.5, 0.5), ln(-1, -2)}},
		{"leading dots", "M.5.5l.25.25", []seg{mv(0.5, 0.5), ln(0.75, 0.75)}},
		{"exponents", "M1e-3,2E+2 L1e2-1e1", []seg{mv(0.001, 200), ln(100, -10)}},
		{"explicit plus and separators", "M+1,\t+2\nL\r3,4", []seg{mv(1, 2), ln(3, 4)}},
		{"empty", "", nil},
		{"no command", "10 10", nil},
		{"missing coordinate keeps what was read", "M0 0 L10", []seg{mv(0, 0)}},
		{"unknown command stops", "M0 0 L10 10 X 5 5", []seg{mv(0, 0), ln(10, 10)}},
		{"bad exponent stops", "M0 0 L1 1 2e", []seg{mv(0, 0), ln(1, 1)}},
		{"infinite number stops", "M0 0 L1e999 0", []seg{mv(0, 0)}},
		{"lone sign stops", "M0 0 L- 5", []seg{mv(0, 0)}},
		{"bad moveto", "M0", nil},
		{"bad cubic", "M0 0 C1 1 2", []seg{mv(0, 0)}},
		{"bad smooth cubic", "M0 0 S1 1 2", []seg{mv(0, 0)}},
		{"bad quadratic", "M0 0 Q1 1 2", []seg{mv(0, 0)}},
		{"bad smooth quadratic", "M0 0 T1", []seg{mv(0, 0)}},
		{"bad horizontal", "M0 0 H", []seg{mv(0, 0)}},
		{"bad vertical", "M0 0 V.", []seg{mv(0, 0)}},
		{"numbers after close are ignored", "M0 0 L1 0 Z 5 5", []seg{mv(0, 0), ln(1, 0), cl}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := &vector.Path{}
			parsePath(c.d, p)
			checkSegs(t, p.Segs, c.want)
		})
	}
}

func bezier(s vector.Seg, from vector.Point, t float64) vector.Point {
	u := 1 - t
	return from.Mul(u * u * u).
		Add(s.P[0].Mul(3 * u * u * t)).
		Add(s.P[1].Mul(3 * u * t * t)).
		Add(s.P[2].Mul(t * t * t))
}

// arcMid is the point halfway along an arc's parameter range. EllipseArc
// splits an arc into equal steps, so the middle is a joint between cubics
// when there is an even number of them, else the middle of the middle one.
func arcMid(t *testing.T, segs []vector.Seg) vector.Point {
	t.Helper()
	first := -1
	for i, s := range segs {
		if s.Op == vector.CubicTo {
			first = i
			break
		}
	}
	if first < 1 {
		t.Fatalf("no arc in %v", segs)
	}
	cubics := segs[first:]
	n := len(cubics)
	if n%2 == 0 {
		return cubics[n/2-1].P[2]
	}
	from := segs[first-1].P[0]
	if k := first + n/2; k > first {
		from = segs[k-1].P[2]
	}
	return bezier(segs[first+n/2], from, 0.5)
}

func TestArcEndpointsAndFlags(t *testing.T) {
	// A chord of 10 on a radius-10 circle: the centre sits 5√3 off the chord,
	// so each flag pair picks a distinct arc with a distinct midpoint.
	h := 5 * math.Sqrt(3)
	cases := []struct {
		name string
		d    string
		mid  vector.Point
	}{
		{"small sweep", "M0 0 A10 10 0 0 1 10 0", pt(5, h-10)},
		{"small no sweep", "M0 0 A10 10 0 0 0 10 0", pt(5, 10-h)},
		{"large sweep", "M0 0 A10 10 0 1 1 10 0", pt(5, -h-10)},
		{"large no sweep", "M0 0 A10 10 0 1 0 10 0", pt(5, h+10)},
		// A semicircle has the same arc for both values of the large flag.
		{"semicircle sweep", "M0 0 A5 5 0 0 1 10 0", pt(5, -5)},
		{"semicircle sweep large", "M0 0 A5 5 0 1 1 10 0", pt(5, -5)},
		{"semicircle no sweep", "M0 0 A5 5 0 0 0 10 0", pt(5, 5)},
		{"semicircle no sweep large", "M0 0 A5 5 0 1 0 10 0", pt(5, 5)},
		// Radii too small to span the chord grow until they just do.
		{"radii scaled up", "M0 0 A1 1 0 0 1 10 0", pt(5, -5)},
		{"negative radii", "M0 0 A-5 -5 0 0 1 10 0", pt(5, -5)},
		{"relative", "M10 10 a5 5 0 0 1 10 0", pt(15, 5)},
		{"compact flags", "M0 0a5 5 0 0110 0", pt(5, -5)},
		// The major axis turned to vertical: half an ellipse from end to end of it.
		{"rotated ellipse", "M0 0 A10 5 90 0 1 0 20", pt(5, 10)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := &vector.Path{}
			parsePath(c.d, p)
			last := p.Segs[len(p.Segs)-1]
			start := p.Segs[0].P[0]
			if last.Op != vector.CubicTo {
				t.Fatalf("last segment %v is not a curve", last)
			}
			wantEnd := pt(10, 0)
			switch c.name {
			case "relative":
				wantEnd = pt(20, 10)
			case "rotated ellipse":
				wantEnd = pt(0, 20)
			}
			// The end is set exactly, not left to accumulated rounding.
			if last.P[2] != wantEnd {
				t.Fatalf("end %v, want exactly %v", last.P[2], wantEnd)
			}
			// The arc continues from the current point.
			if p.Segs[1].Op != vector.LineTo || !nearPt(p.Segs[1].P[0], start) {
				t.Fatalf("arc does not start at %v: %v", start, p.Segs[1])
			}
			if got := arcMid(t, p.Segs); !nearPt(got, c.mid) {
				t.Fatalf("midpoint %v, want %v", got, c.mid)
			}
		})
	}
}

func TestArcDegenerate(t *testing.T) {
	cases := []struct {
		name, d string
		want    []seg
	}{
		{"zero rx is a line", "M0 0 A0 5 0 0 1 10 0", []seg{mv(0, 0), ln(10, 0)}},
		{"zero ry is a line", "M0 0 A5 0 0 0 1 10 0", []seg{mv(0, 0), ln(10, 0)}},
		{"same end point", "M3 4 A5 5 0 0 1 3 4", []seg{mv(3, 4), ln(3, 4)}},
		{"bad flag", "M0 0 A5 5 0 2 1 10 0", []seg{mv(0, 0)}},
		{"missing sweep", "M0 0 A5 5 0 0", []seg{mv(0, 0)}},
		{"missing radii", "M0 0 A5", []seg{mv(0, 0)}},
		{"missing end", "M0 0 A5 5 0 0 1 10", []seg{mv(0, 0)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := &vector.Path{}
			parsePath(c.d, p)
			checkSegs(t, p.Segs, c.want)
		})
	}
}

func TestArcRepeats(t *testing.T) {
	// Two arcs in one command: the second starts where the first ended.
	p := &vector.Path{}
	parsePath("M0 0 A5 5 0 0 1 10 0 5 5 0 0 1 20 0", p)
	if got := p.Segs[len(p.Segs)-1].P[2]; got != pt(20, 0) {
		t.Fatalf("end %v, want (20,0)", got)
	}
	if got := arcMid(t, p.Segs[len(p.Segs)-3:]); !nearPt(got, pt(15, -5)) {
		t.Fatalf("second arc midpoint %v, want (15,-5)", got)
	}
}
