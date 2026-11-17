package floorgraph

import (
	"math"
	"slices"
)

// Finding a void. A void's edge is drawn as loose linework, and loose
// linework is rarely clean: a slab edge, a balustrade's two faces and a
// handrail run side by side, each broken where it meets a stair or a lift,
// with ends a few centimetres short of the next line. A planar graph of
// those lines leaks through the first such gap and finds the whole floor.
// So the void is found on a grid instead: the walls (and their doors) and
// the edges are drawn thickened by voidSeal, which closes any gap narrower
// than twice that, and the void is the area flooded from its point. Its
// outline, traced and simplified, goes into the wall graph as segments
// that bound faces without being walls.
const (
	// voidSeal is how far each line is thickened, metres.
	voidSeal = 0.15
	// voidCell is the grid's cell, metres: fine enough that the traced
	// outline is within a few centimetres of the drawing.
	voidCell = 0.05
	// maxVoidCells caps the grid; a bigger drawing gets coarser cells.
	maxVoidCells = 16 << 20
	// maxVoidShare is the largest share of the building (the walls' convex
	// hull) a void may take. An edge with a gap wider than the seal floods
	// the floor round it: a leak, not a void, and the floor stays.
	maxVoidShare = 0.5
	// voidLabelReach is how far, in metres, a room-name label may be from
	// a void's point and still be the void's own label.
	voidLabelReach = 1.0
)

// voidEdges finds each void's outline and returns it as void segments. It
// records the points whose void was found in b.voidAt and counts the
// others in b.voidMissed. The seal is a size, so voids need the front
// end's scale; the CAD front end, the only one that finds them, has it.
func (b *builder) voidEdges(walls []seg) []seg {
	var at []Point
	for _, p := range b.in.Voids {
		if finite(p.X, p.Y) && math.Abs(p.X) <= maxCoord && math.Abs(p.Y) <= maxCoord {
			at = append(at, qp(p))
		}
	}
	slices.SortFunc(at, cmpPoint)
	at = slices.Compact(at)
	if len(at) == 0 {
		return nil
	}
	if !b.in.Scale.Known() {
		b.voidMissed += len(at)
		return nil
	}
	pxm := math.Min(b.in.Scale.PxPerMetre[0], b.in.Scale.PxPerMetre[1])
	ink, _ := cleanWalls(b.in.Edges)
	for _, o := range b.ops {
		// A door in a void's wall (a shaft's) closes it like the wall.
		ink = append(ink, seg{a: o.A, b: o.B})
	}
	pts := make([]Point, 0, 2*len(walls))
	for _, s := range walls {
		pts = append(pts, s.a, s.b)
	}
	building := math.Abs(signedArea(hull(pts)))
	m := newMask(walls, ink, voidCell*pxm, voidSeal*pxm)
	names := m.nameCells(b.in.Labels, at, voidLabelReach*pxm)
	var out []seg
	for _, p := range at {
		ring := m.flood(p, maxVoidShare*building, voidSeal*pxm, names)
		if len(ring) < 3 {
			b.voidMissed++
			continue
		}
		b.voidAt = append(b.voidAt, p)
		for i := range ring {
			a, c := ring[i], ring[(i+1)%len(ring)]
			s := newSeg(a, c, canonical(unit(sub(c, a))), -1, 0, 0, false)
			s.void = true
			out = append(out, s)
		}
	}
	slices.SortFunc(out, cmpSeg)
	return out
}

// nameCells are the cells of the room names that are no void's own: a
// flood that reaches one has leaked out of the void through a gap wider
// than the seal and across a floor closed further out (the more gaps
// elsewhere doorways close, the likelier), so it is floor, not a hole in
// it. A name behind walls inside the void, a stair or lift core's, is
// never reached: only the open floor round it is flooded; nor does a
// small room's name (a stair, a lift) stop it, as those stand in atria
// unwalled. The void's own label lies within reach of one of the points
// at.
func (m *mask) nameCells(labels []Label, at []Point, reach float64) map[int]bool {
	out := map[int]bool{}
	for _, l := range labels {
		// A stair or lift may stand in an atrium with no walls round it.
		if l.Kind != LabelName || hasWord(nameWords(l.Text), smallRoomWords) {
			continue
		}
		own := false
		for _, p := range at {
			own = own || dist(l.At, p) <= reach
		}
		if i, j := m.index(l.At); !own && i >= 0 && j >= 0 && i < m.w && j < m.h {
			out[j*m.w+i] = true
		}
	}
	return out
}

