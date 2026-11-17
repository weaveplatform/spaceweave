package floorgraph

import (
	"math"
	"slices"
)

// mergeRadius is how close two graph vertices must be to be one, px: closer
// than the eye can tell apart at drawing resolution, and well above the
// 1/16 px working grid.
const mergeRadius = 0.5

// graph is the planar wall graph: segments split wherever they meet.
type graph struct {
	pts   []Point
	edges []edge
}

// edge is a stretch of wall between two vertices, carrying its source
// segment's attributes.
type edge struct {
	v     [2]int
	u     Point // the source segment's direction
	cls   int
	t, w  float64
	glass bool
	// virtual edges are bridges added by bridge, not walls.
	virtual bool
}

func (g *graph) length(e int) float64 {
	return dist(g.pts[g.edges[e].v[0]], g.pts[g.edges[e].v[1]])
}

type split struct {
	t float64
	p Point
}

// node splits the segments at every crossing and T-junction and merges
// vertices within mergeRadius, giving a planar graph.
func node(segs []seg) *graph {
	splits := make([][]split, len(segs))
	for i, s := range segs {
		splits[i] = []split{{0, s.a}, {1, s.b}}
	}
	// Sweep in x so only segments whose boxes overlap are compared.
	order := make([]int, len(segs))
	for i := range order {
		order[i] = i
	}
	minX := func(i int) float64 { return math.Min(segs[i].a.X, segs[i].b.X) }
	slices.SortStableFunc(order, func(i, j int) int { return cmpFloat(minX(i), minX(j)) })
	for k, i := range order {
		si := segs[i]
		maxX := math.Max(si.a.X, si.b.X) + mergeRadius
		yLo, yHi := math.Min(si.a.Y, si.b.Y)-mergeRadius, math.Max(si.a.Y, si.b.Y)+mergeRadius
		for _, j := range order[k+1:] {
			if minX(j) > maxX {
				break
			}
			sj := segs[j]
			if math.Max(sj.a.Y, sj.b.Y) < yLo || math.Min(sj.a.Y, sj.b.Y) > yHi {
				continue
			}
			meet(si, sj, func(t float64, p Point) { splits[i] = append(splits[i], split{t, p}) },
				func(t float64, p Point) { splits[j] = append(splits[j], split{t, p}) })
		}
	}

	g := &graph{}
	ids := vertexMerger{cells: map[[2]int64][]int{}}
	seen := map[[2]int]bool{}
	for i, s := range segs {
		sp := splits[i]
		slices.SortFunc(sp, func(x, y split) int {
			if c := cmpFloat(x.t, y.t); c != 0 {
				return c
			}
			return cmpPoint(x.p, y.p)
		})
		prev := -1
		for _, x := range sp {
			v := ids.id(g, x.p)
			if prev >= 0 && v != prev {
				key := [2]int{min(prev, v), max(prev, v)}
				if !seen[key] {
					seen[key] = true
					g.edges = append(
						g.edges,
						edge{
							v:     [2]int{prev, v},
							u:     s.u,
							cls:   s.cls,
							t:     s.t,
							w:     s.w,
							glass: s.glass,
						},
					)
				}
			}
			prev = v
		}
	}
	return g
}

// meet reports where segments x and y cross or touch, as a parameter along
// each. Ends within mergeRadius of the other segment count: that is a
// T-junction, or two pieces of one wall meeting end to end.
func meet(x, y seg, onX, onY func(float64, Point)) {
	r, s := sub(x.b, x.a), sub(y.b, y.a)
	lx, ly := norm(r), norm(s)
	if den := cross(r, s); math.Abs(den) > float64(1e-9*lx)*ly {
		w := sub(y.a, x.a)
		tx, ty := cross(w, s)/den, cross(w, r)/den
		ex, ey := mergeRadius/lx, mergeRadius/ly
		if tx >= -ex && tx <= 1+ex && ty >= -ey && ty <= 1+ey {
			p := qp(addScaled(x.a, r, tx))
			onX(clamp(tx, 0, 1), p)
			onY(clamp(ty, 0, 1), p)
		}
	}
	for _, p := range [2]Point{y.a, y.b} {
		if t, ok := touches(x, p); ok {
			onX(t, p)
		}
	}
	for _, p := range [2]Point{x.a, x.b} {
		if t, ok := touches(y, p); ok {
			onY(t, p)
		}
	}
}

