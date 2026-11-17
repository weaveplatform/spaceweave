package rasterfront

import (
	"math"

	"github.com/weaveplatform/spaceweave/internal/floorgraph"
)

// symbolSides is how many of the thinnest wall widths a hairline component
// may span and still count as text or a symbol (a letter, a fixture, a
// north arrow) rather than unexplained linework.
const symbolSides = 12

// coverage is the share of the ink explained: wall mask, hairline components
// small enough to be text or symbols, and the hairlines drawing the openings
// found (door leaves and arcs, glazing).
func (r *reader) coverage(ops []floorgraph.Opening, thinnest int) float64 {
	explained := 0
	for _, v := range r.wall.px {
		if v {
			explained++
		}
	}
	symbol := max(24, symbolSides*thinnest)
	zones := make([][4]int, 0, len(ops))
	for _, o := range ops {
		// The square on the gap reaches every leaf and arc the opening can
		// draw, on either side.
		w := math.Hypot(o.B.X-o.A.X, o.B.Y-o.A.Y)
		cx, cy := (o.A.X+o.B.X)/2, (o.A.Y+o.B.Y)/2
		zones = append(zones, [4]int{
			int(
				math.Floor(cx - w),
			),
			int(math.Floor(cy - w)),
			int(math.Ceil(cx + w)),
			int(math.Ceil(cy + w)),
		})
	}
	seen := make([]bool, len(r.ink.px))
	var stack []int
	for start, v := range r.ink.px {
		if !v || r.wall.px[start] || seen[start] {
			continue
		}
		// Flood the 8-connected hairline component from start.
		seen[start] = true
		stack = append(stack[:0], start)
		n := 0
		x0, y0, x1, y1 := r.w, r.h, -1, -1
		for len(stack) > 0 {
			i := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			n++
			x, y := i%r.w, i/r.w
			x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					xx, yy := x+dx, y+dy
					if xx < 0 || yy < 0 || xx >= r.w || yy >= r.h {
						continue
					}
					j := yy*r.w + xx
					if r.ink.px[j] && !r.wall.px[j] && !seen[j] {
						seen[j] = true
						stack = append(stack, j)
					}
				}
			}
		}
		if max(x1-x0, y1-y0)+1 <= symbol || inZone(zones, x0, y0, x1, y1) {
			explained += n
		}
	}
	return float64(explained) / float64(r.inked)
}

func inZone(zones [][4]int, x0, y0, x1, y1 int) bool {
	for _, z := range zones {
		if x0 <= z[2] && x1 >= z[0] && y0 <= z[3] && y1 >= z[1] {
			return true
		}
	}
	return false
}
