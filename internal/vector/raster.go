package vector

import (
	"errors"
	"image"
	"image/color"
	"math"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
	"golang.org/x/image/vector"
)

// Rasterising defaults. 4096 px on the long edge keeps a 50 m floor at about
// 1.2 cm a pixel, finer than any wall the tracer measures, while staying well
// inside the model's image budget once cropped.
const (
	DefaultMaxSide = 4096
	DefaultMargin  = 24
	// DefaultMinStroke is the thinnest line drawn, in pixels. CAD lines are
	// hairlines; at 1 px anti-aliasing turns them light grey, and the
	// tracer's ink threshold can lose them.
	DefaultMinStroke = 1.5
)

// ErrEmpty is returned for a drawing with nothing visible in it.
var ErrEmpty = errors.New("the drawing has nothing in it")

// Options control rasterising.
type Options struct {
	MaxSide   int     // the long edge in pixels; DefaultMaxSide when 0
	MaxPixels int     // a cap on width × height; none when 0
	Margin    int     // white border in pixels; DefaultMargin when 0
	MinStroke float64 // DefaultMinStroke when 0
}

// Raster is a rasterised drawing.
type Raster struct {
	Image *image.Gray
	// PxPerUnit is the scale: pixels per drawing unit, on both axes.
	PxPerUnit float64
	// Bounds is the part of the drawing shown.
	Bounds Rect
	// Trimmed is how many items lay so far from the rest that they were left
	// out of Bounds.
	Trimmed int
	view    Affine
}

// ToPixel maps a drawing point to the image.
func (r *Raster) ToPixel(p Point) Point { return r.view.Apply(p) }

// View is the transform ToPixel applies, for callers that place drawing
// geometry on the image themselves.
func (r *Raster) View() Affine { return r.view }

// Rasterize draws d in black ink on white. Every stroke and every piece of
// text is black whatever its colour, because a yellow or cyan CAD layer would
// otherwise fall under the tracer's ink threshold; fills keep their lightness
// so a pale room fill doesn't read as a wall.
func Rasterize(d *Drawing, o Options) (*Raster, error) {
	o = o.withDefaults()
	bounds, trimmed := Extent(d.Items)
	if bounds.Empty() {
		return nil, ErrEmpty
	}
	s, w, h := fit(bounds, o)
	m := float64(o.Margin)
	view := Affine{A: s, D: -s, E: m - s*bounds.Min.X, F: m + s*bounds.Max.Y}
	canvas := image.NewGray(image.Rect(0, 0, w, h))
	for i := range canvas.Pix {
		canvas.Pix[i] = 0xff
	}
	r := &Raster{Image: canvas, PxPerUnit: s, Bounds: bounds, Trimmed: trimmed, view: view}

	ink := vector.NewRasterizer(w, h)
	inked := false
	for _, it := range d.Items {
		// Text is laid out in drawing space, where up is up, and only then
		// mapped into the image, whose Y axis points down.
		text, isText := it.(*Text)
		if isText {
			outline := text.Outline()
			if outline == nil {
				continue
			}
			it = outline
		}
		it = it.Transformed(view)
		if !overlaps(it.Bounds(), w, h) {
			continue
		}
		switch it := it.(type) {
		case *Image:
			drawImage(canvas, it)
		case *Path:
			if isText {
				addPolys(ink, normalise(flatten(it.Segs, tolerance)))
				inked = true
				continue
			}
			if it.Fill {
				fill(canvas, it)
			}
			if it.Stroke {
				stroke(ink, it, o.MinStroke)
				inked = true
			}
		}
	}
	if inked {
		mask := image.NewAlpha(canvas.Bounds())
		ink.Draw(mask, mask.Bounds(), image.Opaque, image.Point{})
		xdraw.DrawMask(
			canvas,
			canvas.Bounds(),
			image.Black,
			image.Point{},
			mask,
			image.Point{},
			xdraw.Over,
		)
	}
	return r, nil
}

func (o Options) withDefaults() Options {
	if o.MaxSide <= 0 {
		o.MaxSide = DefaultMaxSide
	}
	if o.Margin <= 0 {
		o.Margin = DefaultMargin
	}
	if o.MinStroke <= 0 {
		o.MinStroke = DefaultMinStroke
	}
	o.Margin = min(o.Margin, o.MaxSide/4)
	return o
}

