package floorgraph

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// Label-seeded spaces. A face of the wall graph is a room only if every
// wall end round it meets the next within a tolerance, so one partition
// stopping 5 cm short of the wall it was drawn to meet merges two rooms,
// and one doorway too wide to be closed merges a room into the open floor.
// The spaces are grown on a grid instead: walls (at their thickness),
// glass, voids, the world outside the outline and the openings that are
// doors are drawn into it as barriers, each thickened by spaceSeal, which
// closes any gap narrower than twice that; every region of floor left
// between them is a space. A label names the region it lies in. A region
// is a room when it is walled in and small, else a zone: open floor, named
// by a label in it or "Open plan". The regions are traced, grown back by
// the seal, simplified, and their edges laid on the centrelines of the
// walls they run along, so rooms meet at their walls' centrelines as
// plan.json's rooms do, and a room and its neighbour share the edge.
//
// The faces stay for a floor with nothing to seed from (an image: without
// OCR it has no labels) and when there is no outline to bound the floor.
const (
	// spaceCell is the grid's cell, metres: a wall's face is placed to
	// within half of it before snapping.
	spaceCell = 0.05
	// maxSpaceCells caps the grid (and its memory: a few bytes a cell); a
	// larger floor gets coarser cells.
	maxSpaceCells = 8 << 20
	// spaceSeal is how far each barrier is thickened, metres. It closes
	// the gaps a drawing leaves where walls were meant to meet (a
	// partition short of the façade, a wall end shy of a column), up to
	// 0.4 m: well under the narrowest doorway, so an open doorway still
	// joins the spaces either side.
	spaceSeal = 0.2
	// smallRoomArea is the largest an unlabelled walled region is still a
	// room, m²: a store, a riser, a cupboard, numbered. Bigger and
	// unlabelled, it is floor no room was drawn round (a corridor, a bay
	// of desks) and becomes a zone.
	smallRoomArea = 25.0
	// doorWidth is the widest plain opening, metres, taken to be a door
	// without door evidence: cadfront's maxPlainDoorway. A wider gap with
	// nothing in it is a walkway, and the spaces either side are one.
	doorWidth = 1.0
	// seedReach is how far, metres, a label on a wall's line (in a narrow
	// room, the lettering over its wall) looks for the floor it names.
	seedReach = 0.5
	// snapReach is how far beyond half a wall's thickness, metres, a
	// space's edge may be from the wall's centreline and be laid on it:
	// the traced edge is the wall's face give or take a cell or two.
	snapReach = 0.12
	// doorSeal is how far the barriers are thickened, metres, to find the
	// rooms off a region through gaps no door was found in (pockets): it
	// closes gaps up to 1 m, a door's width and its frame.
	doorSeal = 0.5
	// seedFloor is the least area, m², a region the seal shrank must have
	// to be traced at all when no label is in it: a cupboard 0.8 m square
	// inside its walls keeps about 0.16 m² once shrunk.
	seedFloor = 0.05
)

// sinSnap is how far from parallel, as a sine, an edge may run to a wall
// it is laid on: a traced edge a few cells long is a few degrees off.
var sinSnap = math.Sin(12 * math.Pi / 180)

// spaceGrid is the barrier grid and the regions found on it.
type spaceGrid struct {
	*mask
	// void marks cells inside a void, outside those outside the outline.
	void, outside []bool
	// comp is each open cell's region, -1 for a blocked cell.
	comp []int32
	// size counts each region's cells; box is its cells' bounds.
	size []int
	box  [][4]int // x0, y0, x1, y1, inclusive
	// kept marks the regions to be traced.
	kept []bool
	seal float64 // px
}

// space is a traced region on its way to being a room.
type space struct {
	comp int
	// poly is the region's outline snapped to the walls' centrelines, its
	// holes joined on; raw is as traced, at the walls' faces. outer and
	// rawOuter are the same without the holes, for the region's width.
	poly, raw, outer, rawOuter []Point
	labels                     []int // indices into the labels seeded in it
	areaM, width               float64
}

