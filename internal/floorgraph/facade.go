package floorgraph

import (
	"math"
	"slices"
)

// facadeWidth is the narrowest a room along the façade can be, metres,
// centreline to centreline. A curtain wall's inner and outer lines, a
// brise-soleil or a cill run 0.3–1 m apart, with mullions across, and make a
// ladder of faces along the building's edge; the narrowest real room there,
// a corridor, is 1.2 m clear.
const facadeWidth = 1.2

// faceSides says, for every half-edge, the face it bounds.
func faceSides(fs []face, halves int) []int {
	side := make([]int, halves)
	for i := range side {
		side[i] = -1
	}
	for i, f := range fs {
		for _, h := range f.half {
			side[h] = i
		}
	}
	return side
}

// loneStrip is the shortest, metres, a narrow face along the façade with no
// narrow neighbours must be to be façade: a ladder of cells is façade
// however short its rungs, but a lone short cell, doorless and unnamed, is
// as likely a riser or a duct cupboard drawn without its door.
const loneStrip = 5.0

// outboard is the least share of its boundary a face must have on the
// building's outside to be outboard of the façade; it is also under twice
// facadeWidth wide.
const outboard = 0.5

// doorReach is how far from a face's edge, metres, a door is taken to be
// in it: openings sit on wall centrelines, give or take half a wall.
const doorReach = 0.3

// facadeStrips finds the faces that are part of the façade's construction,
// not rooms: those narrower than facadeWidth along the outside with no door
// into them and no label in them, and the narrow, doorless, unlabelled
// faces next to them in turn (a façade two cells deep), when there are
// several or one is long (loneStrip). A WC or a riser can be as narrow, but
// has a door or a name; a cell between façade lines has neither, and a
// drawing without doors (a CAFM area plan) still names its areas. skip
// marks faces already spoken for (voids).
//
// The strips are left unfloored and their lines stay walls, as drawn, of
// the kinds they were: made exterior, the many thin façade lines would
// pull the exterior walls' default thickness down to theirs.
//
// Only faces that would be rooms are strips. The search passes through the
// cavities too narrow to be rooms (a cill, a wall drawn as two lines) to
// reach a strip behind one.
func (b *builder) facadeStrips(side []int, skip map[int]bool) map[int]bool {
	f := &facadeScan{
		b: b, side: side, skip: skip,
		reach: doorReach * math.Min(b.sc.sx, b.sc.sy),
		seen:  map[int]bool{}, facade: map[int]bool{}, off: map[int]bool{}, strip: map[int]bool{},
	}
	if b.outer < 0 {
		return f.strip
	}
	for i := range b.fs {
		if i != b.outer && b.fs[i].area > 0 && f.outside(i) > 0 && f.narrow(i) {
			f.ladder(i)
		}
	}
	f.outboard()
	return f.strip
}

// facadeScan is facadeStrips' working state.
type facadeScan struct {
	b      *builder
	side   []int
	skip   map[int]bool
	reach  float64      // doorReach in px
	seen   map[int]bool // faces already in a group
	facade map[int]bool // every face of a façade group, rooms or not
	off    map[int]bool // faces outboard of the façade
	strip  map[int]bool
}

// bare says a face could be façade: no door into it, no label in it.
func (f *facadeScan) bare(i int) bool {
	b := f.b
	if i == b.outer || b.fs[i].area <= 0 || f.skip[i] {
		return false
	}
	poly := b.g.poly(b.fs[i].verts)
	return !slices.ContainsFunc(b.ops, func(o opening) bool {
		return o.Kind != Window && math.Abs(boundaryDist(mid(o.A, o.B), poly)) <= f.reach
	}) && !slices.ContainsFunc(b.in.Labels, func(l Label) bool {
		return cleanLabel(l) != "" && inside(l.At, poly)
	})
}

func (f *facadeScan) narrow(i int) bool {
	if f.seen[i] || !f.bare(i) {
		return false
	}
	_, w := faceSize(simplify(f.b.g.poly(f.b.fs[i].verts), 0.25), f.b.sc.sx, f.b.sc.sy)
	return w < facadeWidth
}

// outside is the share of a face's boundary on the outer face, or on faces
// outboard of the façade.
func (f *facadeScan) outside(i int) float64 {
	b := f.b
	on, all := 0.0, 0.0
	for _, h := range b.fs[i].half {
		l := b.g.length(h / 2)
		all += l
		if j := f.side[h^1]; !b.g.edges[h/2].virtual && (j == b.outer || f.off[j]) {
			on += l
		}
	}
	return on / all
}

// ladder gathers the narrow faces joined to face i and makes those that
// would be rooms strips, when there are several or one is long.
func (f *facadeScan) ladder(i int) {
	b := f.b
	f.seen[i] = true
	group := []int{i}
	var rooms []int
	longest := 0.0
	for k := 0; k < len(group); k++ {
		g := group[k]
		if r, ok := faceRoom(b.g.poly(b.fs[g].verts), b.sc.sx, b.sc.sy); ok {
			rooms = append(rooms, g)
			_, w := faceSize(r.poly, b.sc.sx, b.sc.sy)
			longest = math.Max(longest, r.areaM/w)
		}
		for _, h := range b.fs[g].half {
			if j := f.side[h^1]; j >= 0 && !b.g.edges[h/2].virtual && f.narrow(j) {
				f.seen[j] = true
				group = append(group, j)
			}
		}
	}
	if len(rooms) < 2 && longest < loneStrip {
		return
	}
	for _, g := range group {
		f.facade[g] = true
	}
	for _, g := range rooms {
		f.strip[g] = true
	}
}

// outboard finds the faces outboard of the façade: a shallow bare face
// mostly outside, against the façade's strips, is a planter, a louvre box
// or a column casing on the face of the building; and so is one against
// those in turn, mostly outside once they count as outside.
func (f *facadeScan) outboard() {
	b := f.b
	against := func(h int) bool { return f.facade[f.side[h^1]] || f.off[f.side[h^1]] }
	for changed := true; changed; {
		changed = false
		for i := range b.fs {
			if f.facade[i] || f.off[i] || !f.bare(i) || f.outside(i) < outboard ||
				!slices.ContainsFunc(b.fs[i].half, against) {
				continue
			}
			r, ok := faceRoom(b.g.poly(b.fs[i].verts), b.sc.sx, b.sc.sy)
			if _, w := faceSize(r.poly, b.sc.sx, b.sc.sy); ok && w < 2*facadeWidth {
				f.strip[i], f.off[i], changed = true, true, true
			}
		}
	}
}
