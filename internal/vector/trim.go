package vector

import (
	"math"
	"slices"
)

// minTrim is the fewest items worth looking for strays among.
const minTrim = 20

// Extent is the drawing's extent without strays. CAD files often carry a
// stray point or line kilometres from the building (an old xref, a slip of
// the mouse), and fitting the image to it would shrink the plan to a dot.
// Items whose centres lie well outside the middle 96% of the others are left
// out; the count of them is returned.
func Extent(items []Item) (Rect, int) {
	boxes := make([]Rect, 0, len(items))
	for _, it := range items {
		if b := it.Bounds(); !b.Empty() {
			boxes = append(boxes, b)
		}
	}
	var all Rect
	for _, b := range boxes {
		all = all.Union(b)
	}
	if len(boxes) < minTrim {
		return all, 0
	}
	xs := make([]float64, len(boxes))
	ys := make([]float64, len(boxes))
	sizes := make([]float64, len(boxes))
	for i, b := range boxes {
		c, sz := b.Center(), b.Size()
		xs[i], ys[i], sizes[i] = c.X, c.Y, math.Max(sz.X, sz.Y)
	}
	slices.Sort(sizes)
	floor := 10 * sizes[len(sizes)/2]
	xlo, xhi := window(xs, floor)
	ylo, yhi := window(ys, floor)
	var kept Rect
	dropped := 0
	for _, b := range boxes {
		c := b.Center()
		if c.X < xlo || c.X > xhi || c.Y < ylo || c.Y > yhi {
			dropped++
			continue
		}
		kept = kept.Union(b)
	}
	return kept, dropped
}

// window is the range items may lie in: the 2nd to 98th percentile, widened
// by its own span on each side, so the edge of a real building is never cut.
// The span is at least floor, so a drawing that is mostly one long row
// doesn't lose the items just off it.
func window(v []float64, floor float64) (lo, hi float64) {
	slices.Sort(v)
	// Round the indices inwards: with 20 items, rounding to nearest would
	// make the 98th percentile the largest value, so a lone stray would set
	// the window it should fall outside of.
	n := float64(len(v) - 1)
	lo, hi = v[int(math.Ceil(0.02*n))], v[int(math.Floor(0.98*n))]
	span := math.Max(math.Max(hi-lo, floor), 1e-9)
	return lo - span, hi + span
}