// labelSpaces makes b.rs from the regions the labels seed. It reports
// false, leaving b.rs empty, when there is no label to seed from or no
// outline to bound the floor; the wall graph's faces are the rooms then.
func (b *builder) labelSpaces(labels []Label, voidFaces map[int]bool) bool {
	named := func(l Label) bool { return cleanLabel(l) != "" }
	if b.outer < 0 || !slices.ContainsFunc(labels, named) {
		return false
	}
	sx, sy := b.sc.sx, b.sc.sy
	pxm := math.Min(sx, sy)
	outline := b.g.poly(b.fs[b.outer].verts)
	walls, doors := b.barriers()
	seal := spaceSeal * pxm
	g := &spaceGrid{mask: newMask(walls, spaceCell*pxm, seal, maxSpaceCells), seal: seal}
	g.void = make([]bool, len(g.blocked))
	// Outside the outline is no floor, however the façade is broken: a
	// region that leaks out through an entrance without a door is still
	// the floor inside it.
	g.outside = make([]bool, len(g.blocked))
	for c := range g.outside {
		g.outside[c] = true
	}
	g.scan(outline, func(c int) { g.outside[c] = false })
	copy(g.blocked, g.outside)
	for f := range voidFaces {
		g.scan(b.g.poly(b.fs[f].verts), func(c int) { g.blocked[c], g.void[c] = true, true })
	}
	for _, s := range walls {
		g.stampWall(s, seal)
	}
	for _, s := range doors {
		g.stamp(s.a, s.b, seal)
	}
	g.comp, g.size, g.box = g.components(g.blocked)

	// Each label seeds the region it lies in.
	at := make([]int, len(labels))
	for li, l := range labels {
		at[li] = -1
		if cleanLabel(l) != "" {
			at[li] = g.seed(l.At, seedReach*pxm)
		}
	}
	g.pockets(walls, doors, doorSeal*pxm, labels, at, openPlanArea*float64(sx*sy))
	seeds := map[int][]int{}
	stray := 0
	for li, c := range at {
		switch {
		case c >= 0:
			seeds[int(g.comp[c])] = append(seeds[int(g.comp[c])], li)
		case cleanLabel(labels[li]) != "":
			stray++
		}
	}
	floor := int(math.Ceil(seedFloor * float64(sx*sy) / float64(g.cell*g.cell)))
	g.kept = make([]bool, len(g.size))
	for c, n := range g.size {
		g.kept[c] = n >= floor || len(seeds[c]) > 0
	}
	tol, chamfer, maxMove := snapReach*pxm, 3*seal+2*g.cell, 1.5*pxm
	var sps []space
	for c := range g.size {
		if !g.kept[c] {
			continue
		}
		outer, holes := g.trace(c)
		if outer == nil {
			stray += len(seeds[c])
			continue
		}
		sp := space{comp: c, rawOuter: outer, raw: bridge(outer, holes), labels: seeds[c]}
		sp.outer = snapRing(outer, walls, tol, chamfer, maxMove, g.cell)
		snapped := make([][]Point, len(holes))
		for i, h := range holes {
			snapped[i] = snapRing(h, walls, tol, chamfer, maxMove, g.cell)
		}
		if sp.poly = bridge(sp.outer, snapped); crosses(sp.poly) {
			sp.poly, sp.outer = sp.raw, sp.rawOuter
		}
		sps = append(sps, sp)
	}
	b.overlapRooms += g.separate(sps, sx, sy)
	kept := sps[:0]
	for _, sp := range sps {
		if sp.poly != nil {
			sp.poly = simplify(sp.poly, 0.25)
		}
		if len(sp.poly) >= 3 {
			sp.areaM, _ = faceSize(sp.poly, sx, sy)
			_, sp.width = faceSize(sp.outer, sx, sy)
		}
		if sp.areaM <= minRoomArea || sp.width < minRoomWidth {
			stray += len(sp.labels)
			continue
		}
		kept = append(kept, sp)
	}
	strip := b.spaceStrips(kept, outline, g.cell/pxm, func(sp space) bool {
		return g.facing(sp.comp, int(math.Ceil(facadeWidth*pxm/g.cell)))
	})
	for i, sp := range kept {
		if strip[i] {
			b.strips++
			continue
		}
		b.rs = append(b.rs, b.spaceRoom(sp, labels))
	}
	b.strayLabels = stray
	b.spaced = true
	return true
}

// stampWall blocks the cells whose centres are within seal of the wall's
// footprint: its length, its thickness across. Not stamp's round ends: a
// wall a few centimetres long and half a metre thick (a column's face, a
// jamb across a thick wall) would block a disc a metre across.
func (m *mask) stampWall(s seg, seal float64) {
	half := s.t / 2
	n, l := s.normal(), s.length()
	steps := max(1, int(math.Ceil(l/m.cell)))
	reach := int(math.Ceil((half+seal)/m.cell)) + 1
	for k := 0; k <= steps; k++ {
		ci, cj := m.index(lerp(s.a, s.b, float64(k)/float64(steps)))
		for j := max(cj-reach, 0); j <= min(cj+reach, m.h-1); j++ {
			for i := max(ci-reach, 0); i <= min(ci+reach, m.w-1); i++ {
				if m.blocked[j*m.w+i] {
					continue
				}
				d := sub(m.centre(i, j), s.a)
				along, across := dot(d, s.u), math.Abs(dot(d, n))
				dx, dy := math.Max(0, math.Max(-along, along-l)), math.Max(0, across-half)
				if math.Hypot(dx, dy) <= seal {
					m.blocked[j*m.w+i] = true
				}
			}
		}
	}
}

// barriers are the lines spaces stop at: every wall and void edge, less
// the walkways through them, and the doors.
func (b *builder) barriers() (walls, doors []seg) {
	pxm := math.Min(b.sc.sx, b.sc.sy)
	var walkways []opening
	for _, o := range b.ops {
		if b.closes(o.Opening, pxm) {
			doors = append(doors, newSeg(o.A, o.B, canonical(unit(sub(o.B, o.A))), -1, 0, 0, false))
		} else {
			walkways = append(walkways, o)
		}
	}
	for _, s := range b.segs {
		// The wall graph joined each wall through its openings; a walkway
		// is cut back out.
		var cuts [][2]float64
		for _, o := range walkways {
			if _, ok := onLine(s, o.Opening); ok {
				lo, hi := dot(o.A, s.u), dot(o.B, s.u)
				cuts = append(cuts, [2]float64{min(lo, hi), max(lo, hi)})
			}
		}
		walls = append(walls, cut(s, cuts)...)
	}
	return walls, doors
}

// closes says an opening is a barrier between spaces: a door or window, an
// opening the front end found door evidence in, or one no wider than a
// door.
func (b *builder) closes(o Opening, pxm float64) bool {
	if o.Kind != Gap || o.Leafed || o.Doorway {
		return true
	}
	return dist(o.A, o.B) <= doorWidth*pxm
}

