package rasterfront

import (
	"math"
	"slices"

	"github.com/weaveplatform/spaceweave/internal/floorgraph"
)

// Slanted walls are read by turning the image so they lie along an axis and
// banding them there, with everything the axis-aligned reading does
// (openings, jambs, stubs), then turning the evidence back. The directions
// come from what the axis-aligned bands leave of the wall mask.
const (
	// minSlantDeg keeps directions this close to an axis for the axis
	// bands: floorgraph snaps walls within 2° of an axis onto it, and the
	// bands absorb the drift of a wall a few degrees off.
	minSlantDeg = 3.0
	// slantElong is how much longer than wide a leftover blob must be to
	// vote for its direction: a wall, not a column or a corner.
	slantElong = 3
	// maxSlants bounds the directions read, each a pass over the image.
	maxSlants = 3
	// slantSuppress is how close, in degrees, two directions may be before
	// the weaker is taken as the stronger one drawn a little off.
	slantSuppress = 4.0
	// rotBits is the precision of the rotation's sine and cosine. Rounded
	// to multiples of 2⁻²⁰, every product and sum in the resampling is exact
	// in float64, so a fused multiply-add on one machine and not another
	// can't move a pixel.
	rotBits = 1 << 20
)

// blob is a connected leftover of the wall mask, with its principal
// direction.
type blob struct {
	deg    float64 // [0, 180)
	length float64
}

// rotation turns image coordinates by -deg so a wall at deg lies along x,
// and offsets them so the turned image starts at the origin.
type rotation struct {
	c, s   float64 // cos and sin of deg, rounded to rotBits
	ox, oy float64 // integer offset added after turning
	w, h   int     // the turned image's size
}

func newRotation(deg float64, w, h int) rotation {
	rad := deg * math.Pi / 180
	c := math.Round(math.Cos(rad)*rotBits) / rotBits
	s := math.Round(math.Sin(rad)*rotBits) / rotBits
	rt := rotation{c: c, s: s}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, p := range [4][2]float64{{0, 0}, {float64(w), 0}, {0, float64(h)}, {float64(w), float64(h)}} {
		x, y := rt.turn(p[0], p[1])
		minX, minY = math.Min(minX, x), math.Min(minY, y)
		maxX, maxY = math.Max(maxX, x), math.Max(maxY, y)
	}
	rt.ox, rt.oy = -math.Floor(minX), -math.Floor(minY)
	rt.w = int(math.Ceil(maxX + rt.ox))
	rt.h = int(math.Ceil(maxY + rt.oy))
	return rt
}

// turn rotates by -deg about the origin, without the offset.
func (rt rotation) turn(x, y float64) (float64, float64) {
	return float64(x*rt.c) + float64(y*rt.s), float64(y*rt.c) - float64(x*rt.s)
}

// back maps a point of the turned image to the source image.
func (rt rotation) back(p floorgraph.Point) floorgraph.Point {
	x, y := p.X-rt.ox, p.Y-rt.oy
	return floorgraph.Point{
		X: float64(x*rt.c) - float64(y*rt.s),
		Y: float64(x*rt.s) + float64(y*rt.c),
	}
}

// resample is m seen through the rotation, nearest pixel.
func (rt rotation) resample(m mask) mask {
	out := mask{w: rt.w, h: rt.h, px: make([]bool, rt.w*rt.h)}
	for y := range rt.h {
		for x := range rt.w {
			sx := rt.back(floorgraph.Point{X: float64(x) + 0.5, Y: float64(y) + 0.5})
			ix, iy := int(math.Floor(sx.X)), int(math.Floor(sx.Y))
			if ix >= 0 && iy >= 0 && ix < m.w && iy < m.h && m.px[iy*m.w+ix] {
				out.px[y*rt.w+x] = true
			}
		}
	}
	return out
}

// leftover is the wall mask the axis-aligned pieces don't account for: a
// pixel more than one away from every piece's rectangle.
func (r *reader) leftover() mask {
	out := mask{w: r.w, h: r.h, px: slices.Clone(r.wall.px)}
	for _, p := range r.pieces {
		b := p.rect()
		for y := max(b[1]-1, 0); y <= min(b[3]+1, r.h-1); y++ {
			for x := max(b[0]-1, 0); x <= min(b[2]+1, r.w-1); x++ {
				out.px[y*r.w+x] = false
			}
		}
	}
	return out
}

// blobs are the 8-connected parts of m with at least minArea pixels that
// are clearly longer than wide, with their principal directions.
func blobs(m mask, minArea int) []blob {
	seen := make([]bool, len(m.px))
	var out []blob
	var stack, members []int
	for start, v := range m.px {
		if !v || seen[start] {
			continue
		}
		seen[start] = true
		stack = append(stack[:0], start)
		members = members[:0]
		for len(stack) > 0 {
			i := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			members = append(members, i)
			x, y := i%m.w, i/m.w
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					xx, yy := x+dx, y+dy
					if xx < 0 || yy < 0 || xx >= m.w || yy >= m.h {
						continue
					}
					if j := yy*m.w + xx; m.px[j] && !seen[j] {
						seen[j] = true
						stack = append(stack, j)
					}
				}
			}
		}
		if len(members) < minArea {
			continue
		}
		if b, ok := principal(members, m.w); ok {
			out = append(out, b)
		}
	}
	return out
}

