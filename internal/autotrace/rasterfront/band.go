package rasterfront

// Stroke widths are handled as levels: a stroke T px wide has a centre pixel
// ceil(T/2) px from paper, so level 1 is a 1–2 px hairline, level 2 a
// 3–4 px stroke, and so on. The ridge of the distance transform (the pixels
// further from paper than their neighbours across the stroke) carries one
// sample per pixel of stroke length.

// band is the range of stroke levels drawn as walls.
type band struct {
	lo, hi int
	// weight is the ridge length, px, behind the estimate.
	weight int
}

// Thresholds for wallBand.
const (
	// minWallLevel is the thinnest stroke that can be a wall: 3 px. Thinner
	// strokes are hairlines, which plans use for everything (text, furniture,
	// dimensions, glazing) and for walls only when drawn as two parallel
	// lines, a case this front end leaves for later.
	minWallLevel = 2
	// minAlong is the shortest straight run, px, a ridge sample must lie on
	// to count, and alongPerLevel how much longer it must be per level: walls
	// are long for their width, glyph strokes and symbol blobs are not.
	minAlong      = 12
	alongPerLevel = 6
	// The thinnest wall level needs at least loShare of the wall-level ridge
	// length, and minWall px of it, so a few thick glyph strokes or crossings
	// below the real walls don't set the cut-off; hiShare is the same floor
	// for the thickest. loShare is low because a small plan with thick
	// exterior walls can have little partition: a 4.5 m partition in a
	// 10 × 7.5 m room is 12% of the wall length.
	loShare = 0.05
	hiShare = 0.02
	// Walls are only reported when their ridge length is at least minWall px
	// and minWallShare of all the long strokes, so an image of hairlines
	// with a few bold symbols doesn't yield walls made of symbols.
	minWall      = 40
	minWallShare = 0.03
)

// wallBand estimates the wall stroke levels from the histogram of ridge
// levels, counting only ridge pixels on long straight runs.
func wallBand(w, h int, ink []bool, dt []int) (band, bool) {
	return bandOf(ridges(w, h, ink, dt))
}

// ridges is the histogram of ridge levels on long straight runs: hist[k] is
// the length, px, of such ridge k px from paper.
func ridges(w, h int, ink []bool, dt []int) []int {
	hr, vr := runLengths(w, h, ink)
	at := func(x, y int) int {
		if x < 0 || y < 0 || x >= w || y >= h {
			return 0
		}
		return dt[y*w+x]
	}
	var hist []int
	for y := range h {
		for x := range w {
			i := y*w + x
			d := dt[i]
			if !ink[i] {
				continue
			}
			// Strict on one side only, so an even-width stroke, whose two
			// middle lines tie, contributes one line rather than two.
			across := d > at(x, y-1) && d >= at(x, y+1)
			along := d > at(x-1, y) && d >= at(x+1, y)
			if !across && !along {
				continue
			}
			k := isqrt(d)
			if int(max(hr[i], vr[i])) < max(minAlong, alongPerLevel*k) {
				continue
			}
			for len(hist) <= k {
				hist = append(hist, 0)
			}
			hist[k]++
		}
	}
	return hist
}

func bandOf(hist []int) (band, bool) {
	total, wall := 0, 0
	for k, n := range hist {
		total += n
		if k >= minWallLevel {
			wall += n
		}
	}
	if wall < minWall || float64(wall) < minWallShare*float64(total) {
		return band{}, false
	}
	b := band{weight: wall}
	peak := minWallLevel
	for k := minWallLevel; k < len(hist); k++ {
		if b.lo == 0 && hist[k] >= minWall && float64(hist[k]) >= loShare*float64(wall) {
			b.lo = k
		}
		if float64(hist[k]) >= hiShare*float64(wall) {
			b.hi = k
		}
		if hist[k] > hist[peak] {
			peak = k
		}
	}
	// Walls spread over many widths can leave every level under the share;
	// the commonest one is then the best guess at the thinnest.
	if b.lo == 0 {
		b.lo = peak
	}
	b.hi = max(b.hi, b.lo)
	return b, true
}

// runLengths is, for every ink pixel, the length of the horizontal and the
// vertical ink run through it.
func runLengths(w, h int, ink []bool) (hr, vr []int32) {
	hr, vr = make([]int32, w*h), make([]int32, w*h)
	fill := func(dst []int32, start, n, stride int) {
		for i := range n {
			dst[start+i*stride] = int32(n) //nolint:gosec // n is an image dimension
		}
	}
	for y := range h {
		s := -1
		for x := 0; x <= w; x++ {
			on := x < w && ink[y*w+x]
			if on && s < 0 {
				s = x
			} else if !on && s >= 0 {
				fill(hr, y*w+s, x-s, 1)
				s = -1
			}
		}
	}
	for x := range w {
		s := -1
		for y := 0; y <= h; y++ {
			on := y < h && ink[y*w+x]
			if on && s < 0 {
				s = y
			} else if !on && s >= 0 {
				fill(vr, s*w+x, y-s, w)
				s = -1
			}
		}
	}
	return hr, vr
}
