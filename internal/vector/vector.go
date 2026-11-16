// Package vector is a format-neutral line drawing: what the DWG, DXF, PDF and
// SVG readers produce, and what becomes the PNG the tracer reads and the SVG
// people look at.
//
// Coordinates are drawing units with Y pointing up, as in CAD and PDF; the
// SVG reader flips its input to match.
package vector

import (
	"image"
	"math"
)

// Point is a position in drawing units.
type Point struct{ X, Y float64 }

// Add returns p+q.
func (p Point) Add(q Point) Point { return Point{p.X + q.X, p.Y + q.Y} }

// Sub returns p-q.
func (p Point) Sub(q Point) Point { return Point{p.X - q.X, p.Y - q.Y} }

// Mul returns p scaled by k.
func (p Point) Mul(k float64) Point { return Point{p.X * k, p.Y * k} }

// Len is the distance from the origin.
func (p Point) Len() float64 { return math.Hypot(p.X, p.Y) }

// Rect is an axis-aligned box. The zero Rect is empty.
type Rect struct {
	Min, Max Point
	ok       bool
}

// Empty reports whether r holds no point.
func (r Rect) Empty() bool { return !r.ok }

// Extend grows r to hold p.
func (r Rect) Extend(p Point) Rect {
	if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) {
		return r
	}
	if !r.ok {
		return Rect{Min: p, Max: p, ok: true}
	}
	r.Min = Point{math.Min(r.Min.X, p.X), math.Min(r.Min.Y, p.Y)}
	r.Max = Point{math.Max(r.Max.X, p.X), math.Max(r.Max.Y, p.Y)}
	return r
}

// Union grows r to hold s.
func (r Rect) Union(s Rect) Rect {
	if !s.ok {
		return r
	}
	return r.Extend(s.Min).Extend(s.Max)
}

// Size is the box's width and height.
func (r Rect) Size() Point { return r.Max.Sub(r.Min) }

// Center is the middle of the box.
func (r Rect) Center() Point { return r.Min.Add(r.Max).Mul(0.5) }

// Affine maps x' = A*x + C*y + E, y' = B*x + D*y + F; the same order as
// SVG's matrix(a b c d e f).
type Affine struct{ A, B, C, D, E, F float64 }

// Identity is the transform that changes nothing.
var Identity = Affine{A: 1, D: 1}

// Translate moves by (x, y).
func Translate(x, y float64) Affine { return Affine{A: 1, D: 1, E: x, F: y} }

// Scale scales about the origin.
func Scale(sx, sy float64) Affine { return Affine{A: sx, D: sy} }

// Rotate turns counter-clockwise by rad about the origin.
func Rotate(rad float64) Affine {
	s, c := math.Sincos(rad)
	return Affine{A: c, B: s, C: -s, D: c}
}

// Then returns the transform that applies m and then n.
func (m Affine) Then(n Affine) Affine {
	return Affine{
		A: n.A*m.A + n.C*m.B,
		B: n.B*m.A + n.D*m.B,
		C: n.A*m.C + n.C*m.D,
		D: n.B*m.C + n.D*m.D,
		E: n.A*m.E + n.C*m.F + n.E,
		F: n.B*m.E + n.D*m.F + n.F,
	}
}

// Apply maps a point.
func (m Affine) Apply(p Point) Point {
	return Point{m.A*p.X + m.C*p.Y + m.E, m.B*p.X + m.D*p.Y + m.F}
}

// Factor is how much m scales lengths on average, for line widths and text
// heights under a non-uniform transform.
func (m Affine) Factor() float64 { return math.Sqrt(math.Abs(m.A*m.D - m.B*m.C)) }

// Angle is how far m turns the x axis, counter-clockwise.
func (m Affine) Angle() float64 { return math.Atan2(m.B, m.A) }

// Color is an sRGB colour.
type Color struct{ R, G, B uint8 }

// Black is the default ink.
var Black = Color{}

// Luma is the colour's perceived lightness, 0 to 1.
func (c Color) Luma() float64 {
	return (0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)) / 255
}

