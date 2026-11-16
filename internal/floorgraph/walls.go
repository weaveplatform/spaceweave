package floorgraph

import (
	"math"
	"slices"
)

// Wall kinds, as plan.Wall.Kind, in output order.
const (
	exterior = "exterior"
	interior = "interior"
	glass    = "glass"
)

func kindRank(k string) int { return slices.Index([]string{exterior, interior, glass}, k) }

// chain is an output wall: a maximal straight run of graph edges of one kind
// and similar thickness.
type chain struct {
	from, to Point
	kind     string
	t        float64 // px, length-weighted
	w        float64
	length   float64
	ops      []outOpening
}

func (c *chain) dir() Point { return unit(sub(c.to, c.from)) }

// sinChain bounds the bend (as a sine) between two edges continuing one
// straight wall, for walls not snapped to a dominant direction.
var sinChain = math.Sin(0.5 * math.Pi / 180)

// similarThickness: within 25%, or unknown on either side. Beyond that a
// change of thickness is a different wall, as the trace format wants.
func similarThickness(a, b float64) bool {
	if a <= 0 || b <= 0 {
		return true
	}
	return math.Max(a, b) <= 1.25*math.Min(a, b)
}

// continues says edges e and f, both meeting at vertex v, are one straight
// wall through it.
func (g *graph) continues(e, f, v int, kinds []string) (score float64, ok bool) {
	a, b := g.edges[e], g.edges[f]
	if kinds[e] != kinds[f] || !similarThickness(a.t, b.t) {
		return 0, false
	}
	if a.cls != b.cls || a.cls < 0 && math.Abs(cross(a.u, b.u)) > sinChain {
		return 0, false
	}
	da, db := unit(sub(g.pts[g.other(e, v)], g.pts[v])), unit(sub(g.pts[g.other(f, v)], g.pts[v]))
	if dot(da, db) >= 0 || math.Abs(cross(da, db)) > math.Sin(snapDeg*math.Pi/180) {
		return 0, false
	}
	return dot(da, db), true
}

func (g *graph) other(e, v int) int {
	if g.edges[e].v[0] == v {
		return g.edges[e].v[1]
	}
	return g.edges[e].v[0]
}

// chains links collinear edges through their shared vertices, pairing at
// each vertex the straightest continuations first, and walks the links into
// walls.
func (g *graph) chains(kinds []string) []chain {
	inc := make([][]int, len(g.pts))
	seen := make([]bool, len(g.edges))
	for e, ed := range g.edges {
		if ed.virtual {
			seen[e] = true
			continue
		}
		inc[ed.v[0]] = append(inc[ed.v[0]], e)
		inc[ed.v[1]] = append(inc[ed.v[1]], e)
	}
	link := make([][2]int, len(g.edges))
	for e := range link {
		link[e] = [2]int{-1, -1}
	}
	end := func(e, v int) int {
		if g.edges[e].v[0] == v {
			return 0
		}
		return 1
	}
	type pair struct {
		e, f  int
		score float64
	}
	for v, es := range inc {
		var ps []pair
		for i, e := range es {
			for _, f := range es[i+1:] {
				if s, ok := g.continues(e, f, v, kinds); ok {
					ps = append(ps, pair{e, f, s})
				}
			}
		}
		slices.SortStableFunc(ps, func(x, y pair) int { return cmpFloat(x.score, y.score) })
		for _, p := range ps {
			if link[p.e][end(p.e, v)] < 0 && link[p.f][end(p.f, v)] < 0 {
				link[p.e][end(p.e, v)], link[p.f][end(p.f, v)] = p.f, p.e
			}
		}
	}

	var out []chain
	walk := func(e, startEnd int) {
		v := g.edges[e].v[startEnd]
		c := chain{from: g.pts[v], kind: kinds[e]}
		var tl, wl float64
		for cur := e; cur >= 0 && !seen[cur]; {
			seen[cur] = true
			v = g.other(cur, v)
			l := g.length(cur)
			c.length += l
			tl += float64(g.edges[cur].t * l)
			wl += float64(g.edges[cur].w * l)
			cur = link[cur][end(cur, v)]
		}
		c.to = g.pts[v]
		c.t, c.w = tl/c.length, wl/c.length
		out = append(out, c)
	}
	for e := range g.edges {
		if seen[e] {
			continue
		}
		switch {
		case link[e][0] < 0:
			walk(e, 0)
		case link[e][1] < 0:
			walk(e, 1)
		}
	}
	// Whatever is left is a ring of links, possible only for a curve drawn
	// in steps under the bend tolerance; break it anywhere.
	for e := range g.edges {
		if !seen[e] {
			walk(e, 0)
		}
	}
	for i := range out {
		out[i].orient()
	}
	return out
}