// cut is s less the stretches along it between each pair of cuts.
func cut(s seg, cuts [][2]float64) []seg {
	if len(cuts) == 0 {
		return []seg{s}
	}
	slices.SortFunc(cuts, func(x, y [2]float64) int { return cmpFloat(x[0], y[0]) })
	lo, hi := s.span()
	var out []seg
	at := lo
	for _, c := range append(cuts, [2]float64{hi, hi}) {
		if c[0] > at {
			p := s
			p.a, p.b = addScaled(s.a, s.u, at-lo), addScaled(s.a, s.u, math.Min(c[0], hi)-lo)
			if dist(p.a, p.b) > 0 {
				out = append(out, p)
			}
		}
		at = math.Max(at, c[1])
		if at >= hi {
			break
		}
	}
	return out
}

// scan calls fn for each cell whose centre is inside poly (even-odd).
func (m *mask) scan(poly []Point, fn func(c int)) {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, p := range poly {
		lo, hi = math.Min(lo, p.Y), math.Max(hi, p.Y)
	}
	j0 := max(0, int(math.Floor((lo-m.y0)/m.cell)))
	j1 := min(m.h-1, int(math.Ceil((hi-m.y0)/m.cell)))
	var xs []float64
	for j := j0; j <= j1; j++ {
		yc := m.y0 + float64(float64(j)+0.5)*m.cell
		xs = xs[:0]
		for i := range poly {
			a, c := poly[i], poly[(i+1)%len(poly)]
			if (a.Y > yc) != (c.Y > yc) {
				xs = append(xs, a.X+float64((yc-a.Y)*(c.X-a.X))/(c.Y-a.Y))
			}
		}
		slices.Sort(xs)
		for k := 0; k+1 < len(xs); k += 2 {
			// Centres in [left, right): two spaces sharing an edge never
			// both claim a cell on it.
			i0 := max(0, int(math.Ceil((xs[k]-m.x0)/m.cell-0.5)))
			i1 := min(m.w-1, int(math.Ceil((xs[k+1]-m.x0)/m.cell-0.5))-1)
			for i := i0; i <= i1; i++ {
				fn(j*m.w + i)
			}
		}
	}
}

// components labels the 4-connected regions of cells not blocked, in scan
// order: each cell's region (-1 when blocked), and each region's cell
// count and bounds.
func (m *mask) components(blocked []bool) (comp []int32, size []int, box [][4]int) {
	comp = make([]int32, len(blocked))
	for c := range comp {
		comp[c] = -1
	}
	var queue []int
	for c0, b := range blocked {
		if b || comp[c0] >= 0 {
			continue
		}
		id := regionID(len(size))
		comp[c0] = id
		queue = append(queue[:0], c0)
		bx := [4]int{c0 % m.w, c0 / m.w, c0 % m.w, c0 / m.w}
		for k := 0; k < len(queue); k++ {
			c := queue[k]
			x, y := c%m.w, c/m.w
			bx = [4]int{min(bx[0], x), min(bx[1], y), max(bx[2], x), max(bx[3], y)}
			for _, n := range m.neighbours(c) {
				if n >= 0 && !blocked[n] && comp[n] < 0 {
					comp[n] = id
					queue = append(queue, n)
				}
			}
		}
		size = append(size, len(queue))
		box = append(box, bx)
	}
	return comp, size, box
}

// pockets splits off the rooms joined to a region by a gap no door was
// found in: an opening narrower than a door with nothing drawn in it, a
// wall stopping short of the one it was meant to meet. The barriers are
// sealed again by seal px, which closes gaps a door wide, and the region's
// cells go to the nearest of the parts that leaves. In open floor (a region
// openPlan px² or more) each part smaller than that, other than the
// largest, becomes a region of its own: a room off it. A smaller region is
// split only between two room names in different parts. at are the
// labels' cells.
func (g *spaceGrid) pockets(
	walls, doors []seg, seal float64, labels []Label, at []int, openPlan float64,
) {
	blocked := make([]bool, len(g.blocked))
	for c := range blocked {
		blocked[c] = g.outside[c] || g.void[c]
	}
	coarse := &mask{cell: g.cell, x0: g.x0, y0: g.y0, w: g.w, h: g.h, blocked: blocked}
	for _, s := range walls {
		coarse.stampWall(s, seal)
	}
	for _, s := range doors {
		coarse.stamp(s.a, s.b, seal)
	}
	part, _, _ := coarse.components(blocked)
	cellArea := float64(g.cell * g.cell)
	minCells := int(math.Ceil(minRoomArea * openPlan / openPlanArea / cellArea))
	door := int(math.Ceil(2 * seal / g.cell))
	regions := len(g.size)
	for c := range regions {
		if g.size[c] < 2*minCells {
			continue
		}
		owner, queue := g.nearestPart(c, part)
		sizes := map[int32]int{}
		var ids []int32
		for _, i := range queue {
			if sizes[owner[i]] == 0 {
				ids = append(ids, owner[i])
			}
			sizes[owner[i]]++
		}
		if len(ids) < 2 {
			continue
		}
		slices.SortStableFunc(ids, func(x, y int32) int { return sizes[y] - sizes[x] })
		// A part meets the rest across its doorway: a part that meets it
		// along more is a stretch of the region the seal pinched off where
		// lines crowd (round a stair, along a run of risers), not a room.
		meets := map[int32]int{}
		for _, i := range queue {
			for _, n := range g.neighbours(i) {
				if o, ok := owner[n]; ok && o != owner[i] {
					meets[owner[i]]++
				}
			}
		}
		open := float64(g.size[c])*cellArea >= openPlan
		split := open || namesApart(owner, labels, at)
		newID := map[int32]int32{}
		for _, id := range ids[1:] {
			small := !open || float64(sizes[id])*cellArea < openPlan
			if split && small && sizes[id] >= minCells && meets[id] <= 2*door {
				newID[id] = regionID(len(g.size))
				g.size = append(g.size, 0)
				g.box = append(g.box, [4]int{g.w, g.h, -1, -1})
			}
		}
		for _, i := range queue {
			id, ok := newID[owner[i]]
			if !ok {
				continue
			}
			g.comp[i] = id
			g.size[id]++
			g.size[c]--
			x, y := i%g.w, i/g.w
			b := &g.box[id]
			*b = [4]int{min(b[0], x), min(b[1], y), max(b[2], x), max(b[3], y)}
		}
	}
}

