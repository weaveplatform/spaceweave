package floorgraph

import (
	"math"
	"slices"
)

// Tolerances for cleaning the evidence, in px or degrees.
const (
	// maxCoord bounds a usable coordinate: past it the products below
	// overflow, and no drawing is a million pixels across.
	maxCoord = 1e6
	// snapDeg is how far off a dominant direction a wall may be drawn and
	// still be taken as meant to lie on it.
	snapDeg = 2.0
	// slantShare is the share of wall length a slanted direction needs to
	// count as dominant: enough to be a facade, not a stray diagonal.
	slantShare = 0.05
	// minTol is the floor of every thickness-based tolerance, for evidence
	// that came without a thickness.
	minTol = 2.0
)

// seg is a stretch of wall centreline being cleaned. a comes before b along
// u, a unit direction shared by every segment of the same class.
type seg struct {
	a, b  Point
	u     Point
	cls   int // index into the dominant directions, or -1 when not snapped
	t     float64
	w     float64
	glass bool
}

func newSeg(a, b, u Point, cls int, t, w float64, glass bool) seg {
	if dot(a, u) > dot(b, u) {
		a, b = b, a
	}
	return seg{a: a, b: b, u: u, cls: cls, t: t, w: w, glass: glass}
}

func (s seg) length() float64 { return dist(s.a, s.b) }

func (s seg) normal() Point { return Point{X: -s.u.Y, Y: s.u.X} }

// off is the line's signed distance from the origin along its normal.
func (s seg) off() float64 { return dot(mid(s.a, s.b), s.normal()) }

func (s seg) span() (lo, hi float64) { return dot(s.a, s.u), dot(s.b, s.u) }

func (s seg) tol() float64 { return math.Max(s.t, minTol) }

// column says the segment is a pier or column: no longer than it is thick.
// Corner closing only joins columns to walls they touch, since extending one
// further would turn a free-standing pier into a wall.
func (s seg) column() bool { return s.t > 0 && s.length() <= 1.5*s.t }

func cmpSeg(x, y seg) int {
	for _, c := range [...]int{
		cmpPoint(x.a, y.a), cmpPoint(x.b, y.b), cmpFloat(x.t, y.t), cmpFloat(x.w, y.w),
		cmpBool(x.glass, y.glass), x.cls - y.cls,
	} {
		if c != 0 {
			return c
		}
	}
	return 0
}

func cmpBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	}
	return -1
}

// cleanWalls drops what can't be a wall and puts the rest in a canonical
// order, so nothing downstream depends on the order the front end found
// them in. bad counts pieces with unusable numbers.
func cleanWalls(in []Segment) (segs []seg, bad int) {
	segs = make([]seg, 0, len(in))
	for _, s := range in {
		if !finite(s.A.X, s.A.Y, s.B.X, s.B.Y) ||
			max(math.Abs(s.A.X), math.Abs(s.A.Y), math.Abs(s.B.X), math.Abs(s.B.Y)) > maxCoord {
			bad++
			continue
		}
		a, b := qp(s.A), qp(s.B)
		if a == b {
			continue
		}
		t := s.Thickness
		if !finite(t) || t < 0 {
			t = 0
		}
		w := s.Weight
		if !finite(w) {
			w = 0
		}
		segs = append(
			segs,
			newSeg(
				a,
				b,
				canonical(unit(sub(b, a))),
				-1,
				q(min(t, maxCoord)),
				clamp(w, 0, 1),
				s.Glass,
			),
		)
	}
	slices.SortFunc(segs, cmpSeg)
	return segs, bad
}

// angleOf is the segment's direction in degrees, folded into [0, 180) and
// rounded to a thousandth so the histogram below can't split on float noise.
func angleOf(s seg) float64 {
	d := sub(s.b, s.a)
	a := math.Round(math.Atan2(d.Y, d.X)*180/math.Pi*1000) / 1000
	for a < 0 {
		a += 180
	}
	for a >= 180 {
		a -= 180
	}
	return a
}

func angleDiff(a, b float64) float64 {
	d := math.Abs(a - b)
	return math.Min(d, 180-d)
}

