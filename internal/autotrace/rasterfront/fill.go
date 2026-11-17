package rasterfront

import (
	"math"
	"slices"
)

// Architects mostly draw a wall as its two faces, often hatched between:
// a thin strip of paper, or a run of small cells, enclosed by hairlines.
// Filling those makes the walls the solid bands the rest of the reader
// looks for. What a strip is can't be told from its width alone (a
// cabinet, a bench and a wall are all long and narrow), so the width comes
// from the walls themselves. Outlined walls that meet share one outline,
// so the inside of a building's walls is one strip branching through the
// whole plan, by far the longest, and sparse in its bounding box as no room
// is; hatching cuts it into cells, and then the exterior wall's cells, a
// hairline from the outside, give the width. Strips up to about that wide
// are filled, but only when they are more of the drawing than its solid
// walls: in a plan whose walls are filled, the panes between a window's
// glazing lines are such strips, and filling them would close the windows.
const (
	// fillElong is how much longer than at its widest a strip must be,
	// measured along its medial axis, branches and all: a rectangle's
	// axis has a branch to each corner, which makes a desk about three
	// times as long as it is wide.
	fillElong = 4
	// fillBulge is how much wider than the walls' strips a strip may be
	// at its widest: a junction, or a column set in the wall. A room's
	// widest point is many times the gaps between its furniture.
	fillBulge = 3
	// fillNetwork is the share of all strips' length the longest must have
	// to be the walls' outline, and fillSparse the most of its bounding box
	// it may cover.
	fillNetwork = 0.25
	fillSparse  = 0.2
	// fillOutside is the share of an exterior strip's edge that must lie
	// within fillReach px of the outside.
	fillOutside = 0.3
	fillReach   = 3
	// fillSlack is how much wider than the walls' strips another may be.
	fillSlack = 1.25
	// fillCell is the largest area, px², of a cell of hatching filled
	// however short it is: the cells of a hatched partition are no longer
	// than the partition is thick.
	fillCell = 32
)

// region is a 4-connected component of paper.
type region struct {
	area          int
	ridge         int // pixels on its medial axis: its length, summed over branches
	maxD          int // the largest squared distance to ink inside it
	width         float64
	edge, nearOut int // edge pixels, and those within fillReach of the outside
	box           [4]int
	border        bool
}

func (g region) widest() float64 { return 2*math.Sqrt(float64(g.maxD)) - 1 }

func (g region) strip() bool {
	return !g.border && float64(g.ridge) >= fillElong*max(g.widest(), 1)
}

type strip struct{ width, length float64 }

// exteriorWidth is the exterior wall's strip width. Windows complicate it:
// a window is glazing lines inside the wall's faces, so its panes are
// narrower strips, and a plan with many windows has more pane than wall
// along the outside. So it is the widest width that, with the strips wider
// still, makes up a tenth of the length, after dropping strips over three
// times the median width (a stair or a canopy drawn across the wall).
func exteriorWidth(ext []strip) float64 {
	slices.SortFunc(ext, func(a, b strip) int { return cmpFloat(b.width, a.width) })
	median := ext[len(ext)/2].width
	total := 0.0
	kept := ext[:0:0]
	for _, s := range ext {
		if s.width <= 3*median {
			kept = append(kept, s)
			total += s.length
		}
	}
	k, acc := 0, kept[0].length
	for k < len(kept)-1 && 10*acc < total {
		k++
		acc += kept[k].length
	}
	return kept[k].width
}

// networkWidth is the width of the walls' outline when the plan has one:
// the longest strip, with a good share of all the strips' length, sparse
// in its bounding box.
func networkWidth(rs []region) (float64, bool) {
	total, found := 0, false
	var g region
	for _, r := range rs {
		if r.strip() {
			total += r.ridge
			if !found || r.ridge > g.ridge {
				g, found = r, true
			}
		}
	}
	if !found || float64(g.ridge) < fillNetwork*float64(total) {
		return 0, false
	}
	box := (g.box[2] - g.box[0] + 1) * (g.box[3] - g.box[1] + 1)
	return g.width, float64(g.area) <= fillSparse*float64(box)
}

// wallWidth is the width of the strips the walls are drawn as, or 0 when
// the plan has none. Which strips are near the outside is only worked out
// when there is no outline network to go by, as it costs a distance
// transform.
func wallWidth(w, h int, ink []bool, label []int32, rs []region) float64 {
	if ww, ok := networkWidth(rs); ok {
		return ww
	}
	nearOutside(w, h, ink, label, rs)
	var ext []strip
	for _, g := range rs {
		if g.strip() && g.edge > 0 && float64(g.nearOut) >= fillOutside*float64(g.edge) {
			ext = append(ext, strip{g.width, float64(g.ridge)})
		}
	}
	if len(ext) == 0 {
		return 0
	}
	return exteriorWidth(ext)
}

