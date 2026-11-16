// Package rasterfront finds wall, opening and scale evidence in a raster
// floor plan for floorgraph to build a trace from, without a model: the same
// image always gives the same evidence.
//
// The pipeline binarises the image with raster's Otsu threshold, measures
// every stroke's width with an exact distance transform, keeps the strokes
// as wide as the plan's walls (walls are the thick, long strokes; text,
// furniture, dimensions and glazing are hairlines), reads axis-aligned wall
// bands off that mask and looks along each wall line for the gaps openings
// leave.
//
// Seams left for later: slanted and curved walls (thin the wall mask to a
// skeleton instead of banding it), walls drawn as two hairlines, poché
// (hatched or filled walls), local thresholding for uneven scans, and OCR
// for labels.
package rasterfront

import (
	"errors"
	"fmt"
	"slices"

	"github.com/weaveplatform/spaceweave/internal/floorgraph"
	"github.com/weaveplatform/spaceweave/internal/raster"
)

// DefaultMaxPixels bounds the image Read accepts when Options leaves it 0:
// 40 MP is an A1 sheet at 200 dpi, and Read holds a few ints per pixel.
const DefaultMaxPixels = 40 << 20

// Errors Read returns.
var (
	ErrNoImage  = errors.New("no image")
	ErrTooLarge = errors.New("image too large")
	ErrNoInk    = errors.New("the image has no ink")
)

// Options tune Read.
type Options struct {
	// Scale is the image's scale when something other than the picture
	// knows it, e.g. the image was rendered from a DXF in known units. It
	// is passed through, and it sizes the openings looked for.
	Scale floorgraph.Scale
	// MaxPixels is the largest width × height read; DefaultMaxPixels when 0.
	MaxPixels int
}

// Read finds the evidence in src.
func Read(src *raster.Source, o Options) (floorgraph.Input, error) {
	if src == nil || src.Gray == nil {
		return floorgraph.Input{}, ErrNoImage
	}
	limit := o.MaxPixels
	if limit <= 0 {
		limit = DefaultMaxPixels
	}
	b := src.Gray.Bounds()
	w, h := b.Dx(), b.Dy()
	if w*h > limit {
		return floorgraph.Input{}, fmt.Errorf(
			"%w: %d×%d is over %d pixels",
			ErrTooLarge,
			w,
			h,
			limit,
		)
	}
	r := &reader{w: w, h: h, ink: mask{w: w, h: h, px: make([]bool, w*h)}}
	inked := 0
	for y := range h {
		row := src.Gray.Pix[src.Gray.PixOffset(b.Min.X, b.Min.Y+y):]
		for x := range w {
			if row[x] <= src.Threshold {
				r.ink.px[y*w+x] = true
				inked++
			}
		}
	}
	if inked == 0 {
		return floorgraph.Input{}, ErrNoInk
	}
	r.inked = inked
	in := floorgraph.Input{Size: [2]int{w, h}}
	if 2*inked > w*h {
		in.Notes = append(
			in.Notes,
			"over half the image is ink: it may be inverted, a photograph or a filled drawing, which aren't read",
		)
	}
	if o.Scale.Known() {
		in.Scale = o.Scale
	}
	r.read(&in)
	return in, nil
}

// reader holds one image's masks while Read works through it.
type reader struct {
	w, h  int
	ink   mask
	wall  mask
	inked int
	// pieces are all the wall bands found, kept or not.
	pieces []piece
}

func (r *reader) read(in *floorgraph.Input) {
	dt := sqDist(r.w, r.h, invert(r.ink.px), true)
	bd, ok := wallBand(r.w, r.h, r.ink.px, dt)
	if !ok {
		in.Notes = append(
			in.Notes,
			"no walls found: the plan has no long strokes thicker than a hairline; walls drawn as two thin lines aren't read yet",
		)
		r.wall = mask{w: r.w, h: r.h, px: make([]bool, r.w*r.h)}
		in.Coverage = r.coverage(nil, 0)
		return
	}
	r.wall = mask{w: r.w, h: r.h, px: open(r.w, r.h, dt, bd.lo)}
	thinnest := 2*bd.lo - 1
	var ps []piece
	for _, ax := range [2]axis{horizontal, vertical} {
		ps = append(ps, r.wall.pieces(ax, thinnest)...)
	}
	ps = merge(ps)
	// Pieces too short to stand as walls (mullions, the stub between two
	// doors, a corner seen end-on) still show in the wall mask, where they
	// serve as jambs for the openings either side. Pieces thinner than the
	// thinnest wall are slivers: a hairline lying along a wall face thickens
	// the mask there by a line, which stacks as a band of its own.
	ps = slices.DeleteFunc(ps, func(p piece) bool { return p.thick() < thinnest })
	r.pieces = ps
	kept := slices.DeleteFunc(
		slices.Clone(ps),
		func(p piece) bool { return p.length() < 2*thinnest },
	)
	kept = unslice(kept)
	tRef := refThickness(kept)
	lim := r.limits(in.Scale, tRef)
	var ops []floorgraph.Opening
	in.Walls = segments(kept, thinnest)
	for _, l := range lines(kept) {
		o, stubs := r.openings(l, lim[l.ax])
		ops = append(ops, o...)
		in.Walls = append(in.Walls, stubs...)
	}
	in.Openings = ops
	in.Coverage = r.coverage(ops, thinnest)
	in.Notes = append(in.Notes, fmt.Sprintf(
		"walls read as axis-aligned bands %d–%d px thick; slanted and curved walls aren't read yet",
		thinnest, 2*bd.hi))
}

