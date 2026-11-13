package plan

import (
	"image/color"
	"math"

	"github.com/weaveplatform/spaceweave/internal/raster"
)

// Colours match tools/overlay.mjs so both overlays read the same.
var (
	wallColour = map[string]color.RGBA{
		"exterior": {214, 39, 40, 255},
		"interior": {31, 95, 214, 255},
		"glass":    {23, 190, 207, 255},
	}
	openingColour   = color.RGBA{44, 160, 44, 255}
	roomColour      = color.RGBA{123, 63, 228, 255}
	furnitureColour = color.RGBA{255, 127, 14, 255}
	outlineColour   = color.RGBA{0, 0, 0, 255}
)

// footprint is the size drawn for furniture whose shape isn't given by w and
// d: enough to show where it is, which is all the overlay is for.
var footprint = map[string][2]float64{
	"chair": {0.5, 0.5}, "armchair": {0.6, 0.6}, "tub": {0.64, 0.64}, "stool": {0.36, 0.36},
	"toilet": {0.4, 0.7}, "basin": {0.5, 0.4}, "sofa": {0.85, 0},
}

// Shapes draws a trace in source pixels for raster.Overlay.
func Shapes(t *Trace) []raster.Shape {
	sx, sy := t.Calibration.PxPerMetre[0], t.Calibration.PxPerMetre[1]
	if !(sx > 0 && sy > 0) {
		sx, sy = 1, 1 // still show where things are; Check reports the scale
	}
	k := (sx + sy) / 2
	thickness := map[string]float64{
		"exterior": t.Thickness.Exterior,
		"interior": t.Thickness.Interior,
		"glass":    t.Thickness.Glass,
	}

	var out []raster.Shape
	for _, r := range t.Rooms {
		at := centroid(r.Polygon)
		if r.Label != nil {
			at = *r.Label
		}
		out = append(
			out,
			raster.Shape{
				Points:  r.Polygon,
				Closed:  true,
				Width:   1.5,
				Color:   roomColour,
				Opacity: 0.6,
				Text:    r.Name,
				At:      at,
			},
		)
	}
	for _, w := range t.Walls {
		th := w.Thickness
		if th == 0 {
			th = thickness[w.Kind]
		}
		c, ok := wallColour[w.Kind]
		if !ok {
			c = outlineColour
		}
		width := math.Max(2, th*k)
		out = append(
			out,
			raster.Shape{
				Points:  []raster.Point{w.From, w.To},
				Width:   width,
				Color:   c,
				Opacity: 0.7,
			},
		)
		axis := dominant(w.From, w.To)
		span := w.To[axis] - w.From[axis]
		for _, o := range w.Openings {
			if span == 0 {
				continue
			}
			at := func(v float64) raster.Point {
				f := (v - w.From[axis]) / span
				return raster.Point{
					w.From[0] + f*(w.To[0]-w.From[0]),
					w.From[1] + f*(w.To[1]-w.From[1]),
				}
			}
			out = append(
				out,
				raster.Shape{
					Points:  []raster.Point{at(o.A), at(o.B)},
					Width:   width + 3,
					Color:   openingColour,
					Opacity: 0.9,
				},
			)
		}
	}
	for _, it := range t.Furniture {
		out = append(out, itemShape(it, sx, sy))
	}
	if len(t.Outline) > 2 {
		out = append(
			out,
			raster.Shape{
				Points:  t.Outline,
				Closed:  true,
				Width:   1.5,
				Color:   outlineColour,
				Opacity: 0.6,
			},
		)
	}
	return out
}

// itemShape is a furniture footprint: a rectangle of the item's size in
// metres, turned by its angle on the ground, mapped back into pixels.
func itemShape(it Item, sx, sy float64) raster.Shape {
	val := func(v *float64, def float64) float64 {
		if v != nil {
			return *v
		}
		return def
	}
	w, d := val(it.W, 0.5), val(it.D, 0.5)
	if it.R != nil {
		w, d = 2**it.R, 2**it.R
	}
	if f, ok := footprint[it.Type]; ok {
		w, d = f[0], val(it.W, f[1])
		if it.Size != nil {
			w, d = *it.Size, *it.Size
		}
	}
	s := scale{sx: sx, sy: sy}
	deg := val(it.Rot, val(it.Facing, 0))
	r := s.angle(deg) * math.Pi / 180
	c, n := math.Cos(r), math.Sin(r)
	pts := make([]raster.Point, 0, 4)
	for _, corner := range [][2]float64{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
		u, v := corner[0]*w/2, corner[1]*d/2
		mx, my := u*c-v*n, u*n+v*c // metres, rotated on the ground
		pts = append(pts, raster.Point{it.X + mx*sx, it.Y + my*sy})
	}
	return raster.Shape{
		Points:  pts,
		Closed:  true,
		Width:   1.5,
		Color:   furnitureColour,
		Opacity: 0.8,
		Text:    it.Label,
		At:      raster.Point{it.X, it.Y},
	}
}

func centroid(ps [][2]float64) [2]float64 {
	var c [2]float64
	for _, p := range ps {
		c[0] += p[0]
		c[1] += p[1]
	}
	if n := float64(len(ps)); n > 0 {
		c[0] /= n
		c[1] /= n
	}
	return c
}