// principal is the direction and length of a set of pixels along its
// principal axis, if it is clearly longer than wide. The moments are summed
// in integers about the first pixel, so they are exact.
func principal(members []int, w int) (blob, bool) {
	x0, y0 := members[0]%w, members[0]/w
	var n, sx, sy, sxx, syy, sxy int64
	for _, i := range members {
		x, y := int64(i%w-x0), int64(i/w-y0)
		n++
		sx, sy = sx+x, sy+y
		sxx, syy, sxy = sxx+x*x, syy+y*y, sxy+x*y
	}
	cxx, cyy, cxy := n*sxx-sx*sx, n*syy-sy*sy, n*sxy-sx*sy
	rad := 0.5 * math.Atan2(float64(2*cxy), float64(cxx-cyy))
	// The variances along and across the axis give the lengths of the
	// rectangle with the same moments: 12 σ² is a side squared.
	tr, det := float64(cxx+cyy), math.Hypot(float64(cxx-cyy), float64(2*cxy))
	nn := float64(n * n)
	along, across := math.Sqrt(12*(tr+det)/(2*nn)), math.Sqrt(12*math.Max(tr-det, 0)/(2*nn))
	if along < slantElong*math.Max(across, 1) {
		return blob{}, false
	}
	deg := rad * 180 / math.Pi
	if deg < 0 {
		deg += 180
	}
	return blob{deg: deg, length: along}, true
}

// slantDirections are the off-axis directions, degrees in [0, 180) rounded
// to a hundredth, carrying at least minLen px of leftover wall, strongest
// first.
func slantDirections(bs []blob, minLen float64) []float64 {
	var hist [180]float64
	for _, b := range bs {
		if offAxis(b.deg) >= minSlantDeg {
			hist[int(b.deg)%180] += b.length
		}
	}
	var out []float64
	for range maxSlants {
		best, bestW := -1, 0.0
		for k := range hist {
			w := hist[(k+179)%180] + hist[k] + hist[(k+1)%180]
			if w > bestW && !slices.ContainsFunc(out, func(d float64) bool {
				return angleGap(d, float64(k)+0.5) < slantSuppress ||
					angleGap(d+90, float64(k)+0.5) < slantSuppress
			}) {
				best, bestW = k, w
			}
		}
		if best < 0 || bestW < minLen {
			break
		}
		// The direction is the length-weighted mean of the blobs near
		// the peak, measured as offsets from it so the mean doesn't wrap.
		c := float64(best) + 0.5
		sum, wsum := 0.0, 0.0
		for _, b := range bs {
			d := b.deg - c
			d -= 180 * math.Round(d/180)
			if math.Abs(d) <= 2 && offAxis(b.deg) >= minSlantDeg {
				sum += float64(d * b.length)
				wsum += b.length
			}
		}
		deg := c + sum/wsum
		deg -= 180 * math.Floor(deg/180)
		out = append(out, math.Round(deg*100)/100)
	}
	return out
}

func offAxis(deg float64) float64 {
	d := math.Mod(deg, 90)
	return math.Min(d, 90-d)
}

func angleGap(a, b float64) float64 {
	d := math.Mod(math.Abs(a-b), 180)
	return math.Min(d, 180-d)
}

// slanted reads the walls and openings in the leftover wall mask along each
// slanted direction it holds.
func (r *reader) slanted(
	thinnest, tRef int,
	lim [2]gapLimits,
) ([]floorgraph.Segment, []floorgraph.Opening, int) {
	left := r.leftover()
	bs := blobs(left, thinnest*thinnest)
	dirs := slantDirections(bs, float64(8*thinnest))
	walls := make([]floorgraph.Segment, 0, 16*len(dirs))
	ops := make([]floorgraph.Opening, 0, 4*len(dirs))
	for _, deg := range dirs {
		rt := newRotation(deg, r.w, r.h)
		sub := &reader{
			w: rt.w, h: rt.h,
			ink: rt.resample(r.ink), wall: rt.resample(left),
			inked: r.inked,
		}
		kept := dropThin(sub.bands(thinnest), tRef)
		ws := segments(kept, thinnest)
		var found []floorgraph.Opening
		for _, l := range lines(kept) {
			o, stubs := sub.openings(l, lim[l.ax])
			found = append(found, o...)
			ws = append(ws, stubs...)
		}
		for i := range ws {
			ws[i].A, ws[i].B = rt.back(ws[i].A), rt.back(ws[i].B)
		}
		for i := range found {
			o := &found[i]
			o.A, o.B = rt.back(o.A), rt.back(o.B)
			if o.Leafed {
				o.Leaf = rt.back(o.Leaf)
			}
		}
		walls = append(walls, ws...)
		ops = append(ops, found...)
	}
	return walls, ops, len(dirs)
}
