package cadfront

import (
	"cmp"
	"math"
	"slices"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/floorgraph"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// Door swings: a quarter turn, give or take, of a leaf 0.6–1.25 m wide.
// A block named as a door may hold narrower or wider leaves (half of a
// double door, a wide accessible door) and is believed.
const (
	minSweep, maxSweep         = 60 * math.Pi / 180, 120 * math.Pi / 180
	minLeaf, maxLeaf           = 0.6, 1.25
	minNamedLeaf, maxNamedLeaf = 0.3, 1.6
	// leafTol is how near, as a share of the radius, a leaf line's ends
	// must be to the hinge and to the arc's end.
	leafTol = 0.15
	// maxLeafLine is the longest line looked at as a door leaf, in metres.
	maxLeafLine = 2.0
	// doorBlockSpan bounds an unnamed door block: its swings across, plus
	// a frame.
	doorBlockSpan, doorFrame = 2.2, 0.3
)

// Opening weights.
const (
	weightNamedDoor  = 0.95 // a block named or layered as a door, with its swing
	weightSwingBlock = 0.85 // an unnamed block that is a door by its swing
	weightLayerArc   = 0.8  // a loose swing on a door layer
	weightLooseArc   = 0.6  // a loose swing with its leaf, on any layer
	weightSpan       = 0.8  // a door or window block lying in a wall
	weightLoneSpan   = 0.5  // ... with no wall found under it
)

// parallelTol is the sine of the largest angle between an opening and the
// wall it is snapped to.
var parallelTol = math.Sin(15 * math.Pi / 180)

// swing is a door's arc, in drawing coordinates.
type swing struct {
	hinge, e1, e2, mid point
	rad, sweep         float64
	leaf               int
}

// opening is an Opening in drawing coordinates.
type opening struct {
	a, b, leaf     point
	kind           string
	hingeA, leafed bool
	weight         float64
	rad            float64 // the leaf's width, for pairing single doors into doubles
}

// swings are the arcs in a leaf: an ARC, or the bulges of a polyline.
func swings(l *leaf, i int) []swing {
	var out []swing
	add := func(c point, rad, start, sweep float64, m vector.Affine) {
		e1, e2 := arcPoint(c, rad, start), arcPoint(c, rad, start+sweep)
		mid := arcPoint(c, rad, start+sweep/2)
		h := m.Apply(c)
		e1, e2, mid = m.Apply(e1), m.Apply(e2), m.Apply(mid)
		out = append(out, swing{
			hinge: h, e1: e1, e2: e2, mid: mid,
			rad: (e1.Sub(h).Len() + e2.Sub(h).Len()) / 2, sweep: math.Abs(sweep), leaf: i,
		})
	}
	switch e := l.e.(type) {
	case *cad.Arc:
		add(e.Center, e.Radius, e.Start, ccwEnd(e.Start, e.End)-e.Start, l.m)
	case *cad.Ellipse:
		// A door block scaled unevenly, then exploded, leaves its swing as
		// an elliptical arc; nearly round, it is still the swing, and its
		// leaf is as long as the ends are from the hinge.
		if e.Ratio < minEllipse || e.Ratio > 1/minEllipse {
			break
		}
		minor := point{X: -e.Major.Y, Y: e.Major.X}.Mul(e.Ratio)
		at := func(t float64) point {
			s, c := math.Sincos(t)
			return l.m.Apply(e.Center.Add(e.Major.Mul(c)).Add(minor.Mul(s)))
		}
		end := ccwEnd(e.Start, e.End)
		h, e1, e2 := l.m.Apply(e.Center), at(e.Start), at(end)
		sweep := angleBetween(e1.Sub(h), e2.Sub(h))
		out = append(out, swing{
			hinge: h, e1: e1, e2: e2, mid: at((e.Start + end) / 2),
			rad: (e1.Sub(h).Len() + e2.Sub(h).Len()) / 2, sweep: sweep, leaf: i,
		})
	case *cad.Polyline:
		n := len(e.Vertices)
		for k, v := range e.Vertices {
			if math.Abs(v.Bulge) < straightBulge || (k == n-1 && !e.Closed) {
				continue
			}
			if next := e.Vertices[(k+1)%n].P; next != v.P {
				c, rad, start, sweep := bulgeArc(v.P, next, v.Bulge)
				add(c, rad, start, sweep, l.m)
			}
		}
	}
	return out
}

// minEllipse is the least minor-to-major ratio of an elliptical arc read
// as a door swing.
const minEllipse = 0.8

// angleBetween is the unsigned angle between two vectors.
func angleBetween(a, b point) float64 { return math.Abs(math.Atan2(cross(a, b), dot(a, b))) }

func (r *reader) isSwing(s swing, lo, hi float64) bool {
	return s.sweep >= minSweep && s.sweep <= maxSweep && s.rad >= r.metres(lo) &&
		s.rad <= r.metres(hi)
}

// subtree lists the leaves in a block reference and everything inside it.
func (r *reader) subtree(n *node, out []int) []int {
	out = append(out, n.leaves...)
	for _, k := range n.kids {
		out = r.subtree(k, out)
	}
	return out
}

// doorRadius is the median swing radius of what is plainly a door (in a
// block or on a layer named for doors), in drawing units, or 0. It is the
// scale estimate of last resort's best friend: door leaves vary less than
// anything else drawn on a plan.
func (r *reader) doorRadius() float64 {
	var radii []float64
	for i := range r.leaves {
		l := &r.leaves[i]
		inDoor := l.role == roleDoor && l.node == nil
		for n := l.node; n != nil && !inDoor; n = n.parent {
			inDoor = n.role == roleDoor
		}
		if !inDoor {
			continue
		}
		for _, s := range swings(l, i) {
			if s.sweep >= minSweep && s.sweep <= maxSweep {
				radii = append(radii, s.rad)
			}
		}
	}
	if len(radii) == 0 {
		return 0
	}
	slices.Sort(radii)
	return radii[len(radii)/2]
}

// swingBlock reports whether an unnamed block reference is a door: one or
// two door swings of its own, in something no bigger than they are.
func (r *reader) swingBlock(n *node) bool {
	if r.unit == 0 || n.ext.Empty() {
		return false
	}
	count, span := 0, 0.0
	for _, li := range n.leaves {
		for _, s := range swings(&r.leaves[li], li) {
			if r.isSwing(s, minLeaf, maxLeaf) {
				count++
				span += s.rad
			}
		}
	}
	size := n.ext.Size()
	return (count == 1 || count == 2) &&
		math.Max(size.X, size.Y) <= doorBlockSpan*span+r.metres(doorFrame)
}

// straights are the straight pieces of the given leaves.
func (r *reader) straights(leaves []int) []segment {
	var out []segment
	for _, i := range leaves {
		l := &r.leaves[i]
		switch e := l.e.(type) {
		case *cad.Line:
			out = append(out, segment{l.m.Apply(e.A), l.m.Apply(e.B), i})
		case *cad.Polyline:
			for _, s := range polylineSegments(e) {
				out = append(out, segment{l.m.Apply(s[0]), l.m.Apply(s[1]), i})
			}
		}
	}
	return out
}

func (r *reader) findDoors() []opening {
	walls := r.wallGrid()
	var doors, spans []opening
	for _, n := range r.nodes {
		if n.top != n || n.role != roleDoor {
			continue
		}
		leaves := r.subtree(n, nil)
		lo, hi, weight := minNamedLeaf, maxNamedLeaf, weightNamedDoor
		if blockRole(n.ins.Block, n.info.lines) != roleDoor {
			lo, hi, weight = minLeaf, maxLeaf, weightSwingBlock
		}
		var ss []swing
		for _, li := range leaves {
			for _, s := range swings(&r.leaves[li], li) {
				if r.isSwing(s, lo, hi) {
					ss = append(ss, s)
				}
			}
		}
		r.use(n)
		if len(ss) == 0 {
			spans = append(spans, r.span(walls, r.nodePoints(n), n.m, floorgraph.Door))
			continue
		}
		lines := r.straights(leaves)
		doors = append(doors, r.blockDoors(walls, ss, lines, weight)...)
	}
	doors = append(doors, r.looseDoors(walls)...)
	return append(pairDoors(doors), spans...)
}

// blockDoors makes the doors of one block's swings: a double door when two
// hinges are as far apart as their leaves are wide, else a door each.
func (r *reader) blockDoors(walls *grid, ss []swing, lines []segment, weight float64) []opening {
	slices.SortStableFunc(ss, func(a, b swing) int { return cmp.Compare(a.leaf, b.leaf) })
	if len(ss) == 2 {
		a, b := ss[0], ss[1]
		if d := a.hinge.Sub(b.hinge).Len(); d >= 0.7*(a.rad+b.rad) && d <= 1.3*(a.rad+b.rad) {
			o := opening{
				a: a.hinge, b: b.hinge, leaf: a.mid, leafed: true, kind: floorgraph.DoubleDoor,
				weight: weight, rad: a.rad + b.rad,
			}
			r.snap(walls, &o)
			return []opening{o}
		}
	}
	out := make([]opening, 0, len(ss))
	for _, s := range ss {
		out = append(out, r.door(walls, s, lines, weight))
	}
	return out
}

// door makes a single door from its swing. The opening runs from the
// hinge to whichever end of the arc is the leaf's closed position: the
// end the drawn leaf doesn't reach, else the end that lies along a wall,
// else the arc's start.
func (r *reader) door(walls *grid, s swing, lines []segment, weight float64) opening {
	closed, _, ok := leafEnd(s, lines)
	if !ok {
		closed = r.wallEnd(walls, s)
	}
	o := opening{
		a: s.hinge, b: closed, leaf: s.mid, leafed: true, hingeA: true,
		kind: floorgraph.Door, weight: weight, rad: s.rad,
	}
	r.snap(walls, &o)
	return o
}

// leafEnd finds the leaf drawn open, from the hinge to one end of the arc,
// and returns the other end and the leaf's leaf (entity) index. A line to
// each end (a leaf and a threshold) settles nothing.
func leafEnd(s swing, lines []segment) (point, int, bool) {
	tol := leafTol * s.rad
	to1, to2 := -1, -1
	for _, l := range lines {
		for _, ends := range [][2]point{{l.a, l.b}, {l.b, l.a}} {
			if ends[0].Sub(s.hinge).Len() > tol {
				continue
			}
			if ends[1].Sub(s.e1).Len() <= tol && to1 < 0 {
				to1 = l.leaf
			}
			if ends[1].Sub(s.e2).Len() <= tol && to2 < 0 {
				to2 = l.leaf
			}
		}
	}
	switch {
	case to1 >= 0 && to2 < 0:
		return s.e2, to1, true
	case to2 >= 0 && to1 < 0:
		return s.e1, to2, true
	}
	return point{}, -1, false
}

func (r *reader) wallEnd(walls *grid, s swing) point {
	best, bestD := s.e1, math.Inf(1)
	for _, e := range []point{s.e1, s.e2} {
		d := unit(e.Sub(s.hinge))
		for _, j := range walls.near(s.hinge, s.rad) {
			w := r.walls[j]
			wd := w.dir()
			if math.Abs(cross(d, wd)) > parallelTol {
				continue
			}
			dist := math.Abs(cross(wd, s.hinge.Sub(w.a))) + math.Abs(cross(wd, e.Sub(w.a)))
			if dist <= w.thick+leafTol*s.rad && dist < bestD {
				best, bestD = e, dist
			}
		}
	}
	return best
}

// snap moves an opening's ends onto the centreline of the wall it lies
// in, when there is one parallel to it within half a wall of it.
func (r *reader) snap(walls *grid, o *opening) {
	if j, ok := r.host(walls, o.a, o.b); ok {
		w := r.walls[j]
		d := w.dir()
		o.a = w.a.Add(d.Mul(dot(o.a.Sub(w.a), d)))
		o.b = w.a.Add(d.Mul(dot(o.b.Sub(w.a), d)))
	}
}

// hostReach is how far, in metres, an opening may lie beyond the end of the
// wall it belongs to: walls stop at the jambs, so an opening is usually
// just past one.
const hostReach = 1.5

// host finds the wall an opening from a to b lies in: parallel to it,
// within half its thickness (and a little) of the opening's middle, and
// not far past its ends.
func (r *reader) host(walls *grid, a, b point) (int, bool) {
	mid, d := a.Add(b).Mul(0.5), unit(b.Sub(a))
	best, bestD := -1, math.Inf(1)
	for _, j := range walls.near(mid, r.metres(hostReach)+a.Sub(b).Len()/2) {
		w := r.walls[j]
		wd := w.dir()
		if math.Abs(cross(d, wd)) > parallelTol {
			continue
		}
		dist := math.Abs(cross(wd, mid.Sub(w.a)))
		t, reach := dot(wd, mid.Sub(w.a)), r.metres(hostReach)+a.Sub(b).Len()/2
		if dist > w.thick/2+r.metres(minGap) || t < -reach || t > w.b.Sub(w.a).Len()+reach ||
			dist >= bestD {
			continue
		}
		best, bestD = j, dist
	}
	return best, best >= 0
}

// looseDoors are swings drawn outside any door block: on a door layer, or
// on any layer that may hold walls when the leaf is drawn too.
func (r *reader) looseDoors(walls *grid) []opening {
	var short []int
	for i := range r.leaves {
		if l := &r.leaves[i]; !r.consumed(l) && l.owner == nil {
			short = append(short, i)
		}
	}
	lines := newGrid(r.metres(gridCell))
	var segs []segment
	for _, s := range r.straights(short) {
		if s.b.Sub(s.a).Len() <= r.metres(maxLeafLine) {
			lines.add(len(segs), s.a, s.b)
			segs = append(segs, s)
		}
	}
	var out []opening
	for _, i := range short {
		l := &r.leaves[i]
		switch l.role {
		case roleDoor, roleWall, roleGlass, roleNone:
		default:
			continue
		}
		lo, hi := minLeaf, maxLeaf
		if l.role == roleDoor {
			// A door layer is believed as a door block is: a cupboard's
			// narrow leaf or a wide accessible one is still a door.
			lo, hi = minNamedLeaf, maxNamedLeaf
		}
		for _, s := range swings(l, i) {
			if !r.isSwing(s, lo, hi) {
				continue
			}
			near := lines.near(s.hinge, s.rad)
			cands := make([]segment, 0, len(near))
			for _, k := range near {
				cands = append(cands, segs[k])
			}
			_, leafLine, drawn := leafEnd(s, cands)
			weight := weightLooseArc
			switch {
			case l.role == roleDoor:
				weight = weightLayerArc
			case !drawn:
				continue
			}
			l.used = true
			if drawn {
				r.leaves[leafLine].doorLeaf = true
				r.leaves[leafLine].used = true
			}
			out = append(out, r.door(walls, s, cands, weight))
		}
	}
	return out
}

// pairDoors joins single doors whose closed leaves meet into double doors.
func pairDoors(doors []opening) []opening {
	done := make([]bool, len(doors))
	var out []opening
	for i := range doors {
		if done[i] {
			continue
		}
		a := doors[i]
		for j := i + 1; j < len(doors) && a.kind == floorgraph.Door; j++ {
			b := doors[j]
			if done[j] || b.kind != floorgraph.Door ||
				a.b.Sub(b.b).Len() > leafTol*math.Min(a.rad, b.rad) ||
				dot(a.a.Sub(a.b), b.a.Sub(b.b)) >= 0 {
				continue
			}
			a = opening{
				a: a.a, b: b.a, leaf: a.leaf, leafed: true, kind: floorgraph.DoubleDoor,
				weight: math.Max(a.weight, b.weight), rad: a.rad + b.rad,
			}
			done[j] = true
		}
		out = append(out, a)
	}
	return out
}

// nodePoints samples everything in a block reference, in drawing
// coordinates.
func (r *reader) nodePoints(n *node) []point {
	leaves := r.subtree(n, nil)
	out := make([]point, 0, 2*len(leaves))
	for _, li := range leaves {
		out = append(out, r.points(li)...)
	}
	return out
}

// span makes an opening across a block that is a door or window without a
// swing: its extent along the wall it lies in, or, with no wall, along its
// own longer side (m maps its own axes into the drawing).
func (r *reader) span(walls *grid, pts []point, m vector.Affine, kind string) opening {
	o := opening{kind: kind, weight: weightSpan}
	if len(pts) == 0 {
		return o
	}
	var box vector.Rect
	inv, ok := inverse(m)
	if !ok {
		inv, m = vector.Identity, vector.Identity
	}
	for _, p := range pts {
		box = box.Extend(inv.Apply(p))
	}
	size, c := box.Size(), box.Center()
	axis, across := point{X: size.X / 2}, point{Y: size.Y / 2}
	if size.Y > size.X {
		axis, across = across, axis
	}
	a, b := m.Apply(c.Sub(axis)), m.Apply(c.Add(axis))
	j, found := r.host(walls, a, b)
	if !found {
		// A block can be deeper than it is wide (a narrow window in a
		// thick wall), so try its other side before giving up on a wall.
		j, found = r.host(walls, m.Apply(c.Sub(across)), m.Apply(c.Add(across)))
	}
	if !found {
		o.a, o.b, o.weight = a, b, weightLoneSpan
		return o
	}
	w := r.walls[j]
	d := w.dir()
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, p := range pts {
		t := dot(p.Sub(w.a), d)
		lo, hi = math.Min(lo, t), math.Max(hi, t)
	}
	o.a, o.b = w.a.Add(d.Mul(lo)), w.a.Add(d.Mul(hi))
	return o
}

// findOpenings finds doors, window blocks, door blocks without a swing, and
// loose geometry on window layers, and adds every opening.
func (r *reader) findOpenings() {
	ops := r.findDoors()
	walls := r.wallGrid()
	for _, n := range r.nodes {
		if n.top == n && n.role == roleWindow {
			r.use(n)
			ops = append(ops, r.span(walls, r.nodePoints(n), n.m, floorgraph.Window))
		}
	}
	for _, group := range r.windows {
		var pts []point
		for _, i := range group {
			pts = append(pts, r.points(i)...)
			r.leaves[i].used = true
		}
		ops = append(ops, r.span(walls, pts, vector.Identity, floorgraph.Window))
	}
	ops = append(ops, r.doorways(ops)...)
	for _, o := range ops {
		if o.a == o.b {
			continue
		}
		r.in.Openings = append(r.in.Openings, floorgraph.Opening{
			A: r.toPx(o.a), B: r.toPx(o.b), Kind: o.kind, HingeA: o.hingeA,
			Leaf: r.toPx(o.leaf), Leafed: o.leafed, Weight: o.weight,
		})
	}
}

// curtainSpan is the longest a loose window, in metres, is taken to be: a
// window layer's ink that runs on further is curtain walling or a ribbon
// of glazing, a glass wall rather than an opening in one.
const curtainSpan = 3.0

// findCurtains splits the window layers' loose ink into windows, kept for
// findOpenings, and curtain walling, paired here as glass walls. A window
// is short and sits in a wall; glazing that runs on, or fills a gap with
// no wall either side (a glazed bay between a façade's pier and a core),
// is the wall there.
func (r *reader) findCurtains(layered bool) {
	walls := r.wallGrid()
	var curtain []int
	for _, g := range r.windowGroups() {
		var box vector.Rect
		var pts []point
		for _, i := range g {
			box = box.Union(r.leaves[i].box)
			pts = append(pts, r.points(i)...)
		}
		if s := box.Size(); math.Max(s.X, s.Y) <= r.metres(curtainSpan) &&
			r.span(walls, pts, vector.Identity, floorgraph.Window).weight != weightLoneSpan {
			r.windows = append(r.windows, g)
			continue
		}
		for _, i := range g {
			r.leaves[i].curtain = true
		}
		curtain = append(curtain, g...)
	}
	if len(curtain) == 0 {
		return
	}
	slices.Sort(curtain)
	_, gs, _ := r.piecesOf(curtain, layered, true)
	for _, c := range clusters(gs) {
		r.pair(c, r.metres(minGlassGap), r.metres(maxGlassGap), true, weightPair, true)
	}
}

// windowGroups gathers loose entities on window layers into touching
// groups, one per window: sorted by left edge, each box is compared only
// with the boxes still open at that edge.
func (r *reader) windowGroups() [][]int {
	var ids []int
	for i := range r.leaves {
		if l := &r.leaves[i]; l.role == roleWindow && l.owner == nil && !r.consumed(l) &&
			!l.box.Empty() {
			ids = append(ids, i)
		}
	}
	slices.SortStableFunc(ids, func(a, b int) int {
		return cmp.Compare(r.leaves[a].box.Min.X, r.leaves[b].box.Min.X)
	})
	parent := make(map[int]int, len(ids))
	var find func(int) int
	find = func(i int) int {
		if p, ok := parent[i]; ok && p != i {
			parent[i] = find(p)
			return parent[i]
		}
		return i
	}
	tol := r.metres(collinear) * 10
	var open []int
	for _, i := range ids {
		bi := r.leaves[i].box
		parent[i] = i
		// Boxes that end left of this one can't touch it or any after it.
		keep := open[:0]
		for _, j := range open {
			bj := r.leaves[j].box
			if bj.Max.X+tol < bi.Min.X {
				continue
			}
			keep = append(keep, j)
			if bj.Min.Y <= bi.Max.Y+tol && bi.Min.Y <= bj.Max.Y+tol {
				if a, b := find(i), find(j); a != b {
					parent[max(a, b)] = min(a, b)
				}
			}
		}
		keep = append(keep, i)
		open = keep
	}
	groups := map[int][]int{}
	var roots []int
	for _, i := range ids {
		root := find(i)
		if _, ok := groups[root]; !ok {
			roots = append(roots, root)
		}
		groups[root] = append(groups[root], i)
	}
	out := make([][]int, 0, len(roots))
	for _, root := range roots {
		out = append(out, groups[root])
	}
	return out
}
