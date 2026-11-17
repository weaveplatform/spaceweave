package floorgraph

import (
	"container/heap"
	"math"
	"slices"
)

// quantum is the working grid: every derived coordinate is rounded to 1/16
// px. Go may fuse x*y+z into one FMA instruction on arm64 but not on amd64,
// so the same expression can differ in its last bit between machines;
// rounding to a coarse grid stops that difference reaching a comparison or
// the output. Products below are wrapped in float64(...) for the same
// reason: the conversion forces rounding, which forbids the fusion.
const quantum = 16

func q(v float64) float64 { return math.Round(v*quantum) / quantum }

func qp(p Point) Point { return Point{X: q(p.X), Y: q(p.Y)} }

func sub(a, b Point) Point { return Point{X: a.X - b.X, Y: a.Y - b.Y} }

func addScaled(p, u Point, s float64) Point {
	return Point{X: p.X + float64(u.X*s), Y: p.Y + float64(u.Y*s)}
}

func dot(a, b Point) float64 { return float64(a.X*b.X) + float64(a.Y*b.Y) }

func cross(a, b Point) float64 { return float64(a.X*b.Y) - float64(a.Y*b.X) }

func norm(a Point) float64 { return math.Sqrt(dot(a, a)) }

func dist(a, b Point) float64 { return norm(sub(a, b)) }

func mid(a, b Point) Point { return Point{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2} }

func lerp(a, b Point, t float64) Point { return addScaled(a, sub(b, a), t) }

func unit(a Point) Point {
	l := norm(a)
	if l == 0 {
		return Point{}
	}
	return Point{X: a.X / l, Y: a.Y / l}
}

// canonical turns a direction to point right, or straight down, so the two
// orientations of one line compare equal.
func canonical(u Point) Point {
	if u.X < 0 || (u.X == 0 && u.Y < 0) {
		return Point{X: -u.X, Y: -u.Y}
	}
	return u
}