// fit picks the scale and the image size.
func fit(b Rect, o Options) (s float64, w, h int) {
	size := b.Size()
	long := math.Max(size.X, size.Y)
	if long <= 0 {
		long = 1
	}
	avail := float64(o.MaxSide - 2*o.Margin)
	s = avail / long
	dims := func() (int, int) {
		return int(math.Ceil(size.X*s)) + 2*o.Margin, int(math.Ceil(size.Y*s)) + 2*o.Margin
	}
	w, h = dims()
	if o.MaxPixels > 0 && w*h > o.MaxPixels {
		// The margins don't shrink with the scale, so solve
		// (x·s + 2m)(y·s + 2m) = MaxPixels for s rather than scaling by
		// √(MaxPixels / w·h), which overshoots by the margins' share.
		m2, p := float64(2*o.Margin), float64(o.MaxPixels)
		a, b, c := size.X*size.Y, m2*(size.X+size.Y), m2*m2-p
		switch {
		case c >= 0: // the margins alone use up the cap
			s = 1e-9
		case a > 0:
			s = (-b + math.Sqrt(b*b-4*a*c)) / (2 * a)
		default: // a line: one side has no size
			s = -c / b
		}
		// Rounding each side up can still overshoot by a pixel's row.
		for i := 0; i < 64 && s > 1e-9; i++ {
			if w, h = dims(); w*h <= o.MaxPixels {
				break
			}
			s *= 0.99
		}
		w, h = dims()
		w, h = max(w, 1), max(h, 1)
	}
	return s, w, h
}

func overlaps(b Rect, w, h int) bool {
	return !b.Empty() && b.Max.X >= 0 && b.Max.Y >= 0 && b.Min.X <= float64(w) &&
		b.Min.Y <= float64(h)
}

// tolerance is how far, in pixels, a flattened curve may stray from the curve.
const tolerance = 0.25

// polyline is one flattened subpath.
type polyline struct {
	pts    []Point
	closed bool
}

// flatten turns segments into polylines, splitting each cubic into enough
// lines to stay within tol of it.
func flatten(segs []Seg, tol float64) []polyline {
	var out []polyline
	var cur *polyline
	pen := Point{}
	for _, s := range segs {
		switch s.Op {
		case MoveTo:
			out = append(out, polyline{pts: []Point{s.P[0]}})
			cur = &out[len(out)-1]
			pen = s.P[0]
		case LineTo, CubicTo:
			if cur == nil {
				out = append(out, polyline{pts: []Point{pen}})
				cur = &out[len(out)-1]
			}
			if s.Op == LineTo {
				cur.pts = append(cur.pts, s.P[0])
				pen = s.P[0]
				break
			}
			cur.pts = appendCubic(cur.pts, pen, s.P[0], s.P[1], s.P[2], tol)
			pen = s.P[2]
		case Close:
			if cur != nil {
				cur.closed = true
				pen = cur.pts[0]
				cur = nil
			}
		}
	}
	return out
}

func appendCubic(pts []Point, p0, p1, p2, p3 Point, tol float64) []Point {
	dd := math.Max(p0.Sub(p1.Mul(2)).Add(p2).Len(), p1.Sub(p2.Mul(2)).Add(p3).Len())
	n := int(math.Ceil(math.Sqrt(0.75 * dd / tol)))
	n = min(max(n, 1), 256)
	for i := 1; i <= n; i++ {
		t := float64(i) / float64(n)
		u := 1 - t
		pts = append(
			pts,
			p0.Mul(u*u*u).Add(p1.Mul(3*u*u*t)).Add(p2.Mul(3*u*t*t)).Add(p3.Mul(t*t*t)),
		)
	}
	return pts
}

// area is the signed area of a closed ring.
func area(pts []Point) float64 {
	a := 0.0
	for i, p := range pts {
		q := pts[(i+1)%len(pts)]
		a += p.X*q.Y - q.X*p.Y
	}
	return a / 2
}

func reversed(pts []Point) []Point {
	out := make([]Point, len(pts))
	for i, p := range pts {
		out[len(pts)-1-i] = p
	}
	return out
}

// positive returns the ring with a positive signed area.
func positive(pts []Point) []Point {
	if area(pts) < 0 {
		return reversed(pts)
	}
	return pts
}

// stroke adds a path's outline to the ink layer. Every piece is wound the
// same way, so where pieces overlap, x/image/vector's clamped winding sum
// makes a union rather than cancelling them out.
func stroke(z *vector.Rasterizer, p *Path, minWidth float64) {
	hw := math.Max(p.Width, minWidth) / 2
	for _, pl := range flatten(p.Segs, tolerance) {
		pts := pl.pts
		if pl.closed && len(pts) > 1 {
			pts = append(pts, pts[0])
		}
		for i := 1; i < len(pts); i++ {
			if q := quad(pts[i-1], pts[i], hw); q != nil {
				addRing(z, q)
			}
		}
		// Round joins and caps, where a line is thick enough for the corner
		// to show.
		if hw >= 1 {
			for _, c := range pts {
				addRing(z, disc(c, hw))
			}
		}
	}
}

