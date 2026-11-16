package raster

import (
	"image"
	"image/color"
	"image/draw"
	"math"
)

// Point is a position in source pixels.
type Point = [2]float64

// Shape is one thing to draw over a source image: a polyline (Closed for a
// polygon outline) of the given width, and an optional label at Text's anchor.
type Shape struct {
	Points  []Point
	Closed  bool
	Width   float64
	Color   color.RGBA
	Opacity float64
	Text    string
	At      Point // label anchor; the label is centred on it
}

// Overlay draws shapes over a faded copy of the source, the same picture
// tools/overlay.mjs makes, so tracing errors show as colour off the ink.
func Overlay(src image.Image, shapes []Shape) *image.RGBA {
	b := src.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), image.NewUniform(white), image.Point{}, draw.Src)
	draw.Draw(out, out.Bounds(), src, b.Min, draw.Over)
	// Fade the source so the trace reads on top of dense line work.
	fade := image.NewUniform(color.NRGBA{R: 255, G: 255, B: 255, A: 115})
	draw.Draw(out, out.Bounds(), fade, image.Point{}, draw.Over)

	for _, s := range shapes {
		a := s.Opacity
		if a == 0 {
			a = 0.8
		}
		n := len(s.Points)
		for i := 0; i+1 < n; i++ {
			stroke(out, s.Points[i], s.Points[i+1], s.Width, s.Color, a)
		}
		if s.Closed && n > 2 {
			stroke(out, s.Points[n-1], s.Points[0], s.Width, s.Color, a)
		}
	}
	// Labels last so lines never cross them out.
	for _, s := range shapes {
		if s.Text != "" {
			w := 7 * len(s.Text) // basicfont advance
			labelColor(out, int(s.At[0])-w/2, int(s.At[1])+4, s.Text, s.Color)
		}
	}
	return out
}

// stroke draws a thick segment by filling every pixel within width/2 of it.
// Each pixel is painted once per segment, so overlapping walls don't darken.
func stroke(img *image.RGBA, p, q Point, width float64, c color.RGBA, a float64) {
	half := math.Max(width, 1) / 2
	x0 := int(math.Floor(math.Min(p[0], q[0]) - half))
	x1 := int(math.Ceil(math.Max(p[0], q[0]) + half))
	y0 := int(math.Floor(math.Min(p[1], q[1]) - half))
	y1 := int(math.Ceil(math.Max(p[1], q[1]) + half))
	r := image.Rect(x0, y0, x1+1, y1+1).Intersect(img.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if distToSegment(float64(x)+0.5, float64(y)+0.5, p, q) <= half {
				blend(img, x, y, c, a)
			}
		}
	}
}

func distToSegment(x, y float64, p, q Point) float64 {
	dx, dy := q[0]-p[0], q[1]-p[1]
	l2 := dx*dx + dy*dy
	t := 0.0
	if l2 > 0 {
		t = math.Max(0, math.Min(1, ((x-p[0])*dx+(y-p[1])*dy)/l2))
	}
	return math.Hypot(x-(p[0]+t*dx), y-(p[1]+t*dy))
}

// Fit scales img down so its longest side is at most edge, for sending to
// the model; smaller images are returned as they are.
func Fit(img image.Image, edge int) (image.Image, float64) {
	b := img.Bounds()
	long := max(b.Dx(), b.Dy())
	if long <= edge {
		return img, 1
	}
	scale := float64(edge) / float64(long)
	out := image.NewRGBA(image.Rect(0, 0, int(float64(b.Dx())*scale), int(float64(b.Dy())*scale)))
	scaleDown(out, img)
	return out, scale
}