func finite(vs ...float64) bool {
	for _, v := range vs {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// segDist is the distance from p to the segment ab.
func segDist(p, a, b Point) float64 {
	ab := sub(b, a)
	l2 := dot(ab, ab)
	if l2 == 0 {
		return dist(p, a)
	}
	return dist(p, lerp(a, b, clamp(dot(sub(p, a), ab)/l2, 0, 1)))
}

// signedArea is the shoelace sum. Its sign is algebraic: with image y down a
// positive area is clockwise on screen.
func signedArea(poly []Point) float64 {
	s := 0.0
	for i := range poly {
		s += cross(poly[i], poly[(i+1)%len(poly)])
	}
	return s / 2
}

// centroid is the area centroid, or the vertex mean of a degenerate polygon.
func centroid(poly []Point) Point {
	a := signedArea(poly)
	if a == 0 || len(poly) < 3 {
		var c Point
		for _, p := range poly {
			c.X += p.X
			c.Y += p.Y
		}
		n := float64(max(len(poly), 1))
		return Point{X: c.X / n, Y: c.Y / n}
	}
	// Relative to the first vertex: absolute coordinates in the thousands
	// would square into the cancellation this avoids.
	o := poly[0]
	var cx, cy float64
	for i := range poly {
		p, r := sub(poly[i], o), sub(poly[(i+1)%len(poly)], o)
		f := cross(p, r)
		cx += float64((p.X + r.X) * f)
		cy += float64((p.Y + r.Y) * f)
	}
	return Point{X: o.X + cx/(6*a), Y: o.Y + cy/(6*a)}
}

// inside is the even-odd ray test of web/floorplan-iso/js/geometry.js's
// pointInPolygon, so a point it accepts the site's validator accepts.
func inside(p Point, poly []Point) bool {
	in := false
	for i, j := 0, len(poly)-1; i < len(poly); j, i = i, i+1 {
		pi, pj := poly[i], poly[j]
		if (pi.Y > p.Y) != (pj.Y > p.Y) &&
			p.X < float64((pj.X-pi.X)*(p.Y-pi.Y))/(pj.Y-pi.Y)+pi.X {
			in = !in
		}
	}
	return in
}

// boundaryDist is the distance from p to the polygon's edges, negative
// outside.
func boundaryDist(p Point, poly []Point) float64 {
	d := math.Inf(1)
	for i := range poly {
		d = math.Min(d, segDist(p, poly[i], poly[(i+1)%len(poly)]))
	}
	if !inside(p, poly) {
		return -d
	}
	return d
}

// cell is a square of the pole search: its centre, half size, distance to
// the polygon, and the best distance any point in it could have.
type cell struct {
	c       Point
	h, d, m float64
}

type cells []cell

func (c cells) Len() int { return len(c) }

func (c cells) Less(i, j int) bool {
	if c[i].m != c[j].m {
		return c[i].m > c[j].m
	}
	if c[i].c.Y != c[j].c.Y {
		return c[i].c.Y < c[j].c.Y
	}
	return c[i].c.X < c[j].c.X
}

func (c cells) Swap(i, j int) { c[i], c[j] = c[j], c[i] }

func (c *cells) Push(x any) { *c = append(*c, x.(cell)) }

func (c *cells) Pop() any {
	old := *c
	x := old[len(old)-1]
	*c = old[:len(old)-1]
	return x
}

// maxPoleCells bounds the pole search, so a pathological polygon costs
// milliseconds rather than looping on float noise.
const maxPoleCells = 4096

// pole is the polygon's pole of inaccessibility (the inside point farthest
// from its edges, "polylabel"), found to within precision px: where a room
// name reads best when no label was found.
func pole(poly []Point, precision float64) Point {
	lo, hi := poly[0], poly[0]
	for _, p := range poly {
		lo = Point{X: math.Min(lo.X, p.X), Y: math.Min(lo.Y, p.Y)}
		hi = Point{X: math.Max(hi.X, p.X), Y: math.Max(hi.Y, p.Y)}
	}
	size := math.Min(hi.X-lo.X, hi.Y-lo.Y)
	if size <= 0 {
		return poly[0]
	}
	mk := func(c Point, h float64) cell {
		d := boundaryDist(c, poly)
		return cell{c: c, h: h, d: d, m: d + float64(h*math.Sqrt2)}
	}
	h := size / 2
	var queue cells
	for x := lo.X; x < hi.X && queue.Len() < maxPoleCells; x += size {
		for y := lo.Y; y < hi.Y; y += size {
			heap.Push(&queue, mk(Point{X: x + h, Y: y + h}, h))
		}
	}
	best := mk(centroid(poly), 0)
	for n := 0; queue.Len() > 0 && n < maxPoleCells; n++ {
		c := heap.Pop(&queue).(cell)
		if c.d > best.d {
			best = c
		}
		if c.m-best.d <= precision {
			continue
		}
		h := c.h / 2
		for _, o := range [4]Point{{X: -h, Y: -h}, {X: h, Y: -h}, {X: -h, Y: h}, {X: h, Y: h}} {
			heap.Push(&queue, mk(Point{X: c.c.X + o.X, Y: c.c.Y + o.Y}, h))
		}
	}
	return best.c
}

// hull is the convex hull (monotone chain), in order, without collinear
// points. Fewer than three distinct points come back as they are.
func hull(pts []Point) []Point {
	ps := slices.Clone(pts)
	slices.SortFunc(ps, cmpPoint)
	ps = slices.Compact(ps)
	if len(ps) < 3 {
		return ps
	}
	out := make([]Point, 0, 2*len(ps))
	for pass := range 2 {
		start := len(out)
		for _, p := range ps {
			for len(out) >= start+2 && cross(sub(out[len(out)-1], out[len(out)-2]), sub(p, out[len(out)-2])) <= 0 {
				out = out[:len(out)-1]
			}
			out = append(out, p)
		}
		out = out[:len(out)-1]
		if pass == 0 {
			slices.Reverse(ps)
		}
	}
	return out
}

func cmpPoint(a, b Point) int {
	if a.Y != b.Y {
		if a.Y < b.Y {
			return -1
		}
		return 1
	}
	if a.X != b.X {
		if a.X < b.X {
			return -1
		}
		return 1
	}
	return 0
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// simplify drops repeated vertices and vertices within tol px of the line
// through their neighbours where the path carries straight on. A vertex where
// the path doubles back (a slit to an inner wall) stays.
func simplify(poly []Point, tol float64) []Point {
	out := slices.Clone(poly)
	for changed := true; changed && len(out) >= 3; {
		changed = false
		for i := 0; i < len(out) && len(out) >= 3; i++ {
			p, c, n := out[(i+len(out)-1)%len(out)], out[i], out[(i+1)%len(out)]
			d1, d2 := sub(c, p), sub(n, c)
			l := norm(sub(n, p))
			if c == p ||
				(l > 0 && dot(d1, d2) > 0 && math.Abs(cross(sub(c, p), sub(n, p)))/l <= tol) {
				out = slices.Delete(out, i, i+1)
				changed = true
				i--
			}
		}
	}
	return out
}

// unionFind joins indices; find returns the smallest index of a set, so a
// group's representative doesn't depend on the order unions were made in.
type unionFind []int

func newUnionFind(n int) unionFind {
	u := make(unionFind, n)
	for i := range u {
		u[i] = i
	}
	return u
}

func (u unionFind) find(i int) int {
	for u[i] != i {
		u[i] = u[u[i]]
		i = u[i]
	}
	return i
}

func (u unionFind) union(i, j int) {
	a, b := u.find(i), u.find(j)
	if a > b {
		a, b = b, a
	}
	u[b] = a
}