// touches says p lies on s's interior, within mergeRadius.
func touches(s seg, p Point) (float64, bool) {
	r := sub(s.b, s.a)
	t := dot(sub(p, s.a), r) / dot(r, r)
	if t <= 0 || t >= 1 {
		return 0, false
	}
	return t, dist(p, lerp(s.a, s.b, t)) <= mergeRadius
}

// vertexMerger gives points within mergeRadius of an earlier point that
// point's vertex. Points arrive in segment order, which is canonical, so the
// representative chosen is too.
type vertexMerger struct {
	cells map[[2]int64][]int
}

func (m vertexMerger) id(g *graph, p Point) int {
	cx, cy := int64(math.Floor(p.X/mergeRadius)), int64(math.Floor(p.Y/mergeRadius))
	best, bestD := -1, mergeRadius
	for dx := int64(-1); dx <= 1; dx++ {
		for dy := int64(-1); dy <= 1; dy++ {
			for _, v := range m.cells[[2]int64{cx + dx, cy + dy}] {
				if d := dist(g.pts[v], p); d < bestD || (d == bestD && v < best) {
					best, bestD = v, d
				}
			}
		}
	}
	if best >= 0 {
		return best
	}
	g.pts = append(g.pts, p)
	v := len(g.pts) - 1
	m.cells[[2]int64{cx, cy}] = append(m.cells[[2]int64{cx, cy}], v)
	return v
}

// face is a cycle of the half-edge walk.
type face struct {
	verts []int
	half  []int // half-edge 2e runs v[0]→v[1] of edge e, 2e+1 back
	area  float64
}

func (g *graph) halfEnds(h int) (from, to int) {
	e := g.edges[h/2]
	if h%2 == 0 {
		return e.v[0], e.v[1]
	}
	return e.v[1], e.v[0]
}

// prune marks the edges left after repeatedly removing dangling ends: the
// part of the graph that can bound a face.
func (g *graph) prune() []bool {
	alive := make([]bool, len(g.edges))
	deg := make([]int, len(g.pts))
	inc := make([][]int, len(g.pts))
	for i, e := range g.edges {
		alive[i] = true
		for _, v := range e.v {
			deg[v]++
			inc[v] = append(inc[v], i)
		}
	}
	var stack []int
	for v, d := range deg {
		if d == 1 {
			stack = append(stack, v)
		}
	}
	for len(stack) > 0 {
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, e := range inc[v] {
			if !alive[e] {
				continue
			}
			alive[e] = false
			for _, w := range g.edges[e].v {
				deg[w]--
				if w != v && deg[w] == 1 {
					stack = append(stack, w)
				}
			}
		}
	}
	return alive
}

// halfPlane splits directions into [0°, 180°) and [180°, 360°) (y down, so
// clockwise on screen) for the exact angular order below.
func halfPlane(d Point) int {
	if d.Y > 0 || (d.Y == 0 && d.X > 0) {
		return 0
	}
	return 1
}

// faces walks the alive edges' half-edges. Leaving each vertex, half-edges are
// ordered by angle; a walk arriving along h leaves by the half-edge just
// before h's twin in that order. With that rule every bounded face comes out
// with positive shoelace area and each component's outer face negative,
// whichever way y points: the sign is algebraic (TestFaceSigns pins it).
func (g *graph) faces(alive []bool) []face {
	out := make([][]int, len(g.pts))
	for e := range g.edges {
		if alive[e] {
			out[g.edges[e].v[0]] = append(out[g.edges[e].v[0]], 2*e)
			out[g.edges[e].v[1]] = append(out[g.edges[e].v[1]], 2*e+1)
		}
	}
	pos := make([]int, 2*len(g.edges))
	for v, hs := range out {
		dir := func(h int) Point { _, to := g.halfEnds(h); return sub(g.pts[to], g.pts[v]) }
		slices.SortFunc(hs, func(a, b int) int {
			da, db := dir(a), dir(b)
			if c := halfPlane(da) - halfPlane(db); c != 0 {
				return c
			}
			if c := cross(da, db); c != 0 {
				return -cmpFloat(c, 0)
			}
			return a - b
		})
		for i, h := range hs {
			pos[h] = i
		}
	}
	next := func(h int) int {
		_, to := g.halfEnds(h)
		hs := out[to]
		return hs[(pos[h^1]+len(hs)-1)%len(hs)]
	}
	seen := make([]bool, 2*len(g.edges))
	var fs []face
	for h := range seen {
		if !alive[h/2] || seen[h] {
			continue
		}
		var f face
		for c := h; !seen[c]; c = next(c) {
			seen[c] = true
			from, _ := g.halfEnds(c)
			f.verts = append(f.verts, from)
			f.half = append(f.half, c)
		}
		f.area = signedArea(g.poly(f.verts))
		fs = append(fs, f)
	}
	return fs
}

