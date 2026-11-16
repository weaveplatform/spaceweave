package cadfront

import (
	"cmp"
	"math"
	"slices"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/floorgraph"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// Wall thresholds, in metres. A wall's faces are 50–600 mm apart: thinner
// is a finish line or a double-drawn edge, thicker a pair of walls with a
// gap. Glazing is drawn as panes millimetres apart.
const (
	minGap, maxGap           = 0.05, 0.6
	minGlassGap, maxGlassGap = 0.005, 0.3
	// minOverlap is the share of the shorter face its partner must run
	// alongside: less is two walls meeting, not one wall's two faces.
	minOverlap = 0.5
	// angleTol is how far from parallel two faces may be. CAD draws a
	// wall's faces exactly parallel; this only absorbs rounding.
	angleTol = 0.5 * math.Pi / 180
	// collinear is how close two pieces must be to be one face drawn in
	// parts (or twice).
	collinear = 0.002
	// A lone line on a wall layer is a wall drawn single-line only when it
	// is long enough not to be a jamb or an end cap.
	thinWallLen, thinGlassLen = 0.6, 0.3
	thinWall, thinGlass       = 0.1, 0.03
	// maxColumn is the largest rectangle on a wall layer taken as a
	// column rather than as four walls' faces.
	maxColumn = 1.0
)

// Weights: how sure each kind of evidence makes us that it is a wall.
const (
	weightPair       = 0.8
	weightGuessPair  = 0.55 // a pair on a layer that doesn't say it holds walls
	weightWidth      = 0.9  // a polyline drawn with the wall's width
	weightColumn     = 0.7
	weightThin       = 0.3
	weightThinGlass  = 0.5
	weightHatchBonus = 0.15 // poché between the faces
)

// wall is a wall centreline in drawing coordinates.
type wall struct {
	a, b   point
	thick  float64
	glass  bool
	weight float64
	// from is the leaf a lone line came from, -1 for walls with more
	// evidence than one line.
	from int
}

func (w wall) dir() point { return unit(w.b.Sub(w.a)) }

// piece is a straight line a wall face may be made of.
type piece struct {
	a, b  point
	theta float64 // direction, 0 to π
	leaf  int
}

// face is one or more collinear pieces, in a cluster's frame: rho across,
// t0 to t1 along.
type face struct {
	rho, t0, t1 float64
	leaves      []int
	paired      bool
}

// layered says whether the drawing names a wall layer; candidate says
// whether a leaf may hold walls, and whether they are glass.
func (r *reader) layered() bool {
	for i := range r.leaves {
		l := &r.leaves[i]
		if l.role == roleWall && !r.consumed(l) && l.owner == nil {
			switch l.e.(type) {
			case *cad.Line, *cad.Polyline:
				return true
			}
		}
	}
	return false
}

func (r *reader) candidate(l *leaf, layered bool) (ok, glass bool) {
	if l.owner != nil || r.consumed(l) {
		return false, false
	}
	switch l.role {
	case roleGlass:
		return true, true
	case roleWall:
		return true, false
	case roleNone:
		return !layered, false
	}
	return false, false
}

// pieces collects wall-face candidates. With final set it also adds the
// walls that need no pairing (polylines drawn with a width, and columns)
// and collects the hatches that fill walls; without it, it only looks, for
// estimating a scale before there is one.
func (r *reader) pieces(layered, final bool) (walls, glass []piece, hatches [][][]point) {
	for i := range r.leaves {
		l := &r.leaves[i]
		ok, isGlass := r.candidate(l, layered)
		if !ok {
			continue
		}
		add := func(a, b point) {
			a, b = l.m.Apply(a), l.m.Apply(b)
			if a == b {
				return
			}
			th := math.Atan2(b.Y-a.Y, b.X-a.X)
			if th < 0 {
				th += math.Pi
			}
			if th >= math.Pi-angleTol/2 {
				th -= math.Pi // so faces either side of horizontal sort together
			}
			p := piece{a: a, b: b, theta: th, leaf: i}
			if isGlass {
				glass = append(glass, p)
			} else {
				walls = append(walls, p)
			}
		}
		switch e := l.e.(type) {
		case *cad.Line:
			add(e.A, e.B)
		case *cad.Polyline:
			switch {
			case !final:
				for _, s := range polylineSegments(e) {
					add(s[0], s[1])
				}
			case e.Width > 0:
				r.widthWalls(l, e, isGlass)
			case layered && l.role == roleWall && r.column(l, e):
			default:
				for _, s := range polylineSegments(e) {
					add(s[0], s[1])
				}
			}
		case *cad.Hatch:
			if final && (e.Solid || layered) {
				hatches = append(hatches, rings(&e.Boundary, l.m))
				l.used = true
			}
		case *cad.Solid:
			if !final {
				continue
			}
			k, m := e.Corners, l.m
			// Corners are in DXF order: the last two swap going round.
			ring := []point{m.Apply(k[0]), m.Apply(k[1]), m.Apply(k[3]), m.Apply(k[2])}
			hatches = append(hatches, [][]point{ring})
			l.used = true
		}
	}
	return walls, glass, hatches
}

func (r *reader) widthWalls(l *leaf, p *cad.Polyline, glass bool) {
	for _, s := range polylineSegments(p) {
		r.walls = append(r.walls, wall{
			a: l.m.Apply(s[0]), b: l.m.Apply(s[1]), thick: p.Width * l.m.Factor(),
			glass: glass, weight: weightWidth, from: -1,
		})
	}
	l.used = true
}

// column takes a small closed rectangle as a column: a short, thick wall
// along its long side.
func (r *reader) column(l *leaf, p *cad.Polyline) bool {
	if !p.Closed || len(p.Vertices) != 4 {
		return false
	}
	var c [4]point
	for i, v := range p.Vertices {
		if math.Abs(v.Bulge) >= straightBulge {
			return false
		}
		c[i] = l.m.Apply(v.P)
	}
	e0, e1 := c[1].Sub(c[0]), c[2].Sub(c[1])
	l0, l1 := e0.Len(), e1.Len()
	if l0 == 0 || l1 == 0 || math.Abs(dot(e0, e1)) > 0.02*l0*l1 ||
		c[2].Sub(c[3]).Sub(e0).Len() > 0.02*l0 || math.Max(l0, l1) > r.metres(maxColumn) {
		return false
	}
	long, short := e0, l1
	if l1 > l0 {
		long, short = e1, l0
	}
	mid := c[0].Add(c[2]).Mul(0.5)
	half := long.Mul(0.5)
	r.walls = append(r.walls, wall{
		a: mid.Sub(half), b: mid.Add(half), thick: short, weight: weightColumn, from: -1,
	})
	l.used = true
	return true
}

func (r *reader) findWalls() {
	layered := r.layered()
	if !layered {
		r.note("No layer is named for walls, so walls are line pairs on every layer that isn't " +
			"named for something else; expect stray furniture or fittings among them.")
	}
	ws, gs, hatches := r.pieces(layered, true)
	weight := weightPair
	if !layered {
		weight = weightGuessPair
	}
	for _, c := range clusters(ws) {
		r.pair(c, r.metres(minGap), r.metres(maxGap), false, weight, layered)
	}
	for _, c := range clusters(gs) {
		r.pair(c, r.metres(minGlassGap), r.metres(maxGlassGap), true, weightPair, true)
	}
	for i := range r.walls {
		w := &r.walls[i]
		mid := w.a.Add(w.b).Mul(0.5)
		for _, h := range hatches {
			if inside(h, mid) {
				w.weight = math.Min(1, w.weight+weightHatchBonus)
				break
			}
		}
	}
	r.joinEnds()
}

// emitWalls hands the walls over, less lone lines that turned out to be
// door leaves drawn loose on a wall layer.
func (r *reader) emitWalls() {
	for _, w := range r.walls {
		if w.from >= 0 && r.leaves[w.from].doorLeaf {
			continue
		}
		r.in.Walls = append(r.in.Walls, floorgraph.Segment{
			A: r.toPx(
				w.a,
			),
			B:         r.toPx(w.b),
			Thickness: w.thick * r.px,
			Glass:     w.glass,
			Weight:    w.weight,
		})
	}
}

// clusters groups pieces by direction. The sort is stable, so pieces in a
// cluster keep file order and the result doesn't depend on map order.
func clusters(ps []piece) [][]piece {
	slices.SortStableFunc(ps, func(a, b piece) int { return cmp.Compare(a.theta, b.theta) })
	var out [][]piece
	start := 0
	for i := 1; i <= len(ps); i++ {
		if i == len(ps) || ps[i].theta-ps[i-1].theta > angleTol {
			out = append(out, ps[start:i])
			start = i
		}
	}
	return out
}

// frame is a cluster's shared direction: u along, n across.
func frame(c []piece) (u, n point) {
	sum := 0.0
	for _, p := range c {
		sum += p.theta
	}
	th := sum / float64(len(c))
	s, co := math.Sincos(th)
	return point{X: co, Y: s}, point{X: -s, Y: co}
}

// faces merges collinear pieces that overlap or touch into single faces,
// sorted across the cluster.
func faces(c []piece, u, n point, tol float64) []face {
	fs := make([]face, 0, len(c))
	for _, p := range c {
		t0, t1 := dot(u, p.a), dot(u, p.b)
		if t0 > t1 {
			t0, t1 = t1, t0
		}
		fs = append(
			fs,
			face{rho: dot(n, p.a.Add(p.b).Mul(0.5)), t0: t0, t1: t1, leaves: []int{p.leaf}},
		)
	}
	slices.SortStableFunc(fs, func(a, b face) int { return cmp.Compare(a.rho, b.rho) })
	var out []face
	for i := 0; i < len(fs); {
		j := i + 1
		for j < len(fs) && fs[j].rho-fs[i].rho <= tol {
			j++
		}
		line := fs[i:j]
		slices.SortStableFunc(line, func(a, b face) int { return cmp.Compare(a.t0, b.t0) })
		cur := line[0]
		for _, f := range line[1:] {
			if f.t0 <= cur.t1+tol {
				cur.t1 = math.Max(cur.t1, f.t1)
				cur.leaves = append(cur.leaves, f.leaves...)
				continue
			}
			out = append(out, cur)
			cur = f
		}
		out = append(out, cur)
		i = j
	}
	slices.SortStableFunc(out, func(a, b face) int { return cmp.Compare(a.rho, b.rho) })
	return out
}

func overlap(a0, a1, b0, b1 float64) (lo, hi float64) { return math.Max(a0, b0), math.Min(a1, b1) }

// pair finds the walls in one cluster. Faces are taken in order across it;
// each pairs with every later face, a wall's thickness away, that runs
// alongside it for at least half the shorter's length, unless a face in
// between already runs along most of that stretch (then that one is the
// partner). The search stops at the largest gap, so it costs the number
// of faces times the number within a wall's thickness, not the square of
// the cluster.
func (r *reader) pair(c []piece, minG, maxG float64, glass bool, weight float64, thin bool) {
	u, n := frame(c)
	fs := faces(c, u, n, r.metres(collinear))
	var between [][2]float64
	for i := range fs {
		a := &fs[i]
		between = between[:0]
		for j := i + 1; j < len(fs) && fs[j].rho-a.rho <= maxG; j++ {
			b := &fs[j]
			lo, hi := overlap(a.t0, a.t1, b.t0, b.t1)
			if hi <= lo {
				continue
			}
			gap := b.rho - a.rho
			shorter := math.Min(a.t1-a.t0, b.t1-b.t0)
			if gap >= minG && hi-lo >= minOverlap*shorter && !blocked(between, lo, hi) {
				r.addPair(a, b, lo, hi, u, n, glass, weight)
			}
			between = append(between, [2]float64{b.t0, b.t1})
		}
	}
	if !thin {
		return
	}
	minLen, thick, w := r.metres(thinWallLen), r.metres(thinWall), weightThin
	if glass {
		minLen, thick, w = r.metres(thinGlassLen), r.metres(thinGlass), weightThinGlass
	}
	for _, f := range fs {
		if f.paired || f.t1-f.t0 < minLen {
			continue
		}
		from := -1
		if len(f.leaves) == 1 {
			from = f.leaves[0]
		}
		r.walls = append(r.walls, wall{
			a: u.Mul(f.t0).Add(n.Mul(f.rho)), b: u.Mul(f.t1).Add(n.Mul(f.rho)),
			thick: thick, glass: glass, weight: w, from: from,
		})
		r.markUsed(f.leaves)
	}
}

// blocked reports whether a face already seen runs along most of lo–hi.
func blocked(between [][2]float64, lo, hi float64) bool {
	for _, k := range between {
		if l, h := overlap(lo, hi, k[0], k[1]); h-l >= minOverlap*(hi-lo) {
			return true
		}
	}
	return false
}

// addPair makes the wall between two faces. It runs along their common
// stretch, and on past it where one face overshoots the other by no more
// than a wall's thickness: that is an L corner, where the outer face runs
// on across the other wall, and half the overshoot lands the end on the
// corner. A longer overshoot is a T, the face running on past a wall that
// meets this one from the other side; joinEnds finishes those.
func (r *reader) addPair(a, b *face, lo, hi float64, u, n point, glass bool, weight float64) {
	corner := func(over float64) float64 {
		if over <= r.metres(maxGap) {
			return over / 2
		}
		return 0
	}
	start := lo - corner(lo-math.Min(a.t0, b.t0))
	end := hi + corner(math.Max(a.t1, b.t1)-hi)
	mid := (a.rho + b.rho) / 2
	r.walls = append(r.walls, wall{
		a: u.Mul(start).Add(n.Mul(mid)), b: u.Mul(end).Add(n.Mul(mid)),
		thick: b.rho - a.rho, glass: glass, weight: weight, from: -1,
	})
	a.paired, b.paired = true, true
	r.markUsed(a.leaves)
	r.markUsed(b.leaves)
}

func (r *reader) markUsed(leaves []int) {
	for _, i := range leaves {
		r.leaves[i].used = true
	}
}

// gapMode is the commonest distance between a line and the nearest
// parallel line running alongside it, in drawing units, or 0 when there
// are none. In a plan most such pairs are a wall's two faces.
func (r *reader) gapMode() float64 {
	ws, _, _ := r.pieces(r.layered(), false)
	var all vector.Rect
	for _, p := range ws {
		all = all.Extend(p.a).Extend(p.b)
	}
	if all.Empty() {
		return 0
	}
	// Nothing a twentieth of the drawing across is a wall; stopping there
	// keeps the search short on big drawings.
	limit := math.Max(all.Size().X, all.Size().Y) / 20
	var gaps []float64
	for _, c := range clusters(ws) {
		u, n := frame(c)
		fs := faces(c, u, n, 0)
		for i := range fs {
			for j := i + 1; j < len(fs) && fs[j].rho-fs[i].rho <= limit; j++ {
				lo, hi := overlap(fs[i].t0, fs[i].t1, fs[j].t0, fs[j].t1)
				shorter := math.Min(fs[i].t1-fs[i].t0, fs[j].t1-fs[j].t0)
				if gap := fs[j].rho - fs[i].rho; gap > 0 && hi > lo && hi-lo >= minOverlap*shorter {
					gaps = append(gaps, gap)
					break
				}
			}
		}
	}
	if len(gaps) == 0 {
		return 0
	}
	// Gaps cluster on a log scale (100, 150, 200 mm walls); eighth-octave
	// bins are fine enough to separate them and coarse enough to fill.
	bins := map[int]int{}
	for _, g := range gaps {
		bins[int(math.Floor(8*math.Log2(g)))]++
	}
	keys := make([]int, 0, len(bins))
	for k := range bins {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	best := keys[0]
	for _, k := range keys {
		if bins[k] > bins[best] {
			best = k
		}
	}
	return math.Exp2((float64(best) + 0.5) / 8)
}