func quad(a, b Point, hw float64) []Point {
	d := b.Sub(a)
	l := d.Len()
	if l < 1e-9 {
		return nil
	}
	n := Point{-d.Y, d.X}.Mul(hw / l)
	return positive([]Point{a.Add(n), b.Add(n), b.Sub(n), a.Sub(n)})
}

func disc(c Point, r float64) []Point {
	const sides = 12
	pts := make([]Point, sides)
	for i := range pts {
		s, co := math.Sincos(2 * math.Pi * float64(i) / sides)
		pts[i] = Point{c.X + r*co, c.Y + r*s}
	}
	return positive(pts)
}

func addRing(z *vector.Rasterizer, pts []Point) {
	if len(pts) < 3 {
		return
	}
	z.MoveTo(float32(pts[0].X), float32(pts[0].Y))
	for _, p := range pts[1:] {
		z.LineTo(float32(p.X), float32(p.Y))
	}
	z.ClosePath()
}

func addPolys(z *vector.Rasterizer, rings [][]Point) {
	for _, r := range rings {
		addRing(z, r)
	}
}

// maxNesting bounds the containment test, which is quadratic in subpaths.
const maxNesting = 400

// normalise winds a filled shape's rings for the even-odd rule: rings nested
// an even number of times positive, holes negative. CAD hatches and fonts
// wind their holes in no reliable direction, and the rasterizer only sees
// the winding sum.
func normalise(pls []polyline) [][]Point {
	rings := make([][]Point, 0, len(pls))
	for _, pl := range pls {
		if len(pl.pts) >= 3 {
			rings = append(rings, pl.pts)
		}
	}
	if len(rings) > maxNesting {
		for i, r := range rings {
			rings[i] = positive(r)
		}
		return rings
	}
	out := make([][]Point, len(rings))
	for i, r := range rings {
		depth := 0
		for j, o := range rings {
			if i != j && inside(r[0], o) {
				depth++
			}
		}
		out[i] = positive(r)
		if depth%2 == 1 {
			out[i] = reversed(out[i])
		}
	}
	return out
}

// inside is the even-odd point-in-polygon test.
func inside(p Point, ring []Point) bool {
	in := false
	for i, a := range ring {
		b := ring[(i+1)%len(ring)]
		if (a.Y > p.Y) != (b.Y > p.Y) && p.X < (b.X-a.X)*(p.Y-a.Y)/(b.Y-a.Y)+a.X {
			in = !in
		}
	}
	return in
}

// fillShade keeps a light fill light and makes a dark one ink.
func fillShade(c Color) uint8 {
	l := c.Luma()
	if l < 0.6 {
		l *= 0.4
	}
	return uint8(math.Round(l * 255))
}

// fill paints one filled path in its own shade. Each fill is rasterised in
// a rasterizer the size of its own box, so a thousand small fills cost what
// their area costs, not a thousand full images.
func fill(canvas *image.Gray, p *Path) {
	b := p.Bounds()
	r := image.Rect(int(math.Floor(b.Min.X)), int(math.Floor(b.Min.Y)),
		int(math.Ceil(b.Max.X))+1, int(math.Ceil(b.Max.Y))+1).Intersect(canvas.Bounds())
	if r.Empty() {
		return
	}
	z := vector.NewRasterizer(r.Dx(), r.Dy())
	off := Translate(-float64(r.Min.X), -float64(r.Min.Y))
	shifted, _ := p.Transformed(off).(*Path)
	addPolys(z, normalise(flatten(shifted.Segs, tolerance)))
	mask := image.NewAlpha(image.Rect(0, 0, r.Dx(), r.Dy()))
	z.Draw(mask, mask.Bounds(), image.Opaque, image.Point{})
	src := image.NewUniform(color.Gray{Y: fillShade(p.Color)})
	xdraw.DrawMask(canvas, r, src, image.Point{}, mask, image.Point{}, xdraw.Over)
}

func drawImage(canvas *image.Gray, im *Image) {
	// M places pixels counted from the image's corner, as Bounds and the
	// SVG writer count them; a sub-image's own pixels start elsewhere.
	o := im.Img.Bounds().Min
	m := Translate(-float64(o.X), -float64(o.Y)).Then(im.M)
	s2d := f64.Aff3{m.A, m.C, m.E, m.B, m.D, m.F}
	xdraw.BiLinear.Transform(canvas, s2d, im.Img, im.Img.Bounds(), xdraw.Over, nil)
}
