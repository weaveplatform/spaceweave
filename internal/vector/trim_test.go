package vector

import (
	"testing"
)

// square is a unit-ish box with its lower-left corner at (x, y).
func square(x, y, size float64) *Path {
	p := &Path{Stroke: true}
	p.Polyline(
		[]Point{{X: x, Y: y}, {X: x + size, Y: y}, {X: x + size, Y: y + size}, {X: x, Y: y + size}},
		true,
	)
	return p
}

// grid is n boxes of size 1 laid out in rows of ten, 2 units apart.
func grid(n int) []Item {
	items := make([]Item, 0, n+1)
	for i := range n {
		items = append(items, square(float64(i%10)*2, float64(i/10)*2, 1))
	}
	return items
}

func TestExtentFewItemsNotTrimmed(t *testing.T) {
	items := append(grid(minTrim-2), square(1e6, 1e6, 1))
	b, n := Extent(items)
	if n != 0 {
		t.Errorf("trimmed %d of %d items", n, len(items))
	}
	if b.Max.X < 1e6 {
		t.Errorf("stray left out of a small drawing: %v..%v", b.Min, b.Max)
	}
}

func TestExtentTrimsStray(t *testing.T) {
	items := append(grid(40), square(5e5, -5e5, 1))
	b, n := Extent(items)
	if n != 1 {
		t.Fatalf("trimmed %d, want 1", n)
	}
	// The cluster runs from (0,0) to (19,7).
	if b.Min != (Point{X: 0, Y: 0}) || b.Max != (Point{X: 19, Y: 7}) {
		t.Errorf("extent %v..%v, want the cluster (0,0)..(19,7)", b.Min, b.Max)
	}
}

func TestExtentKeepsEdgesOfBuilding(t *testing.T) {
	// A wall a building's width away from the rest is architecture, not a
	// stray: the window is widened by its own span.
	items := append(grid(40), square(35, 3, 1), square(-15, 3, 1))
	b, n := Extent(items)
	if n != 0 {
		t.Errorf("trimmed %d nearby items", n)
	}
	if b.Min.X != -15 || b.Max.X != 36 {
		t.Errorf("extent x %v..%v, want -15..36", b.Min.X, b.Max.X)
	}
}

func TestExtentLongRowKeepsNeighbours(t *testing.T) {
	// Thirty boxes in one row: every centre has the same y, so without the
	// floor the y window would be zero wide and drop a box one size off it.
	items := make([]Item, 0, 31)
	for i := range 30 {
		items = append(items, square(float64(i)*3, 0, 1))
	}
	items = append(items, square(10, 6, 1))
	b, n := Extent(items)
	if n != 0 {
		t.Errorf("trimmed %d from a row", n)
	}
	if b.Max.Y != 7 {
		t.Errorf("extent top %v, want 7", b.Max.Y)
	}
}

func TestExtentAllAtOnePoint(t *testing.T) {
	items := make([]Item, 0, 30)
	for range 30 {
		p := &Path{}
		p.MoveTo(Point{X: 5, Y: 5})
		items = append(items, p)
	}
	b, n := Extent(items)
	if n != 0 {
		t.Errorf("trimmed %d coincident items", n)
	}
	if b.Empty() || b.Min != (Point{X: 5, Y: 5}) || b.Max != (Point{X: 5, Y: 5}) {
		t.Errorf("extent %+v, want the point (5,5)", b)
	}
}

func TestExtentIgnoresEmptyItems(t *testing.T) {
	// Twenty empty paths must not count towards minTrim, or a stray among
	// a handful of real items would be trimmed on the strength of nothing.
	items := make([]Item, 0, 40)
	for range 30 {
		items = append(items, &Path{})
	}
	items = append(items, square(0, 0, 1), square(2, 0, 1), square(1e6, 0, 1))
	b, n := Extent(items)
	if n != 0 || b.Max.X != 1e6+1 {
		t.Errorf("extent %v..%v trimmed %d", b.Min, b.Max, n)
	}
	if b, n := Extent(nil); !b.Empty() || n != 0 {
		t.Errorf("Extent(nil) = %+v, %d", b, n)
	}
}

func TestWindow(t *testing.T) {
	lo, hi := window([]float64{5, 5, 5, 5}, 0)
	if !(lo < 5 && hi > 5) {
		t.Errorf("window around one value collapsed: %v..%v", lo, hi)
	}
	// 21 values, unsorted: the 2nd and 98th percentiles round inwards to
	// the second smallest (1) and second largest (19), and the window is
	// that range widened by itself each side.
	v := make([]float64, 21)
	for i := range v {
		v[i] = float64(20 - i)
	}
	lo, hi = window(v, 0)
	if lo != -17 || hi != 37 {
		t.Errorf("window = %v..%v, want -17..37", lo, hi)
	}
	lo, hi = window([]float64{0, 0, 0}, 10)
	if lo != -10 || hi != 10 {
		t.Errorf("floored window = %v..%v, want -10..10", lo, hi)
	}
}

func TestExtentTrimsStrayFromMinTrimItems(t *testing.T) {
	// minTrim says 20 items are enough to look for strays among, but the 98th
	// percentile of n values only falls short of the maximum once
	// round(0.98*(n-1)) < n-1, i.e. n ≥ 27. From 20 to 26 items the stray is
	// itself the window's upper edge and can never be trimmed.
	items := append(grid(minTrim), square(1e6, 1e6, 1))
	if _, n := Extent(items); n != 1 {
		t.Errorf("trimmed %d of %d items, want the stray", n, len(items))
	}
}
