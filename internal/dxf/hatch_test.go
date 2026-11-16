package dxf

import (
	"bytes"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

func hatch(t *testing.T, pairs ...any) *cad.Hatch {
	t.Helper()
	return one[*cad.Hatch](t, dxftest.E("HATCH", pairs...))
}

// hatchHead is what precedes the boundary in a real file: the elevation
// point and pattern name, whose 10/20 must not be taken for a vertex.
func hatchHead(solid int, loops ...any) []any {
	return append([]any{10, 0.0, 20, 0.0, 2, "SOLID", 70, solid, 71, 0}, loops...)
}

// hatchTail follows the boundary and must be ignored.
var hatchTail = []any{75, 0, 76, 1, 98, 1, 10, 99.0, 20, 99.0}

func ops(p vector.Path) []vector.Op {
	out := make([]vector.Op, 0, len(p.Segs))
	for _, s := range p.Segs {
		out = append(out, s.Op)
	}
	return out
}

func count(p vector.Path, op vector.Op) int {
	n := 0
	for _, s := range p.Segs {
		if s.Op == op {
			n++
		}
	}
	return n
}

// end is where a segment finishes.
func end(s vector.Seg) cad.Point {
	if s.Op == vector.CubicTo {
		return s.P[2]
	}
	return s.P[0]
}

const (
	mv = vector.MoveTo
	ln = vector.LineTo
	cb = vector.CubicTo
	cl = vector.Close
)

func TestHatchSolidFlag(t *testing.T) {
	if h := hatch(t, hatchHead(1)...); !h.Solid || len(h.Boundary.Segs) != 0 {
		t.Errorf("solid hatch %+v", h)
	}
	if h := hatch(t, hatchHead(0)...); h.Solid {
		t.Errorf("pattern hatch %+v", h)
	}
}

func TestHatchPolylineLoop(t *testing.T) {
	pairs := hatchHead(1, 91, 1,
		92, 2, 72, 1, 73, 1, 93, 3,
		10, 0.0, 20, 0.0, 42, 0.5,
		10, 10.0, 20, 0.0, 42, 0.0,
		10, 10.0, 20, 10.0, 42, -1.0,
		97, 0)
	h := hatch(t, append(pairs, hatchTail...)...)
	var want vector.Path
	cad.AddPolyline(&want, []cad.Vertex{
		{P: cad.Point{}, Bulge: 0.5},
		{P: cad.Point{X: 10}},
		{P: cad.Point{X: 10, Y: 10}, Bulge: -1},
	}, true)
	if !reflect.DeepEqual(h.Boundary.Segs, want.Segs) {
		t.Errorf("boundary %v\nwant %v", h.Boundary.Segs, want.Segs)
	}
}

func TestHatchPolylineLoopWithoutBulges(t *testing.T) {
	// Flags 7 (external, polyline, derived): the polyline bit decides.
	h := hatch(t, hatchHead(1, 91, 1,
		92, 7, 72, 0, 73, 1, 93, 3,
		10, 0.0, 20, 0.0, 10, 4.0, 20, 0.0, 10, 4.0, 20, 3.0,
		97, 0)...)
	var want vector.Path
	want.MoveTo(cad.Point{})
	want.LineTo(cad.Point{X: 4})
	want.LineTo(cad.Point{X: 4, Y: 3})
	want.LineTo(cad.Point{})
	want.Close()
	if !reflect.DeepEqual(h.Boundary.Segs, want.Segs) {
		t.Errorf("boundary %v\nwant %v", h.Boundary.Segs, want.Segs)
	}
}

func TestHatchLineEdges(t *testing.T) {
	h := hatch(t, hatchHead(1, 91, 1,
		92, 1, 93, 3,
		72, 1, 10, 0.0, 20, 0.0, 11, 4.0, 21, 0.0,
		72, 1, 10, 4.0, 20, 0.0, 11, 4.0, 21, 3.0,
		72, 1, 10, 4.0, 20, 3.0, 11, 0.0, 21, 0.0,
		97, 0)...)
	var want vector.Path
	want.MoveTo(cad.Point{})
	want.LineTo(cad.Point{X: 4})
	want.LineTo(cad.Point{X: 4})
	want.LineTo(cad.Point{X: 4, Y: 3})
	want.LineTo(cad.Point{X: 4, Y: 3})
	want.LineTo(cad.Point{})
	want.Close()
	if !reflect.DeepEqual(h.Boundary.Segs, want.Segs) {
		t.Errorf("boundary %v\nwant %v", h.Boundary.Segs, want.Segs)
	}
}

func TestHatchArcEdges(t *testing.T) {
	arc := func(start, stop float64, ccw int) vector.Path {
		t.Helper()
		return hatch(t, hatchHead(1, 91, 1, 92, 0, 93, 1,
			72, 2, 10, 1.0, 20, 1.0, 40, 2.0, 50, start, 51, stop, 73, ccw, 97, 0)...).Boundary
	}
	cases := []struct {
		name        string
		start, stop float64
		ccw         int
		from, to    cad.Point
		cubics      int
	}{
		{"counter-clockwise quarter", 0, 90, 1, cad.Point{X: 3, Y: 1}, cad.Point{X: 1, Y: 3}, 1},
		// A clockwise edge stores its angles measured the other way round, so
		// 0 to 90 runs clockwise from east to south.
		{"clockwise quarter", 0, 90, 0, cad.Point{X: 3, Y: 1}, cad.Point{X: 1, Y: -1}, 1},
		{
			"counter-clockwise wrapping",
			270,
			90,
			1,
			cad.Point{X: 1, Y: -1},
			cad.Point{X: 1, Y: 3},
			2,
		},
		{"full circle", 0, 0, 1, cad.Point{X: 3, Y: 1}, cad.Point{X: 3, Y: 1}, 4},
		{"clockwise full circle", 0, 0, 0, cad.Point{X: 3, Y: 1}, cad.Point{X: 3, Y: 1}, 4},
	}
	for _, c := range cases {
		p := arc(c.start, c.stop, c.ccw)
		n := len(p.Segs)
		if n < 3 || p.Segs[0].Op != mv || p.Segs[n-1].Op != cl {
			t.Errorf("%s: ops %v", c.name, ops(p))
			continue
		}
		if !nearPt(end(p.Segs[0]), c.from) || !nearPt(end(p.Segs[n-2]), c.to) ||
			count(p, cb) != c.cubics {
			t.Errorf("%s: from %v to %v in %d cubics; want %v to %v in %d",
				c.name, end(p.Segs[0]), end(p.Segs[n-2]), count(p, cb), c.from, c.to, c.cubics)
		}
	}
}

func TestHatchEllipseEdgeJoinsLine(t *testing.T) {
	h := hatch(t, hatchHead(1, 91, 1, 92, 0, 93, 2,
		72, 1, 10, -2.0, 20, 0.0, 11, 2.0, 21, 0.0,
		72, 3, 10, 0.0, 20, 0.0, 11, 2.0, 21, 0.0, 40, 0.5, 50, 0.0, 51, 180.0, 73, 1,
		97, 0)...)
	p := h.Boundary
	// A following edge continues the subpath: one MoveTo, a LineTo to the
	// ellipse's start, then the half ellipse through (0, 1).
	want := []vector.Op{mv, ln, ln, cb, cb, cl}
	if !reflect.DeepEqual(ops(p), want) {
		t.Fatalf("ops %v, want %v", ops(p), want)
	}
	if !nearPt(end(p.Segs[2]), cad.Point{X: 2}) || !nearPt(end(p.Segs[3]), cad.Point{Y: 1}) ||
		!nearPt(end(p.Segs[4]), cad.Point{X: -2}) {
		t.Errorf("ellipse runs %v %v %v", end(p.Segs[2]), end(p.Segs[3]), end(p.Segs[4]))
	}
}

func TestHatchSplineEdge(t *testing.T) {
	h := hatch(t, hatchHead(1, 91, 1, 92, 0, 93, 2,
		72, 4, 94, 1, 73, 1, 74, 0, 95, 4, 96, 2,
		40, 0.0, 40, 0.0, 40, 1.0, 40, 1.0,
		10, 0.0, 20, 0.0, 42, 1.0,
		10, 10.0, 20, 0.0, 42, 3.0,
		97, 0,
		12, 1.0, 22, 0.0, 13, 1.0, 23, 0.0,
		// The tangents are consumed, so this line edge is read in step.
		72, 1, 10, 10.0, 20, 0.0, 11, 0.0, 21, 0.0,
		97, 0)...)
	p := h.Boundary
	// Degree 1 over one span: eight samples after the start.
	want := []vector.Op{mv, ln, ln, ln, ln, ln, ln, ln, ln, ln, ln, cl}
	if !reflect.DeepEqual(ops(p), want) {
		t.Fatalf("ops %v, want %v", ops(p), want)
	}
	if !nearPt(end(p.Segs[0]), cad.Point{}) || !nearPt(end(p.Segs[8]), cad.Point{X: 10}) ||
		!nearPt(end(p.Segs[10]), cad.Point{}) {
		t.Errorf("segments %v", p.Segs)
	}
	// Halfway along, weights 1 and 3 pull the curve to 7.5, not 5: the
	// weights were read.
	if !nearPt(end(p.Segs[4]), cad.Point{X: 7.5}) {
		t.Errorf("midpoint %v, want (7.5, 0)", end(p.Segs[4]))
	}
}

func TestHatchFitSplineEdge(t *testing.T) {
	// A spline edge given only by fit points, after a line: it joins on.
	h := hatch(t, hatchHead(1, 91, 1, 92, 0, 93, 2,
		72, 1, 10, 0.0, 20, 0.0, 11, 1.0, 21, 1.0,
		72, 4, 94, 3, 73, 0, 74, 0, 95, 0, 96, 0,
		97, 2, 11, 1.0, 21, 1.0, 11, 4.0, 21, 5.0,
		97, 0)...)
	var want vector.Path
	want.MoveTo(cad.Point{})
	want.LineTo(cad.Point{X: 1, Y: 1})
	want.LineTo(cad.Point{X: 1, Y: 1})
	want.LineTo(cad.Point{X: 4, Y: 5})
	want.Close()
	if !reflect.DeepEqual(h.Boundary.Segs, want.Segs) {
		t.Errorf("boundary %v\nwant %v", h.Boundary.Segs, want.Segs)
	}
}

func TestHatchFitSplineEdgeFirst(t *testing.T) {
	h := hatch(t, hatchHead(1, 91, 1, 92, 0, 93, 1,
		72, 4, 94, 3, 73, 0, 74, 0, 95, 0, 96, 0, 97, 2, 11, 1.0, 21, 1.0, 11, 4.0, 21, 5.0,
		97, 0)...)
	if want := []vector.Op{mv, ln, cl}; !reflect.DeepEqual(ops(h.Boundary), want) {
		t.Errorf("ops %v, want %v", ops(h.Boundary), want)
	}
}

func TestHatchLoopsAndSourceHandles(t *testing.T) {
	h := hatch(t, hatchHead(0, 91, 3,
		92, 2, 72, 0, 73, 1, 93, 2, 10, 0.0, 20, 0.0, 10, 1.0, 20, 0.0,
		97, 2, 330, "1A", 330, "1B",
		// An edge loop with no edges, and one whose only edge is of a type
		// we don't know: neither leaves anything behind.
		92, 1, 93, 0, 97, 0,
		92, 1, 93, 1, 72, 9, 10, 0.0, 20, 0.0,
		97, 1, 330, "1C")...)
	// The handles were skipped, so the second and third loops were read in
	// step; only the first drew anything.
	if want := []vector.Op{mv, ln, ln, cl}; !reflect.DeepEqual(ops(h.Boundary), want) {
		t.Errorf("ops %v, want %v", ops(h.Boundary), want)
	}
}

func TestHatchTwoLoops(t *testing.T) {
	sq := []any{
		92,
		2,
		72,
		0,
		73,
		1,
		93,
		3,
		10,
		0.0,
		20,
		0.0,
		10,
		1.0,
		20,
		0.0,
		10,
		1.0,
		20,
		1.0,
		97,
		1,
		330,
		"2F",
	}
	h := hatch(t, hatchHead(1, append(append([]any{91, 2}, sq...), sq...)...)...)
	if count(h.Boundary, mv) != 2 || count(h.Boundary, cl) != 2 {
		t.Errorf("ops %v, want two closed subpaths", ops(h.Boundary))
	}
}

func TestHatchCorruptCounts(t *testing.T) {
	cases := map[string][]any{
		// A loop count far beyond what the file holds is capped by the
		// pairs left, so this can't allocate or spin for ever.
		"huge loop count": {91, 2000000000, 92, 2, 93, 2000000000, 10, 1.0, 20, 1.0},
		"negative counts": {91, -4, 92, 2, 93, -1},
		"huge edge count": {91, 1, 92, 1, 93, 1000000, 72, 1},
		"huge knot count": {91, 1, 92, 1, 93, 1, 72, 4, 95, 1000000, 96, 1000000, 97, 1000000},
		"boundary at end": {91, 1},
	}
	for name, pairs := range cases {
		start := time.Now()
		h := hatch(t, hatchHead(1, pairs...)...)
		if time.Since(start) > time.Second {
			t.Errorf("%s: took %v", name, time.Since(start))
		}
		if len(h.Boundary.Segs) > 20 {
			t.Errorf("%s: %d segments from a handful of pairs", name, len(h.Boundary.Segs))
		}
	}
}

func TestSweep(t *testing.T) {
	cases := []struct {
		a0, a1 float64
		ccw    bool
		w0, w1 float64
	}{
		{0, math.Pi / 2, true, 0, math.Pi / 2},
		{math.Pi, 0, true, math.Pi, 2 * math.Pi},
		{1, 1, true, 1, 1 + 2*math.Pi},
		{0, math.Pi / 2, false, 0, -math.Pi / 2},
		{math.Pi / 2, 0, false, -math.Pi / 2, -2 * math.Pi},
		{1, 1, false, -1, -1 - 2*math.Pi},
	}
	for _, c := range cases {
		g0, g1 := sweep(c.a0, c.a1, c.ccw)
		if !near(g0, c.w0) || !near(g1, c.w1) {
			t.Errorf(
				"sweep(%v, %v, %v) = %v, %v; want %v, %v",
				c.a0,
				c.a1,
				c.ccw,
				g0,
				g1,
				c.w0,
				c.w1,
			)
		}
	}
}

// TestHatchSplineEdgeWithoutFitData shows a pre-2010 spline edge, which has
// no fit-data count, swallowing the loop's source-object count (also code
// 97); the 330 handles are then left unread and every later loop is lost.
func TestHatchSplineEdgeWithoutFitData(t *testing.T) {
	sq := []any{
		92,
		2,
		72,
		0,
		73,
		1,
		93,
		3,
		10,
		0.0,
		20,
		0.0,
		10,
		1.0,
		20,
		0.0,
		10,
		1.0,
		20,
		1.0,
		97,
		0,
	}
	pairs := append([]any{
		91, 2,
		92, 1, 93, 1, 72, 4, 94, 1, 73, 0, 74, 0, 95, 4, 96, 2,
		40, 0.0, 40, 0.0, 40, 1.0, 40, 1.0, 10, 5.0, 20, 5.0, 10, 6.0, 20, 5.0,
		97, 1, 330, "3C",
	}, sq...)
	h := hatch(t, hatchHead(1, pairs...)...)
	if count(h.Boundary, cl) != 2 {
		t.Errorf("ops %v, want two closed subpaths", ops(h.Boundary))
	}
}

// TestHatchHugeArcAngle shows sweep looping for ever when an arc edge's
// angle is so large that adding 2π no longer changes it.
func TestHatchHugeArcAngle(t *testing.T) {
	data := dxftest.New(4).Add(dxftest.E("HATCH", hatchHead(1, 91, 1, 92, 0, 93, 1,
		72, 2, 10, 0.0, 20, 0.0, 40, 1.0, 50, 1e300, 51, 0.0, 73, 1)...)).Bytes()
	done := make(chan error, 1)
	go func() {
		_, err := Read(bytes.NewReader(data))
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Read: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Read did not return")
	}
}
