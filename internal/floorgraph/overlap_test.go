package floorgraph

import (
	"math"
	"slices"
	"testing"
)

// overlapArea is the area two polygons share, by the even-odd rule: the Go
// copy of overlapArea in web/floorplan-iso/js/geometry.js, for siteCheck.
// Both are cut into slabs at every vertex height. No edge starts or ends
// inside a slab, so each polygon is a row of trapezoids there, and the width
// two trapezoids share is linear between the heights where any two of their
// sides cross: summing it exactly there keeps two spaces that only share a
// wall line at an overlap of 0, which sampling can't promise.
func overlapArea(a, b [][2]float64) float64 {
	ax0, ay0, ax1, ay1 := bounds(a)
	bx0, by0, bx1, by1 := bounds(b)
	lo, hi := math.Max(ay0, by0), math.Min(ay1, by1)
	if !(lo < hi && math.Max(ax0, bx0) < math.Min(ax1, bx1)) {
		return 0
	}
	cuts := []float64{lo, hi}
	for _, p := range slices.Concat(a, b) {
		if p[1] > lo && p[1] < hi {
			cuts = append(cuts, p[1])
		}
	}
	slices.Sort(cuts)
	cuts = slices.Compact(cuts)
	area := 0.0
	for s := 0; s+1 < len(cuts); s++ {
		y0, y1 := cuts[s], cuts[s+1]
		ta := trapezoids(a, y0, y1)
		if len(ta) == 0 {
			continue
		}
		tb := trapezoids(b, y0, y1)
		for _, u := range ta {
			for _, v := range tb {
				area += (y1 - y0) * sharedWidth(u, v)
			}
		}
	}
	return area
}

func bounds(p [][2]float64) (x0, y0, x1, y1 float64) {
	x0, y0, x1, y1 = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, q := range p {
		x0, x1 = math.Min(x0, q[0]), math.Max(x1, q[0])
		y0, y1 = math.Min(y0, q[1]), math.Max(y1, q[1])
	}
	return x0, y0, x1, y1
}

// trapezoids are a polygon's parts in the slab y0–y1, each its left and
// right x at y0 then at y1.
func trapezoids(p [][2]float64, y0, y1 float64) [][4]float64 {
	ym := (y0 + y1) / 2
	var xs [][3]float64
	for i, j := 0, len(p)-1; i < len(p); j, i = i, i+1 {
		a, b := p[j], p[i]
		if (a[1] > ym) != (b[1] > ym) {
			at := func(y float64) float64 { return a[0] + (y-a[1])*(b[0]-a[0])/(b[1]-a[1]) }
			xs = append(xs, [3]float64{at(ym), at(y0), at(y1)})
		}
	}
	slices.SortFunc(xs, func(u, v [3]float64) int { return cmpFloat(u[0], v[0]) })
	var out [][4]float64
	for k := 0; k+1 < len(xs); k += 2 {
		out = append(out, [4]float64{xs[k][1], xs[k+1][1], xs[k][2], xs[k+1][2]})
	}
	return out
}

// sharedWidth integrates over t in [0, 1] the width two trapezoids share.
func sharedWidth(u, v [4]float64) float64 {
	edges := [4][2]float64{{u[0], u[2]}, {u[1], u[3]}, {v[0], v[2]}, {v[1], v[3]}}
	ts := []float64{0, 1}
	for i := range edges {
		for j := i + 1; j < len(edges); j++ {
			d0, d1 := edges[i][0]-edges[j][0], edges[i][1]-edges[j][1]
			if d0 != d1 {
				if t := d0 / (d0 - d1); t > 0 && t < 1 {
					ts = append(ts, t)
				}
			}
		}
	}
	slices.Sort(ts)
	w := func(t float64) float64 {
		l := math.Max(u[0]+(u[2]-u[0])*t, v[0]+(v[2]-v[0])*t)
		r := math.Min(u[1]+(u[3]-u[1])*t, v[1]+(v[3]-v[1])*t)
		return math.Max(0, r-l)
	}
	s := 0.0
	for k := 0; k+1 < len(ts); k++ {
		s += (w(ts[k]) + w(ts[k+1])) / 2 * (ts[k+1] - ts[k])
	}
	return s
}

// The same cases as test/geometry.test.js's, so the two copies agree.
func TestOverlapArea(t *testing.T) {
	sq := func(x0, y0, x1, y1 float64) [][2]float64 {
		return [][2]float64{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
	}
	backward := func(p [][2]float64) [][2]float64 {
		p = slices.Clone(p)
		slices.Reverse(p)
		return p
	}
	diamond := [][2]float64{{2, 0}, {4, 2}, {2, 4}, {0, 2}}
	ell := [][2]float64{{0, 0}, {4, 0}, {4, 1}, {1, 1}, {1, 4}, {0, 4}}
	cases := []struct {
		name string
		a, b [][2]float64
		want float64
	}{
		{"overlapping squares", sq(0, 0, 4, 4), sq(2, 1, 6, 3), 4},
		{"opposite winding", sq(0, 0, 4, 4), backward(sq(2, 1, 6, 3)), 4},
		{"one inside the other", sq(0, 0, 4, 4), sq(1, 1, 2, 2), 1},
		{"apart", sq(0, 0, 4, 4), sq(5, 0, 6, 4), 0},
		{"diamond", diamond, sq(0, 0, 4, 4), 8},
		{"square in a diamond", diamond, sq(1, 1, 3, 3), 4},
		{"two triangles", [][2]float64{{0, 0}, {2, 0}, {0, 2}}, [][2]float64{{0, 0}, {2, 0}, {2, 2}}, 1},
		{"concave neighbour", ell, sq(1, 1, 4, 4), 0},
		{"concave inside", ell, sq(0, 0, 4, 4), 7},
		{"shared side", sq(0, 0, 2, 2), sq(2, 0, 4, 2), 0},
		{"shared top", sq(0, 0, 2, 2), sq(0, 2, 2, 4), 0},
		{"shared corner", sq(0, 0, 2, 2), sq(2, 2, 4, 4), 0},
		{"part of a side", sq(0, 0, 2, 2), sq(1, 2, 3, 4), 0},
		{"shared diagonal", [][2]float64{{0, 0}, {3, 0}, {0, 3}}, [][2]float64{{3, 0}, {3, 3}, {0, 3}}, 0},
		{"a line has no area", [][2]float64{{0, 0}, {1, 0}}, sq(0, 0, 2, 2), 0},
	}
	for _, c := range cases {
		if got := overlapArea(c.a, c.b); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}