// nearestPart gives each cell of region c the part of it (a region of
// part) nearest: the cells in a part keep it, and the rest are reached
// breadth first from them. queue lists the cells given one, in that order.
func (g *spaceGrid) nearestPart(c int, part []int32) (owner map[int]int32, queue []int) {
	owner = map[int]int32{}
	id := regionID(c)
	bx := g.box[c]
	for y := bx[1]; y <= bx[3]; y++ {
		for x := bx[0]; x <= bx[2]; x++ {
			if i := y*g.w + x; g.comp[i] == id && part[i] >= 0 {
				owner[i] = part[i]
				queue = append(queue, i)
			}
		}
	}
	for k := 0; k < len(queue); k++ {
		for _, n := range g.neighbours(queue[k]) {
			if _, ok := owner[n]; n >= 0 && !ok && g.comp[n] == id {
				owner[n] = owner[queue[k]]
				queue = append(queue, n)
			}
		}
	}
	return owner, queue
}

// namesApart reports whether two different room names (not numbers) lie
// in different parts.
func namesApart(owner map[int]int32, labels []Label, at []int) bool {
	names := map[int32]string{}
	for li, i := range at {
		o, ok := owner[i]
		if !ok || i < 0 || labels[li].Kind == LabelNumber {
			continue
		}
		for id, n := range names {
			if id != o && n != cleanLabel(labels[li]) {
				return true
			}
		}
		names[o] = cleanLabel(labels[li])
	}
	return false
}

// regionID is a region's index as stored in spaceGrid.comp. There are
// fewer regions than cells, and cells are capped by maxSpaceCells.
func regionID(i int) int32 { return int32(i) } //nolint:gosec // under maxSpaceCells

// neighbours are a cell's four, -1 past the grid's edge.
func (m *mask) neighbours(c int) [4]int {
	x, y := c%m.w, c/m.w
	n := [4]int{-1, -1, -1, -1}
	if x > 0 {
		n[0] = c - 1
	}
	if x < m.w-1 {
		n[1] = c + 1
	}
	if y > 0 {
		n[2] = c - m.w
	}
	if y < m.h-1 {
		n[3] = c + m.w
	}
	return n
}

// seed is the open cell a label at p lies in, else the nearest within
// reach px (the first in scan order among equals), or -1.
func (g *spaceGrid) seed(p Point, reach float64) int {
	i, j := g.index(p)
	if i < 0 || j < 0 || i >= g.w || j >= g.h {
		return -1
	}
	if g.comp[j*g.w+i] >= 0 {
		return j*g.w + i
	}
	k := int(math.Ceil(reach / g.cell))
	best, bestD := -1, k*k+1
	for y := max(j-k, 0); y <= min(j+k, g.h-1); y++ {
		for x := max(i-k, 0); x <= min(i+k, g.w-1); x++ {
			if d := (x-i)*(x-i) + (y-j)*(y-j); d < bestD && g.comp[y*g.w+x] >= 0 {
				best, bestD = y*g.w+x, d
			}
		}
	}
	return best
}

// trace outlines region c, grown back by the seal to the barriers'
// faces, and simplified: its outer boundary and the boundaries of the
// holes in it that hold another space or a void (plan.json's rooms have
// no holes; bridge joins them on). Holes that hold neither (a column, a
// free-standing wall) are filled: a zone may cover their footprint. outer
// is nil when nothing is left.
func (g *spaceGrid) trace(c int) (outer []Point, holes [][]Point) {
	k := int(math.Ceil(g.seal/g.cell)) + 2
	bx := g.box[c]
	x0, y0 := bx[0]-k, bx[1]-k
	s := &mask{
		cell: g.cell, w: bx[2] - bx[0] + 1 + 2*k, h: bx[3] - bx[1] + 1 + 2*k,
		x0: g.x0 + float64(float64(x0)*g.cell), y0: g.y0 + float64(float64(y0)*g.cell),
	}
	in := make([]bool, s.w*s.h)
	// global maps a sub-grid cell to the grid's, or -1 off it.
	global := func(l int) int {
		x, y := l%s.w+x0, l/s.w+y0
		if x < 0 || y < 0 || x >= g.w || y >= g.h {
			return -1
		}
		return y*g.w + x
	}
	for l := range in {
		if gc := global(l); gc >= 0 && g.comp[gc] == regionID(c) {
			in[l] = true
		}
	}
	s.grow(in, g.seal)
	// Where a region was split from another (pockets) they meet with no
	// barrier between, and neither grows into the other.
	for l := range in {
		if gc := global(l); in[l] && gc >= 0 && g.comp[gc] >= 0 && g.comp[gc] != regionID(c) {
			in[l] = false
		}
	}
	for _, l := range g.holes(s, in, c, global) {
		// l is the hole's first cell in scan order, so the cell above it
		// is the region's and the walk round the hole starts along its
		// top, heading left with the region on its right.
		if h := rdpRing(s.walk(in, l%s.w+1, l/s.w, 2), 1.5*g.cell); h != nil {
			holes = append(holes, h)
		}
	}
	return rdpRing(s.outline(in), 1.5*g.cell), holes
}