func (g *graph) poly(vs []int) []Point {
	ps := make([]Point, len(vs))
	for i, v := range vs {
		ps[i] = g.pts[v]
	}
	return ps
}

// components labels each vertex with its connected component over the
// given edges.
func (g *graph) components(alive []bool) unionFind {
	uf := newUnionFind(len(g.pts))
	for e, ed := range g.edges {
		if alive[e] {
			uf.union(ed.v[0], ed.v[1])
		}
	}
	return uf
}

// bridge connects each free-standing group of walls that stands inside a
// face of another group (a building core in an open-plan floor) to the wall
// straight above its topmost vertex, with a virtual edge. The face walk then
// goes round the group as a hole in the face around it, by way of a zero-width
// slit, so the group's own faces can be rooms without overlapping that one.
// It returns the alive flags for the grown edge list and how many bridges it
// added.
func (g *graph) bridge(alive []bool, fs []face) ([]bool, int) {
	comp := g.components(alive)
	top := map[int]int{} // component → its topmost, then leftmost, vertex
	var roots []int
	for e, ed := range g.edges {
		if !alive[e] {
			continue
		}
		for _, v := range ed.v {
			r := comp.find(v)
			t, ok := top[r]
			if !ok {
				roots = append(roots, r)
			}
			if !ok || cmpPoint(g.pts[v], g.pts[t]) < 0 {
				top[r] = v
			}
		}
	}
	slices.Sort(roots)
	// Each edge's component as it was before any bridge: new vertices from
	// splitting are past the union-find's end.
	ecomp := make([]int, len(g.edges))
	for e, ed := range g.edges {
		ecomp[e] = comp.find(ed.v[0])
	}
	added := 0
	for _, r := range roots {
		v := top[r]
		p := g.pts[v]
		contained := slices.ContainsFunc(fs, func(f face) bool {
			return f.area > 0 && comp.find(f.verts[0]) != r && inside(p, g.poly(f.verts))
		})
		if !contained {
			continue
		}
		hit, y := -1, math.Inf(-1)
		for e, ed := range g.edges {
			if !alive[e] || ed.virtual || ecomp[e] == r {
				continue
			}
			a, b := g.pts[ed.v[0]], g.pts[ed.v[1]]
			if a.X == b.X || p.X < math.Min(a.X, b.X) || p.X > math.Max(a.X, b.X) {
				continue
			}
			if yh := a.Y + float64((p.X-a.X)*(b.Y-a.Y))/(b.X-a.X); yh < p.Y && yh > y {
				hit, y = e, yh
			}
		}
		if hit < 0 {
			continue
		}
		h := qp(Point{X: p.X, Y: y})
		ed := g.edges[hit]
		w := -1
		for _, u := range ed.v {
			if dist(g.pts[u], h) <= mergeRadius {
				w = u
			}
		}
		if w < 0 {
			g.pts = append(g.pts, h)
			w = len(g.pts) - 1
			tail := ed
			tail.v = [2]int{w, ed.v[1]}
			g.edges[hit].v[1] = w
			g.edges = append(g.edges, tail)
			alive = append(alive, true)
			ecomp = append(ecomp, ecomp[hit])
		}
		g.edges = append(g.edges, edge{v: [2]int{v, w}, cls: -1, virtual: true})
		alive = append(alive, true)
		ecomp = append(ecomp, r)
		added++
	}
	return alive, added
}