// nearOutside counts each region's edge pixels within fillReach of the
// paper outside the building.
func nearOutside(w, h int, ink []bool, label []int32, rs []region) {
	outside := make([]bool, len(ink))
	for i, l := range label {
		if l >= 0 && rs[l].border {
			outside[i] = true
		}
	}
	outD := sqDist(w, h, outside, false)
	for i, l := range label {
		if l >= 0 && outD[i] <= fillReach*fillReach && edgeOf(w, h, ink, i) {
			rs[l].nearOut++
		}
	}
}

func edgeOf(w, h int, ink []bool, i int) bool {
	x, y := i%w, i/w
	return x > 0 && ink[i-1] || x < w-1 && ink[i+1] || y > 0 && ink[i-w] || y < h-1 && ink[i+w]
}

// fillWalls returns the ink with wall strips and hatch cells filled, and how
// many pixels it filled. solid is the length, px, of the solid walls found
// without filling.
func fillWalls(w, h int, ink []bool, solid int) ([]bool, int) {
	label, rs := regions(w, h, ink)
	if len(rs) == 0 {
		return ink, 0
	}
	measureRegions(w, h, ink, label, rs)
	ww := wallWidth(w, h, ink, label, rs)
	if ww == 0 {
		return ink, 0
	}
	limit := fillSlack*ww + 0.5
	fill := make([]bool, len(rs))
	strips := 0
	for k, g := range rs {
		long := g.strip() && g.width <= limit && g.widest() <= fillBulge*limit
		fill[k] = long || !g.border && g.area <= fillCell
		if long {
			strips += g.ridge
		}
	}
	if strips <= solid {
		return ink, 0
	}
	out := slices.Clone(ink)
	n := 0
	for i, l := range label {
		if l >= 0 && fill[l] {
			out[i] = true
			n++
		}
	}
	return out, n
}

// measureRegions fills in each region's ridge, widths, edge count and box. A region's
// width is the median width along its medial axis, so neither its
// junctions nor its ends skew it.
func measureRegions(w, h int, ink []bool, label []int32, rs []region) {
	paperD := sqDist(w, h, ink, true)
	at := func(x, y int) int {
		if x < 0 || y < 0 || x >= w || y >= h {
			return 0
		}
		return paperD[y*w+x]
	}
	type sample struct {
		l int32
		d int
	}
	var ridge []sample
	for k := range rs {
		rs[k].box = [4]int{w, h, -1, -1}
	}
	for i, l := range label {
		if l < 0 {
			continue
		}
		g := &rs[l]
		x, y := i%w, i/w
		g.box = [4]int{min(g.box[0], x), min(g.box[1], y), max(g.box[2], x), max(g.box[3], y)}
		d := paperD[i]
		g.maxD = max(g.maxD, d)
		// Strict on one side only, so a strip an even number of pixels
		// wide, whose two middle lines tie, counts one line.
		if d > at(x, y-1) && d >= at(x, y+1) || d > at(x-1, y) && d >= at(x+1, y) {
			g.ridge++
			ridge = append(ridge, sample{l, d})
		}
		if edgeOf(w, h, ink, i) {
			g.edge++
		}
	}
	slices.SortFunc(ridge, func(a, b sample) int {
		if a.l != b.l {
			return int(a.l - b.l)
		}
		return a.d - b.d
	})
	for i := 0; i < len(ridge); {
		j := i
		for j < len(ridge) && ridge[j].l == ridge[i].l {
			j++
		}
		rs[ridge[i].l].width = 2*math.Sqrt(float64(ridge[(i+j)/2].d)) - 1
		i = j
	}
}

// regions labels the 4-connected components of paper: label is -1 on ink.
func regions(w, h int, ink []bool) ([]int32, []region) {
	label := make([]int32, len(ink))
	for i, v := range ink {
		if v {
			label[i] = -1
		} else {
			label[i] = -2
		}
	}
	var rs []region
	var stack []int
	for start := range label {
		if label[start] != -2 {
			continue
		}
		id := int32(len(rs)) //nolint:gosec // fewer regions than pixels
		var g region
		label[start] = id
		stack = append(stack[:0], start)
		for len(stack) > 0 {
			i := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			g.area++
			x, y := i%w, i/w
			if x == 0 || y == 0 || x == w-1 || y == h-1 {
				g.border = true
			}
			for _, j := range [4]int{i - 1, i + 1, i - w, i + w} {
				if j < 0 || j >= len(label) || (j == i-1 && x == 0) || (j == i+1 && x == w-1) {
					continue
				}
				if label[j] == -2 {
					label[j] = id
					stack = append(stack, j)
				}
			}
		}
		rs = append(rs, g)
	}
	return label, rs
}