// holes finds the holes in a region: the groups of cells outside it, joined
// at edges or corners (the region's own cells join only at edges, as walk
// treats them), that don't reach the sub-grid's edge. It fills those that
// hold no other kept region and no void, and returns the first cell, in
// scan order, of each of the others.
func (g *spaceGrid) holes(s *mask, in []bool, c int, global func(int) int) []int {
	seen := make([]bool, len(in))
	flood := func(queue []int) []int {
		for k := 0; k < len(queue); k++ {
			x, y := queue[k]%s.w, queue[k]/s.w
			for j := max(y-1, 0); j <= min(y+1, s.h-1); j++ {
				for i := max(x-1, 0); i <= min(x+1, s.w-1); i++ {
					if n := j*s.w + i; !in[n] && !seen[n] {
						seen[n] = true
						queue = append(queue, n)
					}
				}
			}
		}
		return queue
	}
	var edge []int
	for l := range in {
		x, y := l%s.w, l/s.w
		if !in[l] && (x == 0 || y == 0 || x == s.w-1 || y == s.h-1) {
			seen[l] = true
			edge = append(edge, l)
		}
	}
	flood(edge)
	var out []int
	for l := range in {
		if in[l] || seen[l] {
			continue
		}
		seen[l] = true
		hole := flood([]int{l})
		busy := func(h int) bool {
			gc := global(h)
			if gc < 0 {
				return false
			}
			other := g.comp[gc]
			return g.void[gc] || other >= 0 && int(other) != c && g.kept[other]
		}
		if slices.ContainsFunc(hole, busy) {
			out = append(out, l)
			continue
		}
		for _, h := range hole {
			in[h] = true
		}
	}
	return out
}

// bridge joins each hole onto the ring round it by a cut of no width: up
// from the hole's topmost corner to the nearest edge above it, round the
// hole and back, the way the wall graph bridges a core (graph.bridge). The
// holes go topmost first, so the edge above each is the outer ring's or
// that of a hole already joined. Rings and holes run with the region on
// their right, so the joined ring does too.
func bridge(ring []Point, holes [][]Point) []Point {
	out := slices.Clone(ring)
	order := make([]int, len(holes))
	tops := make([]int, len(holes))
	for i, h := range holes {
		order[i] = i
		for k, p := range h {
			if p.Y < h[tops[i]].Y || p.Y == h[tops[i]].Y && p.X < h[tops[i]].X {
				tops[i] = k
			}
		}
	}
	slices.SortStableFunc(order, func(i, j int) int {
		return cmpPoint(Point{X: holes[i][tops[i]].Y, Y: holes[i][tops[i]].X},
			Point{X: holes[j][tops[j]].Y, Y: holes[j][tops[j]].X})
	})
	for _, i := range order {
		h, top := holes[i], holes[i][tops[i]]
		at, bestY := -1, math.Inf(-1)
		for e := range out {
			p, q := out[e], out[(e+1)%len(out)]
			if p.X == q.X || top.X < math.Min(p.X, q.X) || top.X > math.Max(p.X, q.X) {
				continue
			}
			if y := p.Y + float64((top.X-p.X)*(q.Y-p.Y))/(q.X-p.X); y < top.Y && y > bestY {
				at, bestY = e, y
			}
		}
		if at < 0 {
			continue
		}
		x := qp(Point{X: top.X, Y: bestY})
		loop := slices.Concat([]Point{x}, h[tops[i]:], h[:tops[i]+1], []Point{x})
		out = slices.Insert(out, at+1, loop...)
	}
	return out
}

// snapRing lays each edge of a traced outline that runs along a wall, its
// ends within half the wall's thickness and tol of the centreline and over
// half of it beside the wall, on that centreline. A corner between two
// edges laid on walls goes where the walls' lines meet, if that is within
// maxMove of it; else (parallel walls, or a sliver of an angle) it goes
// on both lines with a step between. A corner beside one such edge is
// moved straight across onto its wall: where the other edge's line meets
// the wall's at a shallow angle, that point can be far along it. Growing
// the region back rounded its corners: the short edges, under chamfer in
// all, cutting a corner between two edges laid on walls go, and the
// walls' lines meet. An edge whose snapping makes the outline cross itself
// is left where it was traced, and the outline as traced is returned if
// that doesn't mend it or its area changes by over half.
func snapRing(ring []Point, walls []seg, tol, chamfer, maxMove, spike float64) []Point {
	along := make([]*seg, len(ring))
	for i := range ring {
		p, q := ring[i], ring[(i+1)%len(ring)]
		if s, ok := alongWall(p, q, unit(sub(q, p)), walls, tol); ok {
			along[i] = &s
		}
	}
	free := make([]bool, len(ring))
	for range 16 {
		out, from := snapOnce(ring, along, free, chamfer, maxMove)
		i, j, crossed := crossing(out)
		if !crossed {
			out = simplify(despike(out, spike), 0.5)
			a0, a1 := math.Abs(signedArea(ring)), math.Abs(signedArea(out))
			if len(out) < 3 || a1 < a0/2 || a1 > 2*a0 {
				break
			}
			return out
		}
		freed := false
		n := len(out)
		for _, e := range [...]int{i, j, (i + n - 1) % n, (j + n - 1) % n} {
			if f := from[e]; f >= 0 && !free[f] && along[f] != nil {
				free[f], freed = true, true
			}
		}
		if !freed {
			break
		}
	}
	return ring
}