// mask is the grid voids are flooded on: blocked cells are within the seal
// of a wall or an edge.
type mask struct {
	cell    float64
	x0, y0  float64
	w, h    int
	blocked []bool
}

func newMask(walls, ink []seg, cell, seal float64) *mask {
	lo, hi := Point{X: math.Inf(1), Y: math.Inf(1)}, Point{X: math.Inf(-1), Y: math.Inf(-1)}
	for _, s := range slices.Concat(walls, ink) {
		for _, p := range [2]Point{s.a, s.b} {
			lo = Point{X: math.Min(lo.X, p.X), Y: math.Min(lo.Y, p.Y)}
			hi = Point{X: math.Max(hi.X, p.X), Y: math.Max(hi.Y, p.Y)}
		}
	}
	// Open cells all round, so a flood that gets out of the building
	// reaches the border.
	margin := seal + 2*cell
	lo, hi = Point{X: lo.X - margin, Y: lo.Y - margin}, Point{X: hi.X + margin, Y: hi.Y + margin}
	cell = math.Max(cell, math.Sqrt((hi.X-lo.X)*(hi.Y-lo.Y)/maxVoidCells))
	m := &mask{cell: cell, x0: lo.X, y0: lo.Y}
	m.w, m.h = int(math.Ceil((hi.X-lo.X)/cell))+1, int(math.Ceil((hi.Y-lo.Y)/cell))+1
	m.blocked = make([]bool, m.w*m.h)
	for _, s := range walls {
		m.stamp(s.a, s.b, seal+s.t/2)
	}
	for _, s := range ink {
		m.stamp(s.a, s.b, seal)
	}
	return m
}

// stamp blocks the cells whose centres are within r of segment a–b,
// visiting only the squares round points a cell apart along it: a long
// slanted wall's bounding box would be most of the grid.
func (m *mask) stamp(a, b Point, r float64) {
	n := max(1, int(math.Ceil(dist(a, b)/m.cell)))
	reach := int(math.Ceil(r/m.cell)) + 1
	for k := 0; k <= n; k++ {
		ci, cj := m.index(lerp(a, b, float64(k)/float64(n)))
		for j := max(cj-reach, 0); j <= min(cj+reach, m.h-1); j++ {
			for i := max(ci-reach, 0); i <= min(ci+reach, m.w-1); i++ {
				if !m.blocked[j*m.w+i] && segDist(m.centre(i, j), a, b) <= r {
					m.blocked[j*m.w+i] = true
				}
			}
		}
	}
}

func (m *mask) index(p Point) (int, int) {
	return int(math.Floor((p.X - m.x0) / m.cell)), int(math.Floor((p.Y - m.y0) / m.cell))
}

func (m *mask) centre(i, j int) Point {
	return Point{X: m.x0 + (float64(i)+0.5)*m.cell, Y: m.y0 + (float64(j)+0.5)*m.cell}
}

// flood fills the open cells reachable from p and returns the region's
// outline, grown back by the seal and simplified, or nil when p is
// blocked, or the region reaches the grid's border or a cell in stop or
// is larger than limit px².
func (m *mask) flood(p Point, limit, seal float64, stop map[int]bool) []Point {
	i, j := m.index(p)
	if i < 0 || j < 0 || i >= m.w || j >= m.h || m.blocked[j*m.w+i] {
		return nil
	}
	in := make([]bool, len(m.blocked))
	in[j*m.w+i] = true
	queue := []int{j*m.w + i}
	maxCells := int(limit / (m.cell * m.cell))
	for k := 0; k < len(queue); k++ {
		c := queue[k]
		x, y := c%m.w, c/m.w
		if x == 0 || y == 0 || x == m.w-1 || y == m.h-1 || len(queue) > maxCells || stop[c] {
			return nil
		}
		for _, n := range [4]int{c - 1, c + 1, c - m.w, c + m.w} {
			if !in[n] && !m.blocked[n] {
				in[n] = true
				queue = append(queue, n)
			}
		}
	}
	m.grow(in, seal)
	return rdpRing(m.outline(in), 1.5*m.cell)
}

// grow dilates a region by r px. The flood stops r short of every line, as
// it was drawn thickened; grown back, the region's edge is on the lines,
// and across a gap it sealed, half-way.
func (m *mask) grow(in []bool, r float64) {
	k := int(math.Round(r / m.cell))
	var edge []int
	for c, ok := range in {
		x, y := c%m.w, c/m.w
		if ok && (!in[c-1] || !in[c+1] || !in[c-m.w] || !in[c+m.w]) && x > 0 && y > 0 {
			edge = append(edge, c)
		}
	}
	for _, c := range edge {
		x, y := c%m.w, c/m.w
		for j := max(y-k, 0); j <= min(y+k, m.h-1); j++ {
			for i := max(x-k, 0); i <= min(x+k, m.w-1); i++ {
				if (i-x)*(i-x)+(j-y)*(j-y) <= k*k {
					in[j*m.w+i] = true
				}
			}
		}
	}
}

