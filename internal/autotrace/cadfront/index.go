package cadfront

import (
	"math"
	"slices"
)

// grid is a spatial hash of segments, so finding what lies near a point
// costs the few segments nearby rather than all of them.
type grid struct {
	cell  float64
	cells map[[2]int][]int
}

func newGrid(cell float64) *grid { return &grid{cell: cell, cells: map[[2]int][]int{}} }

// maxCells bounds the cells one segment is filed under: a wall many
// kilometres long in a drawing's stray corner shouldn't fill memory.
const maxCells = 4096

func (g *grid) key(p point) [2]int {
	return [2]int{int(math.Floor(p.X / g.cell)), int(math.Floor(p.Y / g.cell))}
}

// add files segment i under every cell it passes through, sampled at half
// a cell so no cell it crosses is missed by more than a neighbour (and
// lookups search one cell further).
func (g *grid) add(i int, a, b point) {
	steps := min(int(math.Ceil(2*b.Sub(a).Len()/g.cell)), maxCells)
	last := [2]int{math.MinInt, math.MinInt}
	for s := 0; s <= steps; s++ {
		t := 0.0
		if steps > 0 {
			t = float64(s) / float64(steps)
		}
		k := g.key(a.Add(b.Sub(a).Mul(t)))
		if k != last {
			g.cells[k] = append(g.cells[k], i)
			last = k
		}
	}
}

// near returns, in increasing order, the segments filed within rad of p
// (and perhaps a little further).
func (g *grid) near(p point, rad float64) []int {
	lo, hi := g.key(p.Sub(point{X: rad, Y: rad})), g.key(p.Add(point{X: rad, Y: rad}))
	lo[0], lo[1], hi[0], hi[1] = lo[0]-1, lo[1]-1, hi[0]+1, hi[1]+1
	if (hi[0]-lo[0]+1)*(hi[1]-lo[1]+1) > maxCells {
		return nil
	}
	var out []int
	for x := lo[0]; x <= hi[0]; x++ {
		for y := lo[1]; y <= hi[1]; y++ {
			out = append(out, g.cells[[2]int{x, y}]...)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// wallGrid indexes r.walls.
func (r *reader) wallGrid() *grid {
	g := newGrid(r.metres(gridCell))
	for i, w := range r.walls {
		g.add(i, w.a, w.b)
	}
	return g
}

// gridCell is the spatial index's cell, in metres: about a door's width,
// the distance most lookups search.
const gridCell = 1.0

// joinEnds runs each wall end that stops short of a wall across its path
// on to that wall's centreline. Paired faces end where they meet the other
// wall's face, half its thickness short of the centreline where the two
// should join; the shortfall is within the wall it stops at, so it can't
// cross a gap.
//
// Every end moves at once, against where the others were, so the result
// doesn't depend on the order walls were found in. A second pass reaches
// walls that only came within reach in the first, such as a partition
// meeting a wall that was itself stopping short at a T.
func (r *reader) joinEnds() {
	for range 2 {
		g := r.wallGrid()
		ends := make([][2]point, len(r.walls))
		for i, w := range r.walls {
			ends[i] = [2]point{r.reach(g, i, w.a, w.a.Sub(w.b)), r.reach(g, i, w.b, w.b.Sub(w.a))}
		}
		for i := range r.walls {
			r.walls[i].a, r.walls[i].b = ends[i][0], ends[i][1]
		}
	}
}

// minCrossing is the sine of the smallest angle at which a wall end is
// joined to another wall: shallower, and the two are one wall continuing.
var minCrossing = math.Sin(10 * math.Pi / 180)

func (r *reader) reach(g *grid, i int, end, out point) point {
	d := unit(out)
	self := r.walls[i]
	best, bestS := end, math.Inf(1)
	for _, j := range g.near(end, r.metres(maxGap)) {
		if j == i {
			continue
		}
		w := r.walls[j]
		dj := w.b.Sub(w.a)
		c := cross(d, dj)
		if math.Abs(c) < minCrossing*dj.Len() {
			continue
		}
		s := cross(w.a.Sub(end), dj) / c
		if s <= 0 || s > w.thick/2+r.metres(collinear) || s >= bestS {
			continue
		}
		hit := end.Add(d.Mul(s))
		t, l := dot(hit.Sub(w.a), unit(dj)), dj.Len()
		if slack := self.thick/2 + r.metres(collinear); t < -slack || t > l+slack {
			continue
		}
		best, bestS = hit, s
	}
	return best
}