// snapOnce is one pass of snapRing, the raw edges marked free left where
// they were traced. from says which raw edge each edge of the result lies
// on, -1 for a step between two walls.
func snapOnce(
	ring []Point, along []*seg, free []bool, chamfer, maxMove float64,
) (out []Point, from []int) {
	type line struct {
		o, u    Point
		snapped bool
		raw     int
	}
	lines := make([]line, len(ring))
	for i := range ring {
		p, q := ring[i], ring[(i+1)%len(ring)]
		u := unit(sub(q, p))
		lines[i] = line{o: p, u: u, raw: i}
		if s := along[i]; s != nil && !free[i] {
			su := s.u
			if dot(su, u) < 0 {
				su = Point{X: -su.X, Y: -su.Y}
			}
			lines[i] = line{o: s.a, u: su, snapped: true, raw: i}
		}
	}
	meet := func(a, c line, v Point) (Point, bool) {
		x := cross(a.u, c.u)
		if math.Abs(x) <= sinSnap/2 {
			return Point{}, false
		}
		p := addScaled(a.o, a.u, cross(sub(c.o, a.o), c.u)/x)
		return p, dist(p, v) <= maxMove
	}
	pts := slices.Clone(ring)
	for cut := true; cut; {
		cut = false
		n := len(lines)
		for i := 0; i < n && !cut; i++ {
			if !lines[i].snapped {
				continue
			}
			k, run := 0, 0.0
			for k < n-3 && !lines[(i+1+k)%n].snapped {
				e := (i + 1 + k) % n
				run += dist(pts[e], pts[(e+1)%n])
				k++
			}
			next := (i + 1 + k) % n
			if k == 0 || run > chamfer || !lines[next].snapped {
				continue
			}
			if _, ok := meet(lines[i], lines[next], pts[next]); !ok {
				continue
			}
			for range k {
				e := (i + 1) % len(lines)
				lines, pts = slices.Delete(lines, e, e+1), slices.Delete(pts, e, e+1)
				if e < i {
					i--
				}
			}
			cut = true
		}
	}
	project := func(v Point, l line) Point { return addScaled(l.o, l.u, dot(sub(v, l.o), l.u)) }
	for i, v := range pts {
		a, c := lines[(i+len(lines)-1)%len(lines)], lines[i]
		switch {
		case a.snapped && c.snapped:
			if p, ok := meet(a, c, v); ok {
				out, from = append(out, qp(p)), append(from, c.raw)
			} else {
				out = append(out, qp(project(v, a)), qp(project(v, c)))
				from = append(from, -1, c.raw)
			}
		case a.snapped:
			out, from = append(out, qp(project(v, a))), append(from, c.raw)
		case c.snapped:
			out, from = append(out, qp(project(v, c))), append(from, c.raw)
		default:
			out, from = append(out, v), append(from, c.raw)
		}
	}
	return out, from
}

// crossing finds two edges of a ring that cross, not just touch: their
// indices, edge i running from vertex i.
func crossing(ring []Point) (i, j int, ok bool) {
	n := len(ring)
	for i := range n {
		a, b := ring[i], ring[(i+1)%n]
		for j := i + 2; j < n; j++ {
			if i == 0 && j == n-1 {
				continue
			}
			c, d := ring[j], ring[(j+1)%n]
			d1, d2 := cross(sub(b, a), sub(c, a)), cross(sub(b, a), sub(d, a))
			d3, d4 := cross(sub(d, c), sub(a, c)), cross(sub(d, c), sub(b, c))
			if d1*d2 < 0 && d3*d4 < 0 {
				return i, j, true
			}
		}
	}
	return 0, 0, false
}

// crosses reports whether two edges of a ring cross.
func crosses(ring []Point) bool {
	_, _, ok := crossing(ring)
	return ok
}

// despike drops the vertices where a ring doubles back on itself, the
// way back within tol px of the way out: snapped onto the centreline of a
// partition that stops in the room (a WC cubicle's), the inlet the region
// had round it closes to nothing, and a room is an area.
func despike(ring []Point, tol float64) []Point {
	out := slices.Clone(ring)
	for changed := true; changed && len(out) >= 3; {
		changed = false
		for i := 0; i < len(out) && len(out) >= 3; i++ {
			p, c, n := out[(i+len(out)-1)%len(out)], out[i], out[(i+1)%len(out)]
			d1, d2 := sub(c, p), sub(n, c)
			if c == n || dot(d1, d2) < 0 && (segDist(n, p, c) <= tol || segDist(p, c, n) <= tol) {
				out = slices.Delete(out, i, i+1)
				changed = true
				i--
			}
		}
	}
	return out
}