// Item is something drawn: a *Path, *Text or *Image.
type Item interface {
	// Bounds is the item's extent in drawing units.
	Bounds() Rect
	// Transformed returns a copy of the item under m.
	Transformed(m Affine) Item
}

// Op is a path segment's kind.
type Op uint8

// The segment kinds.
const (
	MoveTo Op = iota
	LineTo
	CubicTo
	Close
)

// Seg is one path segment. MoveTo and LineTo use P[0]; CubicTo uses the two
// control points P[0], P[1] and the end P[2].
type Seg struct {
	Op Op
	P  [3]Point
}

// Path is an outline that is stroked, filled or both, in one colour.
type Path struct {
	Segs   []Seg
	Stroke bool
	Fill   bool
	Color  Color
	// Width is the stroke width in drawing units; 0 is the thinnest line the
	// output can show.
	Width float64
}

// MoveTo starts a subpath.
func (p *Path) MoveTo(a Point) { p.Segs = append(p.Segs, Seg{Op: MoveTo, P: [3]Point{a}}) }

// LineTo adds a straight segment.
func (p *Path) LineTo(a Point) { p.Segs = append(p.Segs, Seg{Op: LineTo, P: [3]Point{a}}) }

// CubicTo adds a cubic Bézier segment.
func (p *Path) CubicTo(c1, c2, to Point) {
	p.Segs = append(p.Segs, Seg{Op: CubicTo, P: [3]Point{c1, c2, to}})
}

// Close ends the subpath with a line back to its start.
func (p *Path) Close() { p.Segs = append(p.Segs, Seg{Op: Close}) }

// Polyline adds a subpath through pts, closed if asked.
func (p *Path) Polyline(pts []Point, closed bool) {
	for i, q := range pts {
		if i == 0 {
			p.MoveTo(q)
		} else {
			p.LineTo(q)
		}
	}
	if closed && len(pts) > 0 {
		p.Close()
	}
}

// EllipseArc adds the part of an ellipse from parameter t0 to t1 (radians,
// counter-clockwise when t1 > t0). major is the semi-major axis as a vector
// from the centre; the minor axis is ratio times it, turned a quarter
// counter-clockwise. A circle has ratio 1. When join is set the arc continues
// the current subpath; otherwise it starts a new one.
func (p *Path) EllipseArc(c, major Point, ratio, t0, t1 float64, join bool) {
	minor := Point{-major.Y, major.X}.Mul(ratio)
	at := func(t float64) (Point, Point) {
		s, co := math.Sincos(t)
		return c.Add(major.Mul(co)).Add(minor.Mul(s)), major.Mul(-s).Add(minor.Mul(co))
	}
	n := int(math.Ceil(math.Abs(t1-t0) / (math.Pi / 2)))
	n = max(n, 1)
	step := (t1 - t0) / float64(n)
	k := 4.0 / 3 * math.Tan(step/4)
	start, _ := at(t0)
	if join && len(p.Segs) > 0 {
		p.LineTo(start)
	} else {
		p.MoveTo(start)
	}
	for i := range n {
		a, da := at(t0 + float64(i)*step)
		b, db := at(t0 + float64(i+1)*step)
		p.CubicTo(a.Add(da.Mul(k)), b.Sub(db.Mul(k)), b)
	}
}

// Circle adds a closed circle.
func (p *Path) Circle(c Point, r float64) {
	p.EllipseArc(c, Point{r, 0}, 1, 0, 2*math.Pi, false)
	p.Close()
}

// Bounds is the extent of the path's points, control points included, which
// is never smaller than the curve and usually close to it.
func (p *Path) Bounds() Rect {
	var r Rect
	for _, s := range p.Segs {
		switch s.Op {
		case MoveTo, LineTo:
			r = r.Extend(s.P[0])
		case CubicTo:
			r = r.Extend(s.P[0]).Extend(s.P[1]).Extend(s.P[2])
		case Close:
		}
	}
	return r
}

// Transformed implements Item.
func (p *Path) Transformed(m Affine) Item {
	q := *p
	q.Segs = make([]Seg, len(p.Segs))
	for i, s := range p.Segs {
		for j := range s.P {
			s.P[j] = m.Apply(s.P[j])
		}
		q.Segs[i] = s
	}
	q.Width = p.Width * m.Factor()
	return &q
}

