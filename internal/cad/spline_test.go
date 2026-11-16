package cad

import (
	"math"
	"testing"
)

// onSegment reports whether p lies on the segment a-b.
func onSegment(p, a, b Point) bool {
	ab, ap := b.Sub(a), p.Sub(a)
	cross := ab.X*ap.Y - ab.Y*ap.X
	dot := ab.X*ap.X + ab.Y*ap.Y
	return math.Abs(cross) < eps && dot >= -eps && dot <= ab.X*ab.X+ab.Y*ab.Y+eps
}

func TestSplineDegreeOneIsControlPolygon(t *testing.T) {
	ctrl := []Point{{X: 0, Y: 0}, {X: 4, Y: 0}, {X: 4, Y: 3}}
	s := &Spline{Degree: 1, Control: ctrl}
	pts := s.Points()
	if len(pts) < len(ctrl) {
		t.Fatalf("got %d points, want at least %d", len(pts), len(ctrl))
	}
	for _, p := range pts {
		if !onSegment(p, ctrl[0], ctrl[1]) && !onSegment(p, ctrl[1], ctrl[2]) {
			t.Errorf("point %v is off the control polygon", p)
		}
	}
	if !nearPt(pts[0], ctrl[0]) || !nearPt(pts[len(pts)-1], ctrl[2]) {
		t.Errorf("ends %v..%v, want %v..%v", pts[0], pts[len(pts)-1], ctrl[0], ctrl[2])
	}
	// A linear spline passes through its corner, so a sample must land on it
	// or the polygon's corner would be cut.
	corner := false
	for _, p := range pts {
		corner = corner || nearPt(p, ctrl[1])
	}
	if !corner {
		t.Error("no sample on the middle control point")
	}
}

func TestSplineClampedCubicHitsEnds(t *testing.T) {
	ctrl := []Point{{X: 0, Y: 0}, {X: 1, Y: 3}, {X: 3, Y: -2}, {X: 5, Y: 4}, {X: 7, Y: 0}}
	for _, deg := range []int{3, 0, 9} {
		// Degrees 0 and 9 are invalid for five points and fall back to cubic.
		pts := (&Spline{Degree: deg, Control: ctrl}).Points()
		if !nearPt(pts[0], ctrl[0]) || !nearPt(pts[len(pts)-1], ctrl[4]) {
			t.Errorf(
				"degree %d: ends %v..%v, want %v..%v",
				deg,
				pts[0],
				pts[len(pts)-1],
				ctrl[0],
				ctrl[4],
			)
		}
		// Clamping must not make the curve a polygon: the interior samples
		// lie strictly inside the hull's x range and off the control points.
		for _, p := range pts[1 : len(pts)-1] {
			if p.X <= 0 || p.X >= 7 {
				t.Errorf("degree %d: interior point %v outside the hull", deg, p)
			}
		}
	}
}

func TestSplineExplicitKnots(t *testing.T) {
	ctrl := []Point{{X: 0, Y: 0}, {X: 1, Y: 2}, {X: 2, Y: 0}}
	// A quadratic with these knots is the Bézier curve, whose midpoint is
	// a quarter of the way to its control point: (1, 1).
	pts := (&Spline{Degree: 2, Control: ctrl, Knots: []float64{0, 0, 0, 5, 5, 5}}).Points()
	mid := pts[len(pts)/2]
	if !nearPt(mid, Point{X: 1, Y: 1}) {
		t.Errorf("midpoint %v, want (1, 1)", mid)
	}
	// A degenerate knot vector has no span to sample; the control points
	// are the best picture of the curve left.
	flat := (&Spline{Degree: 2, Control: ctrl, Knots: []float64{0, 0, 0, 0, 0, 0}}).Points()
	if len(flat) != len(ctrl) || !nearPt(flat[1], ctrl[1]) {
		t.Errorf("degenerate knots gave %v, want the control points", flat)
	}
}

func TestSplineWeightsPull(t *testing.T) {
	ctrl := []Point{{X: 0, Y: 0}, {X: 1, Y: 1}, {X: 2, Y: 0}}
	mid := func(w []float64) Point {
		pts := (&Spline{Degree: 2, Control: ctrl, Weights: w}).Points()
		return pts[len(pts)/2]
	}
	plain := mid(nil)
	heavy := mid([]float64{1, 10, 1})
	if !near(plain.Y, 0.5) {
		t.Errorf("unweighted midpoint y = %v, want 0.5", plain.Y)
	}
	// For a rational quadratic Bézier the midpoint height is
	// w/(1+w) of the control point's, 10/11 here.
	if !near(heavy.Y, 10.0/11) || !near(heavy.X, 1) {
		t.Errorf("weighted midpoint %v, want (1, %v)", heavy, 10.0/11)
	}
	// Non-positive weights are file damage and count as 1, as does a
	// weight list that doesn't match the control points.
	for _, w := range [][]float64{{1, 0, 1}, {1, -3, 1}, {1, 10}} {
		if got := mid(w); !nearPt(got, plain) {
			t.Errorf("weights %v: midpoint %v, want unweighted %v", w, got, plain)
		}
	}
}

func TestSplineFitPointsOnly(t *testing.T) {
	fit := []Point{{X: 0, Y: 0}, {X: 1, Y: 1}, {X: 2, Y: 0}}
	got := (&Spline{Degree: 3, Fit: fit}).Points()
	if len(got) != len(fit) {
		t.Fatalf("got %v, want the fit points %v", got, fit)
	}
	for i := range fit {
		if got[i] != fit[i] {
			t.Errorf("point %d = %v, want %v", i, got[i], fit[i])
		}
	}
	// One control point is not a curve; the fit points win.
	one := (&Spline{Control: []Point{{X: 9, Y: 9}}, Fit: fit}).Points()
	if len(one) != len(fit) {
		t.Errorf("single control point gave %v, want fit points", one)
	}
}

func TestClampedKnots(t *testing.T) {
	got := clamped(5, 3)
	want := []float64{0, 0, 0, 0, 1, 2, 2, 2, 2}
	if len(got) != len(want) {
		t.Fatalf("clamped(5, 3) = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("clamped(5, 3) = %v, want %v", got, want)
		}
	}
}