// alongWall is the wall an edge p–q (direction u) runs along: within
// sinSnap of parallel, its ends within half the wall's thickness and tol
// of the centreline, and over half of it beside the wall. Of several, the
// one whose face the edge is on, rather than one beyond it.
func alongWall(p, q, u Point, walls []seg, tol float64) (seg, bool) {
	best, bestD := -1, math.Inf(1)
	for k, s := range walls {
		if math.Abs(cross(u, s.u)) > sinSnap {
			continue
		}
		nrm := s.normal()
		dp, dq := dot(sub(p, s.a), nrm), dot(sub(q, s.a), nrm)
		if math.Abs(dp) > s.t/2+tol || math.Abs(dq) > s.t/2+tol {
			continue
		}
		lo, hi := s.span()
		a, c := dot(p, s.u), dot(q, s.u)
		if a > c {
			a, c = c, a
		}
		if math.Min(hi+s.t/2, c)-math.Max(lo-s.t/2, a) < (c-a)/2 {
			continue
		}
		if d := math.Abs(math.Abs(dp+dq)/2 - s.t/2); d < bestD {
			best, bestD = k, d
		}
	}
	if best < 0 {
		return seg{}, false
	}
	return walls[best], true
}

// separate makes the spaces disjoint, as plan.json requires, should
// snapping have laid two over each other: where two share more than a
// tenth of a square metre of cells, both go back to their outlines as
// traced (at the walls' faces, which can't overlap); where they still do,
// the smaller is dropped (poly set to nil). It returns how many were.
func (g *spaceGrid) separate(sps []space, sx, sy float64) int {
	limit := int(0.1 * float64(sx*sy) / float64(g.cell*g.cell))
	overlaps := func() map[[2]int]int {
		owner := make([]int32, len(g.blocked))
		for c := range owner {
			owner[c] = -1
		}
		shared := map[[2]int]int{}
		for i, sp := range sps {
			if sp.poly == nil {
				continue
			}
			g.scan(sp.poly, func(c int) {
				if o := owner[c]; o >= 0 {
					shared[[2]int{int(o), i}]++
				} else {
					owner[c] = regionID(i)
				}
			})
		}
		return shared
	}
	for pair, n := range overlaps() {
		if n > limit {
			for _, i := range pair {
				sps[i].poly, sps[i].outer = sps[i].raw, sps[i].rawOuter
			}
		}
	}
	dropped := 0
	shared := overlaps()
	pairs := make([][2]int, 0, len(shared))
	for pair := range shared {
		pairs = append(pairs, pair)
	}
	slices.SortFunc(pairs, func(x, y [2]int) int {
		if x[0] != y[0] {
			return x[0] - y[0]
		}
		return x[1] - y[1]
	})
	for _, pair := range pairs {
		x, y := &sps[pair[0]], &sps[pair[1]]
		if shared[pair] <= limit || x.poly == nil || y.poly == nil {
			continue
		}
		if math.Abs(signedArea(x.poly)) < math.Abs(signedArea(y.poly)) {
			x, y = y, x
		}
		y.poly = nil
		dropped++
	}
	return dropped
}

// minLadder is how many narrow, bare spaces side by side are façade even
// with no sign of the outside beyond them.
const minLadder = 3

// spaceStrips finds the unlabelled spaces that are part of the façade's
// construction, not rooms, as facadeStrips does among faces: narrower than
// facadeWidth, along the outline, with no door into them, in a ladder of
// several or one long one; and those outboard of them, a planter or a
// louvre box under twice as wide, half their boundary on the outline or
// the strips.
func (b *builder) spaceStrips(
	sps []space,
	outline []Point,
	widthSlack float64,
	facing func(space) bool,
) map[int]bool {
	f := stripFinder{
		sps: sps, outline: outline, facing: facing,
		reach:      doorReach * math.Min(b.sc.sx, b.sc.sy),
		bare:       make([]bool, len(sps)),
		width:      make([]float64, len(sps)),
		widthSlack: widthSlack,
		strip:      map[int]bool{},
	}
	for i, sp := range sps {
		f.width[i] = sp.width
		if len(sp.rawOuter) >= 3 {
			// The façade limit describes usable floor between wall faces,
			// before snapping the outline to their centrelines.
			_, f.width[i] = faceSize(sp.rawOuter, b.sc.sx, b.sc.sy)
		}
		f.bare[i] = len(sp.labels) == 0 && !slices.ContainsFunc(b.ops, func(o opening) bool {
			return o.Kind != Window && math.Abs(boundaryDist(mid(o.A, o.B), sp.poly)) <= f.reach
		})
	}
	f.ladders()
	for f.outboard() {
	}
	return f.strip
}

// stripFinder is spaceStrips' state: the spaces, which are bare (no label,
// no door), and which are strips so far.
type stripFinder struct {
	sps        []space
	outline    []Point
	facing     func(space) bool
	reach      float64
	bare       []bool
	width      []float64
	widthSlack float64
	strip      map[int]bool
}

func (f *stripFinder) near(p Point, poly []Point) bool {
	return math.Abs(boundaryDist(p, poly)) <= f.reach
}

// touch reports whether two spaces' boundaries come within reach.
func (f *stripFinder) touch(i, j int) bool {
	x, y := f.sps[i].poly, f.sps[j].poly
	return slices.ContainsFunc(x, func(p Point) bool { return f.near(p, y) }) ||
		slices.ContainsFunc(y, func(p Point) bool { return f.near(p, x) })
}