// dominantDirections are the directions walls are snapped to: always
// horizontal and vertical, plus any slant carrying slantShare of the wall
// length (a splayed wing, a diagonal facade).
func dominantDirections(segs []seg) []float64 {
	var hist [180]float64
	total := 0.0
	angles := make([]float64, len(segs))
	for i, s := range segs {
		angles[i] = angleOf(s)
		hist[int(angles[i])%180] += s.length()
		total += s.length()
	}
	type peak struct {
		bin int
		w   float64
	}
	var peaks []peak
	// Bins whose ±2° window stays clear of 0, 90 and 180, which are
	// dominant anyway.
	for b := 3; b < 177; b++ {
		if b >= 87 && b < 93 {
			continue
		}
		w := 0.0
		for k := b - 2; k <= b+2; k++ {
			w += hist[k]
		}
		if w >= slantShare*total {
			peaks = append(peaks, peak{b, w})
		}
	}
	slices.SortStableFunc(peaks, func(x, y peak) int { return cmpFloat(y.w, x.w) })
	dirs := []float64{0, 90}
	for _, p := range peaks {
		// The peak's angle is the length-weighted mean of the walls near it.
		c := float64(p.bin) + 0.5
		sum, wsum := 0.0, 0.0
		for i, a := range angles {
			if math.Abs(a-c) <= 2.5 {
				sum += float64(a * segs[i].length())
				wsum += segs[i].length()
			}
		}
		a := math.Round(sum/wsum*1000) / 1000
		if !slices.ContainsFunc(dirs, func(d float64) bool { return angleDiff(a, d) < 2*snapDeg }) {
			dirs = append(dirs, a)
		}
	}
	return dirs
}

// dirUnit is the canonical unit vector of a direction in degrees. The two
// axes are exact so that snapped walls on them are exactly straight.
func dirUnit(deg float64) Point {
	switch deg {
	case 0:
		return Point{X: 1}
	case 90:
		return Point{Y: 1}
	}
	r := deg * math.Pi / 180
	return canonical(Point{X: math.Cos(r), Y: math.Sin(r)})
}

// snap turns each wall drawn within snapDeg of a dominant direction onto it
// exactly, about its midpoint: a wall drawn half a degree off keeps its
// position and loses the tilt.
func snap(segs []seg, dirs []float64) []seg {
	out := make([]seg, 0, len(segs))
	for _, s := range segs {
		a := angleOf(s)
		best, bestD := -1, snapDeg
		for i, d := range dirs {
			if diff := angleDiff(a, d); diff <= bestD {
				best, bestD = i, diff
			}
		}
		if best < 0 {
			out = append(out, s)
			continue
		}
		u := dirUnit(dirs[best])
		m := mid(s.a, s.b)
		var pa, pb Point
		switch dirs[best] {
		case 0:
			pa, pb = Point{X: s.a.X, Y: q(m.Y)}, Point{X: s.b.X, Y: q(m.Y)}
		case 90:
			pa, pb = Point{X: q(m.X), Y: s.a.Y}, Point{X: q(m.X), Y: s.b.Y}
		default:
			h := dot(sub(s.b, s.a), u) / 2
			pa, pb = qp(addScaled(m, u, -h)), qp(addScaled(m, u, h))
		}
		if pa == pb {
			continue
		}
		out = append(out, newSeg(pa, pb, u, best, s.t, s.w, s.glass))
	}
	slices.SortFunc(out, cmpSeg)
	return out
}

// mergeable says two parallel segments are one wall: their lines within half
// the larger thickness and their spans overlapping or nearly touching.
// Touching pieces of different kinds (a glazed screen running on from a solid
// partition) stay apart; overlapping ones are one wall seen twice. When the
// thicknesses differ a lot (a column set into a wall's face) the lines must
// be within half the smaller one, or the column would drag the wall's line
// off its centre.
func mergeable(x, y seg) bool {
	tol := math.Max(x.tol(), y.tol())
	near := tol / 2
	if !similarThickness(x.t, y.t) {
		near = math.Min(x.tol(), y.tol()) / 2
	}
	if math.Abs(x.off()-y.off()) >= near {
		return false
	}
	xlo, xhi := x.span()
	ylo, yhi := y.span()
	gap := math.Max(xlo, ylo) - math.Min(xhi, yhi)
	if gap > tol {
		return false
	}
	return x.glass == y.glass || gap < -tol
}

// mergeCollinear joins collinear pieces of one wall. Only snapped segments
// merge: two unsnapped ones are parallel only by accident.
func mergeCollinear(segs []seg) []seg {
	uf := newUnionFind(len(segs))
	byCls := map[int][]int{}
	maxCls := -1
	for i, s := range segs {
		if s.cls >= 0 {
			byCls[s.cls] = append(byCls[s.cls], i)
			maxCls = max(maxCls, s.cls)
		}
	}
	for c := 0; c <= maxCls; c++ {
		idx := byCls[c]
		maxTol := 0.0
		for _, i := range idx {
			maxTol = math.Max(maxTol, segs[i].tol())
		}
		slices.SortStableFunc(
			idx,
			func(i, j int) int { return cmpFloat(segs[i].off(), segs[j].off()) },
		)
		for k, i := range idx {
			for _, j := range idx[k+1:] {
				if segs[j].off()-segs[i].off() >= maxTol/2 {
					break
				}
				if mergeable(segs[i], segs[j]) {
					uf.union(i, j)
				}
			}
		}
	}
	return combine(segs, uf, nil)
}