// Align is where a text's anchor sits on the text.
type Align uint8

// Horizontal alignments.
const (
	Left Align = iota
	Center
	Right
)

// VAlign is where a text's anchor sits vertically.
type VAlign uint8

// Vertical alignments.
const (
	Baseline VAlign = iota
	Bottom
	Middle
	Top
)

// Text is a single line of text.
type Text struct {
	At      Point
	Height  float64 // cap height in drawing units
	Angle   float64 // radians, counter-clockwise
	Align   Align
	VAlign  VAlign
	Content string
	Color   Color
	// WidthFactor stretches the text along its baseline; 0 means 1.
	WidthFactor float64
}

// avgAdvance is a typical glyph advance as a fraction of the height, for
// extents before the font is laid out.
const avgAdvance = 0.6

func (t *Text) widthFactor() float64 {
	if t.WidthFactor <= 0 {
		return 1
	}
	return t.WidthFactor
}

// box is the text's corners before rotation, relative to its anchor.
func (t *Text) box(width float64) (x0, y0, x1, y1 float64) {
	h := t.Height
	switch t.Align {
	case Left:
		x0 = 0
	case Center:
		x0 = -width / 2
	case Right:
		x0 = -width
	}
	switch t.VAlign {
	case Baseline:
		y0 = -0.25 * h
	case Bottom:
		y0 = 0
	case Middle:
		y0 = -h / 2
	case Top:
		y0 = -h
	}
	return x0, y0, x0 + width, y0 + 1.25*h
}

// Bounds estimates the text's extent from its length.
func (t *Text) Bounds() Rect {
	n := float64(len([]rune(t.Content)))
	x0, y0, x1, y1 := t.box(n * avgAdvance * t.Height * t.widthFactor())
	m := Rotate(t.Angle).Then(Translate(t.At.X, t.At.Y))
	var r Rect
	for _, c := range []Point{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}} {
		r = r.Extend(m.Apply(c))
	}
	return r
}

// Transformed implements Item. A mirroring transform moves the text but
// doesn't mirror it, and leaves it the right way up: CAD keeps mirrored text
// readable (MIRRTEXT 0) unless told otherwise.
func (t *Text) Transformed(m Affine) Item {
	q := *t
	q.At = m.Apply(t.At)
	q.Height = t.Height * m.Factor()
	q.Angle = t.Angle + m.Angle()
	if m.A*m.D-m.B*m.C < 0 && math.Cos(q.Angle) < -1e-9 {
		q.Angle += math.Pi
	}
	return &q
}

// Image is an embedded raster, such as a scan inside a PDF. M maps its pixel
// coordinates (x right, y down, 0 to width and height) to drawing units.
type Image struct {
	Img image.Image
	M   Affine
}

// Bounds is where the image's corners land.
func (im *Image) Bounds() Rect {
	b := im.Img.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	var r Rect
	for _, c := range []Point{{0, 0}, {w, 0}, {w, h}, {0, h}} {
		r = r.Extend(im.M.Apply(c))
	}
	return r
}

// Transformed implements Item.
func (im *Image) Transformed(m Affine) Item {
	return &Image{Img: im.Img, M: im.M.Then(m)}
}

// Drawing is a whole drawing.
type Drawing struct {
	Items []Item
	// Unit is metres per drawing unit; 0 when the source doesn't say.
	Unit float64
	// Assumed means Unit was guessed rather than read from the file.
	Assumed bool
	// Paper means Unit is a paper size (PDF points, SVG millimetres), not
	// the building's: the drawing's scale on paper is still unknown.
	Paper bool
	// Notes are things a reader should know about the conversion.
	Notes []string
}

// Add appends items.
func (d *Drawing) Add(items ...Item) { d.Items = append(d.Items, items...) }

// Bounds is the extent of every item.
func (d *Drawing) Bounds() Rect {
	var r Rect
	for _, it := range d.Items {
		r = r.Union(it.Bounds())
	}
	return r
}