// ladders marks the groups of narrow bare spaces side by side that are
// façade.
func (f *stripFinder) ladders() {
	// A grid outline's two faces each have half a cell of uncertainty.
	narrow := func(i int) bool { return f.bare[i] && f.width[i] < facadeWidth+f.widthSlack }
	seen := map[int]bool{}
	for i := range f.sps {
		if seen[i] || !narrow(i) {
			continue
		}
		seen[i] = true
		group := []int{i}
		longest, faces := 0.0, false
		for k := 0; k < len(group); k++ {
			sp := f.sps[group[k]]
			longest = math.Max(longest, sp.areaM/sp.width)
			faces = faces || f.facing(sp)
			for j := range f.sps {
				if !seen[j] && narrow(j) && f.touch(group[k], j) {
					seen[j] = true
					group = append(group, j)
				}
			}
		}
		// A ladder of three is façade wherever it stands: a terrace or a
		// courtyard inside the outline can lie between it and the world
		// outside.
		if faces && (len(group) >= 2 || longest >= loneStrip) || len(group) >= minLadder {
			for _, j := range group {
				f.strip[j] = true
			}
		}
	}
}

// outside is the share of space i's boundary on the outline or a strip.
func (f *stripFinder) outside(i int) float64 {
	poly := f.sps[i].poly
	on, all := 0.0, 0.0
	for k := range poly {
		p, q := poly[k], poly[(k+1)%len(poly)]
		l := dist(p, q)
		all += l
		m := mid(p, q)
		onEdge := f.near(m, f.outline)
		for j := range f.sps {
			onEdge = onEdge || f.strip[j] && f.near(m, f.sps[j].poly)
		}
		if onEdge {
			on += l
		}
	}
	return on / all
}

// outboard marks a group of bare spaces, each under twice facadeWidth wide
// and facing the outside or with half its boundary on the outline or the
// strips, that touches no other space, and reports whether it marked any.
// A store in a corner of the building touches the floor it opens off.
func (f *stripFinder) outboard() bool {
	changed := false
	cand := map[int]bool{}
	for i := range f.sps {
		cand[i] = !f.strip[i] && f.bare[i] && f.width[i] < 2*facadeWidth &&
			(f.outside(i) >= outboard || f.facing(f.sps[i]))
	}
	seen := map[int]bool{}
	for i := range f.sps {
		if !cand[i] || seen[i] {
			continue
		}
		seen[i] = true
		group, alone := []int{i}, true
		for k := 0; k < len(group); k++ {
			for j := range f.sps {
				if j == group[k] || f.strip[j] || !f.touch(group[k], j) {
					continue
				}
				if !cand[j] {
					alone = false
				} else if !seen[j] {
					seen[j] = true
					group = append(group, j)
				}
			}
		}
		if alone {
			for _, j := range group {
				f.strip[j], changed = true, true
			}
		}
	}
	return changed
}

// facing says region c faces the world outside the outline across nothing
// but barriers within steps cells of it: across the façade's construction,
// whose cavities the seal closed, but no other space.
func (g *spaceGrid) facing(c, steps int) bool {
	dist := map[int]int{}
	var queue []int
	bx := g.box[c]
	for y := bx[1]; y <= bx[3]; y++ {
		for x := bx[0]; x <= bx[2]; x++ {
			if i := y*g.w + x; g.comp[i] == regionID(c) {
				dist[i] = 0
				queue = append(queue, i)
			}
		}
	}
	for k := 0; k < len(queue); k++ {
		i := queue[k]
		if g.outside[i] {
			return true
		}
		if dist[i] >= steps {
			continue
		}
		for _, n := range g.neighbours(i) {
			if _, ok := dist[n]; n >= 0 && !ok && g.blocked[n] && !g.void[n] {
				dist[n] = dist[i] + 1
				queue = append(queue, n)
			}
		}
	}
	return false
}

// spaceRoom makes a traced space a room named by the best label in it.
func (b *builder) spaceRoom(sp space, labels []Label) room {
	r := room{poly: sp.poly, areaM: sp.areaM, c: centroid(sp.poly)}
	numbers := map[string]bool{}
	for _, li := range sp.labels {
		if labels[li].Kind == LabelNumber {
			numbers[cleanLabel(labels[li])] = true
		}
	}
	best := -1
	for _, li := range sp.labels {
		l, text := labels[li], cleanLabel(labels[li])
		if !fitsRoom(text, l.Kind, sp.areaM, len(numbers)) {
			continue
		}
		if best < 0 || betterLabel(l, labels[best], text, cleanLabel(labels[best])) {
			best = li
		}
	}
	// A labelled space is a room drawn as one even when no label fits it.
	r.seeded = len(sp.labels) > 0
	if best < 0 {
		return r
	}
	l := labels[best]
	r.name, r.labeled = cleanLabel(l), true
	if r.label = qp(l.At); !inside(r.label, r.poly) {
		r.label = qp(pole(r.poly, 1))
	}
	// A room walled in with two names in it: the wall between them isn't
	// drawn, or is broken more than the seal closes. The room is kept
	// whole, as inventing the wall would be a guess.
	if sp.areaM < openPlanArea {
		var others []string
		for _, li := range sp.labels {
			o := labels[li]
			if text := cleanLabel(o); o.Kind == l.Kind && o.Kind != LabelNumber && text != r.name &&
				!slices.Contains(others, text) {
				others = append(others, text)
			}
		}
		if len(others) > 0 {
			slices.Sort(others)
			b.shared = append(b.shared, fmt.Sprintf(
				"%q also holds the label %s, with no wall between them: it was kept as one room; "+
					"split it in review.", r.name, quoteAll(others)))
		}
	}
	return r
}

func quoteAll(ss []string) string {
	q := make([]string, len(ss))
	for i, s := range ss {
		q[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(q, ", ")
}