// combine replaces each union-find group with one segment along the line of
// its longest member: the length-weighted mean offset, thickness and weight,
// the union of the spans (and of extra, points the group must reach), and
// glass when most of the length is glass.
func combine(segs []seg, uf unionFind, extra map[int][]Point) []seg {
	groups := map[int][]int{}
	roots := []int{}
	for i := range segs {
		r := uf.find(i)
		if _, ok := groups[r]; !ok {
			roots = append(roots, r)
		}
		groups[r] = append(groups[r], i)
	}
	out := make([]seg, 0, len(roots))
	for _, r := range roots {
		members := groups[r]
		if len(members) == 1 && len(extra[r]) == 0 {
			out = append(out, segs[r])
			continue
		}
		rep := segs[members[0]]
		for _, i := range members[1:] {
			if segs[i].length() > rep.length() {
				rep = segs[i]
			}
		}
		u, n := rep.u, rep.normal()
		var total, off, t, w, glassLen float64
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, i := range members {
			s := segs[i]
			l := s.length()
			total += l
			off += float64(dot(mid(s.a, s.b), n) * l)
			t += float64(s.t * l)
			w += float64(s.w * l)
			if s.glass {
				glassLen += l
			}
			for _, p := range [2]Point{s.a, s.b} {
				lo, hi = math.Min(lo, dot(p, u)), math.Max(hi, dot(p, u))
			}
		}
		for _, p := range extra[r] {
			lo, hi = math.Min(lo, dot(p, u)), math.Max(hi, dot(p, u))
		}
		off /= total
		base := Point{X: float64(n.X * off), Y: float64(n.Y * off)}
		a, b := qp(addScaled(base, u, lo)), qp(addScaled(base, u, hi))
		if a == b {
			continue
		}
		out = append(out, newSeg(a, b, u, rep.cls, q(t/total), w/total, 2*glassLen > total))
	}
	slices.SortFunc(out, cmpSeg)
	return out
}

// opening is an Opening in working form.
type opening struct {
	Opening

	matched bool
}

// knownKind lists the opening kinds plan.Opening accepts.
var knownKind = map[string]bool{Door: true, DoubleDoor: true, Window: true, Gap: true}

// cleanOpenings drops openings with unusable numbers or no width (bad) and
// files unknown kinds as plain gaps (odd), in a canonical order.
func cleanOpenings(in []Opening) (ops []opening, bad, odd int) {
	for _, o := range in {
		if !finite(o.A.X, o.A.Y, o.B.X, o.B.Y) ||
			max(math.Abs(o.A.X), math.Abs(o.A.Y), math.Abs(o.B.X), math.Abs(o.B.Y)) > maxCoord {
			bad++
			continue
		}
		o.A, o.B = qp(o.A), qp(o.B)
		if dist(o.A, o.B) < 1 {
			bad++
			continue
		}
		if !knownKind[o.Kind] {
			o.Kind = Gap
			odd++
		}
		if o.Leafed && !finite(o.Leaf.X, o.Leaf.Y) {
			o.Leafed = false
		}
		if !o.Leafed {
			o.Leaf = Point{}
		}
		if !finite(o.Weight) {
			o.Weight = 0
		}
		o.Weight = clamp(o.Weight, 0, 1)
		ops = append(ops, opening{Opening: o})
	}
	slices.SortFunc(ops, cmpOpening)
	return ops, bad, odd
}

func cmpOpening(x, y opening) int {
	for _, c := range [...]int{
		cmpPoint(x.A, y.A), cmpPoint(x.B, y.B), cmpString(x.Kind, y.Kind),
		cmpBool(x.HingeA, y.HingeA), cmpBool(x.Leafed, y.Leafed), cmpPoint(x.Leaf, y.Leaf),
		cmpFloat(x.Weight, y.Weight),
	} {
		if c != 0 {
			return c
		}
	}
	return 0
}