// outline traces the outer boundary of a 4-connected region of cells, as
// grid corners in px. Corner (x, y) is the top-left of cell (x, y); the walk
// keeps the region on its right in image coordinates (y down), starting
// along the top of the topmost, then leftmost, cell.
func (m *mask) outline(in []bool) []Point {
	cell := func(i, j int) bool { return i >= 0 && j >= 0 && i < m.w && j < m.h && in[j*m.w+i] }
	steps := [4][2]int{{1, 0}, {0, 1}, {-1, 0}, {0, -1}} // → ↓ ← ↑
	// sides are the cells to the right and left of the unit step from
	// corner (x, y) along d.
	sides := func(x, y, d int) (right, left bool) {
		switch d {
		case 0:
			return cell(x, y), cell(x, y-1)
		case 1:
			return cell(x-1, y), cell(x, y)
		case 2:
			return cell(x-1, y-1), cell(x-1, y)
		}
		return cell(x, y-1), cell(x-1, y-1)
	}
	start := slices.Index(in, true)
	sx, sy := start%m.w, start/m.w
	x, y, d := sx, sy, 0
	var ring []Point
	for len(ring) <= len(in) {
		ring = append(ring, Point{X: m.x0 + float64(x)*m.cell, Y: m.y0 + float64(y)*m.cell})
		x, y = x+steps[d][0], y+steps[d][1]
		// Right turn first: where two of the region's cells touch only at
		// a corner they are not joined there, and the right turn keeps to
		// the cell the walk is on.
		for t := range 3 {
			if r, l := sides(x, y, (d+5-t)%4); r && !l {
				d = (d + 5 - t) % 4
				break
			}
		}
		if x == sx && y == sy {
			return ring
		}
	}
	return nil
}

// rdpRing simplifies a closed ring (Douglas–Peucker, split at the vertex
// farthest from the first) to vertices within tol of it, on the working
// grid.
func rdpRing(ring []Point, tol float64) []Point {
	if len(ring) < 4 {
		return nil
	}
	far := 0
	for i, p := range ring {
		if dist(p, ring[0]) > dist(ring[far], ring[0]) {
			far = i
		}
	}
	closed := append(slices.Clone(ring), ring[0])
	keep := make([]bool, len(closed))
	keep[0], keep[far] = true, true
	var rdp func(i, j int)
	rdp = func(i, j int) {
		best, bestD := -1, tol
		for k := i + 1; k < j; k++ {
			if d := segDist(closed[k], closed[i], closed[j]); d > bestD {
				best, bestD = k, d
			}
		}
		if best >= 0 {
			keep[best] = true
			rdp(i, best)
			rdp(best, j)
		}
	}
	rdp(0, far)
	rdp(far, len(ring))
	var out []Point
	for i, p := range ring {
		if keep[i] {
			out = append(out, qp(p))
		}
	}
	out = slices.Compact(out)
	if len(out) < 3 || signedArea(out) == 0 {
		return nil
	}
	return out
}

// smallestFace is the bounded face of least area with p inside, or -1.
func smallestFace(g *graph, fs []face, p Point) int {
	best := -1
	for i, f := range fs {
		if f.area > 0 && (best < 0 || f.area < fs[best].area) && inside(p, g.poly(f.verts)) {
			best = i
		}
	}
	return best
}

// voidFaces marks the faces of the voids voidEdges found and records their
// polygons. A face too small to be a room is no void either: a label boxed
// in on the stairs layer, say.
func (b *builder) voidFaces() map[int]bool {
	out := map[int]bool{}
	for _, p := range b.voidAt {
		f := smallestFace(b.g, b.fs, p)
		// Two labels in one atrium make one void.
		if f >= 0 && out[f] {
			continue
		}
		var poly []Point
		if f >= 0 {
			poly = simplify(b.g.poly(b.fs[f].verts), 0.25)
		}
		if a, _ := faceSize(poly, b.sc.sx, b.sc.sy); a <= minRoomArea {
			b.voidMissed++
			continue
		}
		out[f] = true
		b.voids = append(b.voids, poly)
	}
	return out
}