// orient makes from come first along the wall's dominant axis, the way the
// trace format reads a and b.
func (c *chain) orient() {
	ax := axis(c.from, c.to)
	if c.to.X < c.from.X && ax == 0 || c.to.Y < c.from.Y && ax == 1 {
		c.from, c.to = c.to, c.from
	}
}

// axis is the wall's dominant axis, as plan's dominant: 0 for x.
func axis(a, b Point) int {
	if math.Abs(b.X-a.X) >= math.Abs(b.Y-a.Y) {
		return 0
	}
	return 1
}

func coord(p Point, ax int) float64 {
	if ax == 0 {
		return p.X
	}
	return p.Y
}

func cmpChain(x, y chain) int {
	for _, c := range [...]int{
		kindRank(x.kind) - kindRank(y.kind), cmpPoint(x.from, y.from), cmpPoint(x.to, y.to),
		cmpFloat(x.t, y.t),
	} {
		if c != 0 {
			return c
		}
	}
	return 0
}

// hullExterior is the fallback when the walls enclose nothing: an edge lying
// along the convex hull of all vertices is taken as exterior. If none does,
// the longest edge is.
func (g *graph) hullExterior(kinds []string) {
	h := hull(g.pts)
	found := false
	for e, ed := range g.edges {
		if ed.virtual {
			continue
		}
		a, b := g.pts[ed.v[0]], g.pts[ed.v[1]]
		for i := range h {
			p, r := h[i], h[(i+1)%len(h)]
			if len(h) == 1 {
				r = p
			}
			if segDist(a, p, r) <= mergeRadius && segDist(b, p, r) <= mergeRadius {
				kinds[e] = exterior
				found = true
				break
			}
		}
	}
	if !found {
		longest := 0
		for e := range g.edges {
			if !g.edges[e].virtual && g.length(e) > g.length(longest) {
				longest = e
			}
		}
		kinds[longest] = exterior
	}
}

// metresPerPx is the length in metres of one px along direction d, for an
// image that may be scaled differently across and down.
func metresPerPx(d Point, sx, sy float64) float64 {
	d = unit(d)
	return math.Sqrt(float64(d.X/sx*(d.X/sx)) + float64(d.Y/sy*(d.Y/sy)))
}

// thicknessM is the chain's thickness in metres, measured across it.
func (c *chain) thicknessM(sx, sy float64) float64 {
	d := c.dir()
	return c.t * metresPerPx(Point{X: -d.Y, Y: d.X}, sx, sy)
}

func (c *chain) lengthM(sx, sy float64) float64 {
	return dist(c.from, c.to) * metresPerPx(sub(c.to, c.from), sx, sy)
}

// weightedMedian is the value at half the total weight.
func weightedMedian(vs, ws []float64) float64 {
	idx := make([]int, len(vs))
	total := 0.0
	for i := range idx {
		idx[i] = i
		total += ws[i]
	}
	slices.SortStableFunc(idx, func(i, j int) int { return cmpFloat(vs[i], vs[j]) })
	acc := 0.0
	for _, i := range idx {
		acc += ws[i]
		if 2*acc >= total {
			return vs[i]
		}
	}
	return vs[idx[len(idx)-1]]
}

// mm rounds metres to the millimetre, as plan.Convert does.
func mm(v float64) float64 { return math.Round(v*1000) / 1000 }

// Default thicknesses, metres, for a kind the drawing has no walls of (or
// none with a measured thickness): typical office construction.
const (
	defaultExterior = 0.3
	defaultInterior = 0.12
	defaultGlass    = 0.1
)

// kindThickness is each kind's median thickness in metres, or its default.
func kindThickness(cs []chain, sx, sy float64) map[string]float64 {
	out := map[string]float64{
		exterior: defaultExterior,
		interior: defaultInterior,
		glass:    defaultGlass,
	}
	for _, k := range []string{exterior, interior, glass} {
		var vs, ws []float64
		for i := range cs {
			if cs[i].kind == k && cs[i].t > 0 {
				vs = append(vs, cs[i].thicknessM(sx, sy))
				ws = append(ws, cs[i].length)
			}
		}
		if len(vs) > 0 {
			if m := mm(weightedMedian(vs, ws)); m > 0 {
				out[k] = m
			}
		}
	}
	return out
}