func cmpString(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// sinOpening is how far (as a sine) an opening may be drawn off its wall's
// direction: a door marked with its leaf slightly askew is still in the wall.
var sinOpening = math.Sin(5 * math.Pi / 180)

// onLine says the opening's ends both lie within half a thickness (and at
// least a couple of px) of s's line, running the same way.
func onLine(s seg, o Opening) (perp float64, ok bool) {
	ou := canonical(unit(sub(o.B, o.A)))
	if math.Abs(cross(ou, s.u)) > sinOpening {
		return 0, false
	}
	n := s.normal()
	lim := math.Max(s.t/2, minTol) + 1
	da, db := math.Abs(dot(sub(o.A, s.a), n)), math.Abs(dot(sub(o.B, s.a), n))
	return da + db, da <= lim && db <= lim
}

// joinOpenings joins the wall pieces either side of each opening into one
// wall through it, or stretches a single piece over an opening at its end.
// It marks the openings that found a wall.
func joinOpenings(segs []seg, ops []opening) []seg {
	uf := newUnionFind(len(segs))
	extra := map[int][]Point{}
	for k := range ops {
		o := &ops[k]
		left, right := -1, -1
		var leftKey, rightKey [2]float64
		for i, s := range segs {
			perp, ok := onLine(s, o.Opening)
			if !ok {
				continue
			}
			lo, hi := dot(o.A, s.u), dot(o.B, s.u)
			if lo > hi {
				lo, hi = hi, lo
			}
			slo, shi := s.span()
			if slo <= lo+1 && shi >= hi-1 {
				o.matched = true
				break
			}
			reach := max(2*s.t, (hi-lo)/2, 4)
			sm, om := (slo+shi)/2, (lo+hi)/2
			switch {
			case sm < om && shi < hi-1 && shi >= lo-reach:
				if key := [2]float64{-shi, perp}; left < 0 || less2(key, leftKey) {
					left, leftKey = i, key
				}
			case sm > om && slo > lo+1 && slo <= hi+reach:
				if key := [2]float64{slo, perp}; right < 0 || less2(key, rightKey) {
					right, rightKey = i, key
				}
			}
		}
		if o.matched {
			continue
		}
		if left >= 0 && right >= 0 {
			uf.union(left, right)
		}
		for _, i := range [2]int{left, right} {
			if i >= 0 {
				o.matched = true
				extra[i] = append(extra[i], o.A, o.B)
			}
		}
	}
	// extra is keyed by member; combine wants it by group.
	byRoot := map[int][]Point{}
	for i := range segs {
		if ps, ok := extra[i]; ok {
			r := uf.find(i)
			byRoot[r] = append(byRoot[r], ps...)
		}
	}
	return combine(segs, uf, byRoot)
}

func less2(a, b [2]float64) bool { return a[0] < b[0] || (a[0] == b[0] && a[1] < b[1]) }

// sinCorner is the least angle (as a sine) between two walls for one to be
// extended to meet the other; shallower meetings are left as drawn.
var sinCorner = math.Sin(10 * math.Pi / 180)

// closeCorners extends each wall end to the nearest other wall's centreline
// within 1.5 × max(thickness, 3 px), or trims an end that overshoots one by
// as much: front ends often stop a centreline at the face of the wall it
// meets, or run it to the far face. Ends are computed from the walls as they
// were, so the result doesn't depend on the order walls are visited in.
func closeCorners(segs []seg) []seg {
	out := slices.Clone(segs)
	for i, s := range segs {
		for end := range 2 {
			e, d := s.a, Point{X: -s.u.X, Y: -s.u.Y}
			if end == 1 {
				e, d = s.b, s.u
			}
			hit, ok := nearestMeeting(segs, i, e, d)
			if !ok {
				continue
			}
			if end == 0 {
				out[i].a = hit
			} else {
				out[i].b = hit
			}
		}
		if out[i].a == out[i].b {
			out[i] = s
		}
	}
	slices.SortFunc(out, cmpSeg)
	return out
}

// nearestMeeting is where the end e of segs[i], heading d, meets another
// wall's centreline, if one is close enough.
func nearestMeeting(segs []seg, i int, e, d Point) (Point, bool) {
	s := segs[i]
	best, bestS := Point{}, math.Inf(1)
	for j, o := range segs {
		if j == i || o.column() {
			continue
		}
		den := cross(d, o.u)
		if math.Abs(den) < sinCorner {
			continue
		}
		tol := 1.5 * max(s.t, o.t, 3)
		if s.column() {
			// A pier or column reaches only a wall it already touches: a
			// short piece of a wall broken at a junction, not a free-standing
			// pier a thickness away from one.
			tol = minTol
		}
		sHit := cross(sub(o.a, e), o.u) / den
		if math.Abs(sHit) > tol || math.Abs(sHit) >= bestS {
			continue
		}
		h := addScaled(e, d, sHit)
		olo, ohi := o.span()
		th := dot(h, o.u)
		if sHit >= 0 {
			if th < olo-tol || th > ohi+tol {
				continue
			}
		} else if th < olo || th > ohi || -sHit >= s.length()/2 {
			continue
		}
		best, bestS = qp(h), math.Abs(sHit)
	}
	return best, !math.IsInf(bestS, 1)
}