// unslice drops pieces that lie mostly inside a bigger perpendicular one.
// Banding cuts a blob that is neither long nor thin (a column with the pier
// above it) into strips along both axes; the strips are slices of the one
// wall, not walls crossing it. Where real walls cross, the shared square is
// a small part of each, since a kept piece is at least twice as long as the
// thinnest wall is thick.
func unslice(ps []piece) []piece {
	out := make([]piece, 0, len(ps))
	for _, p := range ps {
		inside := false
		for _, q := range ps {
			if q.ax == p.ax || area(q) <= area(p) {
				continue
			}
			if 5*overlap(p, q) > 3*area(p) {
				inside = true
				break
			}
		}
		if !inside {
			out = append(out, p)
		}
	}
	return out
}

// rect is a piece's rectangle as x0, y0, x1, y1, inclusive.
func (p piece) rect() [4]int {
	if p.ax == horizontal {
		return [4]int{p.lo, p.a0, p.hi, p.a1}
	}
	return [4]int{p.a0, p.lo, p.a1, p.hi}
}

func area(p piece) int { return p.length() * p.thick() }

func overlap(p, q piece) int {
	a, b := p.rect(), q.rect()
	w := min(a[2], b[2]) - max(a[0], b[0]) + 1
	h := min(a[3], b[3]) - max(a[1], b[1]) + 1
	if w <= 0 || h <= 0 {
		return 0
	}
	return w * h
}

func invert(px []bool) []bool {
	out := make([]bool, len(px))
	for i, v := range px {
		out[i] = !v
	}
	return out
}

// refThickness is the length-weighted median wall thickness: the plan's
// ordinary partition, which outnumbers the exterior walls and columns.
func refThickness(ps []piece) int {
	type tw struct{ t, w int }
	v := make([]tw, 0, len(ps))
	total := 0
	for _, p := range ps {
		v = append(v, tw{p.thick(), p.length()})
		total += p.length()
	}
	slices.SortFunc(v, func(a, b tw) int { return a.t - b.t })
	acc := 0
	for _, x := range v {
		acc += x.w
		if 2*acc >= total {
			return x.t
		}
	}
	return 0
}

// limits sizes the openings looked for. With a scale they are the sizes
// buildings have: doors and passages 0.5–2.2 m, windows from 0.3 m. Without
// one, the ordinary partition stands in for a ruler: partitions are drawn
// about 0.1–0.25 m thick, so a passage of 0.5–2.2 m is 3–22 of them, and a
// window at least 2.
func (r *reader) limits(s floorgraph.Scale, tRef int) [2]gapLimits {
	var out [2]gapLimits
	for ax := range out {
		if s.Known() {
			along, across := s.PxPerMetre[ax], s.PxPerMetre[1-ax]
			out[ax] = gapLimits{
				minGap:    0.5 * along,
				maxGap:    2.2 * along,
				minWindow: 0.3 * along,
				squashes:  []float64{across / along},
			}
			continue
		}
		t := float64(tRef)
		out[ax] = gapLimits{
			minGap:    3 * t,
			maxGap:    22 * t,
			minWindow: 2 * t,
			squashes:  []float64{1, 0.87, 1.15},
		}
	}
	return out
}

// segments turns pieces into centrelines. An end buried in a perpendicular
// wall is pulled to that wall's centreline, where a hand trace puts the
// corner, rather than left at its outer face.
func segments(ps []piece, thinnest int) []floorgraph.Segment {
	out := make([]floorgraph.Segment, 0, len(ps))
	for _, p := range ps {
		s0, s1 := float64(p.lo), float64(p.hi+1)
		for _, q := range ps {
			if q.ax == p.ax || p.mid() < float64(q.lo-1) || p.mid() > float64(q.hi+2) {
				continue
			}
			if s0 >= float64(q.a0-1) && s0 <= float64(q.a1+2) {
				s0 = q.mid()
			}
			if s1 >= float64(q.a0-1) && s1 <= float64(q.a1+2) {
				s1 = q.mid()
			}
		}
		l := wallLine{ax: p.ax, mid: p.mid()}
		// Long, solid bands are walls; short ones may be a thick glyph or a
		// symbol that survived the width cut.
		weight := p.solid * min(1, float64(p.length())/float64(6*thinnest))
		out = append(out, floorgraph.Segment{
			A:         l.point(s0, 0),
			B:         l.point(s1, 0),
			Thickness: float64(p.thick()),
			Weight:    weight,
		})
	}
	return out
}
