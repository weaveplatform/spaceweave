// Package synth draws a trace back out as the kind of black-on-white floor
// plan an architect would publish, at the trace's own pixel coordinates. The
// result is a test image whose right answer is known exactly, so a tracer can
// be scored against the trace it was drawn from.
package synth

import (
	"bytes"
	"image"
	"image/draw"
	"image/jpeg"
	"math"
	"math/rand/v2"
	"sort"

	xvector "golang.org/x/image/vector"

	"github.com/weaveplatform/spaceweave/internal/plan"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// Options choose what is drawn besides walls and openings, and how cleanly.
// The zero value draws walls and openings only, with hard pixel edges.
type Options struct {
	// Size is the image size; zero sizes it to the trace plus Margin, keeping
	// the origin so the trace's coordinates stay valid.
	Size image.Point
	// Margin is the white border past the trace's extent, px; 40 when 0.
	Margin int
	// Text draws each room's name at its label point or centroid.
	Text bool
	// TextHeight is the cap height of room names, metres; 0.22 when 0.
	TextHeight float64
	// Furniture outlines every furniture item in thin ink.
	Furniture bool
	// AntiAlias keeps the rasteriser's grey edges; off, every pixel is black
	// or white.
	AntiAlias bool
	// Noise adds up to ±Noise to every pixel, from Seed.
	Noise uint8
	Seed  uint64
	// JPEG round-trips the image through JPEG at this quality (1–100) for
	// the ringing a scanned or downloaded plan has; 0 skips it.
	JPEG int
	// WallScale multiplies every wall's thickness, for the thinner and
	// bolder pens drawings use; 1 when 0.
	WallScale float64
	// Walls is how wall bodies are drawn.
	Walls WallStyle
}

// WallStyle is how a drawing inks its walls.
type WallStyle uint8

// Wall styles. Architects' drawings mostly outline walls, often with a
// hatch or a fill (poché) between the lines; presentation plans fill them.
const (
	// Solid fills each wall black.
	Solid WallStyle = iota
	// Outline draws only the edge of the walls' union: two hairlines a
	// wall's thickness apart, closed across its ends at every opening.
	Outline
	// Hatch outlines the walls and fills them with 45° hairlines
	// HatchPitch px apart.
	Hatch
)

// HatchPitch is the spacing of Hatch's lines, px.
const HatchPitch = 6

// LineWidth is the width of every thin line (door leaves, arcs, glazing,
// furniture), px. Plans draw these as hairlines; one pixel is the thinnest
// line that survives anti-aliasing.
const LineWidth = 1.0

// Render draws t as a floor plan.
func Render(t *plan.Trace, o Options) *image.Gray {
	r := newRenderer(t, o)
	for _, w := range t.Walls {
		r.wall(w)
	}
	r.styleWalls()
	if o.Furniture {
		for _, it := range t.Furniture {
			r.item(it)
		}
	}
	if o.Text {
		for _, room := range t.Rooms {
			r.label(room)
		}
	}
	return r.finish()
}

type pt = vector.Point

type renderer struct {
	o   Options
	t   *plan.Trace
	ppm [2]float64
	// mask is the ink; walls holds the wall bodies until styleWalls inks
	// them, so an outline follows the walls' union rather than each wall.
	mask, walls *image.Alpha
}

func newRenderer(t *plan.Trace, o Options) *renderer {
	if o.Margin <= 0 {
		o.Margin = 40
	}
	if o.TextHeight <= 0 {
		o.TextHeight = 0.22
	}
	if o.WallScale <= 0 {
		o.WallScale = 1
	}
	ppm := t.Calibration.PxPerMetre
	for i := range ppm {
		if ppm[i] <= 0 {
			// An uncalibrated trace still renders; 40 px/m is a typical
			// published plan.
			ppm[i] = 40
		}
	}
	size := o.Size
	if size.X <= 0 || size.Y <= 0 {
		size = extent(t, o.Margin)
	}
	rect := image.Rect(0, 0, size.X, size.Y)
	return &renderer{o: o, t: t, ppm: ppm, mask: image.NewAlpha(rect), walls: image.NewAlpha(rect)}
}

// extent is the smallest image from the origin that holds every wall and
// room corner plus the margin.
func extent(t *plan.Trace, margin int) image.Point {
	var mx, my float64
	see := func(p [2]float64) {
		mx, my = math.Max(mx, p[0]), math.Max(my, p[1])
	}
	for _, w := range t.Walls {
		see(w.From)
		see(w.To)
	}
	for _, r := range t.Rooms {
		for _, p := range r.Polygon {
			see(p)
		}
	}
	return image.Point{int(math.Ceil(mx)) + margin, int(math.Ceil(my)) + margin}
}

// px turns a vector in metres into pixels; the two axes may differ on a
// squashed image.
func (r *renderer) px(m pt) pt { return pt{X: m.X * r.ppm[0], Y: m.Y * r.ppm[1]} }

func (r *renderer) thickness(w plan.Wall) float64 {
	th := r.t.Thickness.Interior
	switch {
	case w.Thickness > 0:
		th = w.Thickness
	case w.Kind == "exterior":
		th = r.t.Thickness.Exterior
	case w.Kind == "glass":
		th = r.t.Thickness.Glass
	}
	return th * r.o.WallScale
}

// frame is a wall's geometry in metres: the start in pixels, the unit
// direction and normal in metres, and the length in metres. Working in
// metres keeps a wall's thickness and a door's leaf true on a squashed image.
type frame struct {
	a    pt
	u, n pt
	len  float64
	r    *renderer
}

func (f frame) at(s, off float64) pt { return f.a.Add(f.r.px(f.u.Mul(s).Add(f.n.Mul(off)))) }

func (r *renderer) frameOf(w plan.Wall) (frame, bool) {
	a, b := pt{X: w.From[0], Y: w.From[1]}, pt{X: w.To[0], Y: w.To[1]}
	d := pt{X: (b.X - a.X) / r.ppm[0], Y: (b.Y - a.Y) / r.ppm[1]}
	l := d.Len()
	if l == 0 {
		return frame{}, false
	}
	u := d.Mul(1 / l)
	return frame{a: a, u: u, n: pt{X: -u.Y, Y: u.X}, len: l, r: r}, true
}

// along converts an opening coordinate, which is in pixels on the wall's
// dominant axis, to metres along the wall.
func along(w plan.Wall, f frame, v float64) float64 {
	dx, dy := w.To[0]-w.From[0], w.To[1]-w.From[1]
	if math.Abs(dx) >= math.Abs(dy) {
		return (v - w.From[0]) / dx * f.len
	}
	return (v - w.From[1]) / dy * f.len
}

func (r *renderer) wall(w plan.Wall) {
	f, ok := r.frameOf(w)
	if !ok {
		return
	}
	th := r.thickness(w)
	ext := th / 2
	if w.Ext != nil {
		ext = *w.Ext
	}
	type span struct{ s0, s1 float64 }
	ops := make([]span, 0, len(w.Openings))
	for _, o := range w.Openings {
		s0, s1 := along(w, f, o.A), along(w, f, o.B)
		if s0 > s1 {
			s0, s1 = s1, s0
		}
		ops = append(ops, span{s0, s1})
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i].s0 < ops[j].s0 })
	from := -ext
	for _, o := range ops {
		r.band(f, from, o.s0, th)
		from = math.Max(from, o.s1)
	}
	r.band(f, from, f.len+ext, th)
	for _, o := range w.Openings {
		r.opening(w, f, o, th)
	}
}

func (r *renderer) band(f frame, s0, s1, th float64) {
	if s1 <= s0 {
		return
	}
	h := th / 2
	r.fillInto(r.walls, [][]pt{{f.at(s0, -h), f.at(s1, -h), f.at(s1, h), f.at(s0, h)}}, true)
}

// styleWalls inks the wall bodies in the chosen style. An outline pixel is
// a wall pixel with paper beside it, so walls that meet share one outline
// and an opening's jambs are closed across.
func (r *renderer) styleWalls() {
	b := r.walls.Rect
	on := func(x, y int) bool {
		return image.Pt(x, y).In(b) && r.walls.Pix[r.walls.PixOffset(x, y)] >= 128
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			i := r.walls.PixOffset(x, y)
			a := r.walls.Pix[i]
			if r.o.Walls != Solid {
				edge := on(x, y) && (!on(x-1, y) || !on(x+1, y) || !on(x, y-1) || !on(x, y+1))
				hatch := r.o.Walls == Hatch && on(x, y) && (x+y)%HatchPitch < 2
				a = 0
				if edge || hatch {
					a = 255
				}
			}
			r.mask.Pix[i] = max(r.mask.Pix[i], a)
		}
	}
}

var swings = map[string]pt{"N": {Y: -1}, "S": {Y: 1}, "E": {X: 1}, "W": {X: -1}}

func (r *renderer) opening(w plan.Wall, f frame, o plan.Opening, th float64) {
	// A and B are image coordinates, so on a wall drawn right to left or
	// bottom to top A is the far end; hinges follow A and B, not the wall.
	sa, sb := along(w, f, o.A), along(w, f, o.B)
	side := 1.0
	if v, ok := swings[o.Swing]; ok && v.X*f.n.X+v.Y*f.n.Y < 0 {
		side = -1
	}
	switch o.Kind {
	case "door":
		hinge, free := sa, sb
		if o.Hinge == "b" {
			hinge, free = sb, sa
		}
		r.door(f, hinge, math.Copysign(1, free-hinge), math.Abs(sb-sa), side)
	case "double-door":
		half := math.Abs(sb-sa) / 2
		r.door(f, sa, math.Copysign(1, sb-sa), half, side)
		r.door(f, sb, math.Copysign(1, sa-sb), half, side)
	case "window":
		// Glazing lines just inside the wall faces, and a middle pane line
		// when the wall is thick enough for three lines to stay apart.
		across := r.px(f.n).Len()
		inset := th/2 - LineWidth/across
		offs := []float64{-inset, inset}
		if th*across >= 7 {
			offs = append(offs, 0)
		}
		for _, off := range offs {
			r.stroke([]pt{f.at(sa, off), f.at(sb, off)})
		}
	}
}

// door draws a leaf standing open at right angles to the wall from the hinge
// at s, and the quarter circle its free end sweeps back to the closed
// position. dir is +1 when the gap lies at increasing s from the hinge.
func (r *renderer) door(f frame, s, dir, width, side float64) {
	r.stroke([]pt{f.at(s, 0), f.at(s, side*width)})
	const n = 32
	arc := make([]pt, 0, n+1)
	for i := range n + 1 {
		th := float64(i) / n * math.Pi / 2
		arc = append(arc, f.at(s+dir*width*math.Cos(th), side*width*math.Sin(th)))
	}
	r.stroke(arc)
}

func (r *renderer) item(it plan.Item) {
	c := pt{X: it.X, Y: it.Y}
	rot := 0.0
	if it.Rot != nil {
		rot = *it.Rot * math.Pi / 180
	}
	if it.R != nil {
		const n = 48
		ring := make([]pt, 0, n+1)
		for i := range n + 1 {
			a := float64(i) / n * 2 * math.Pi
			ring = append(ring, c.Add(r.px(pt{X: math.Cos(a), Y: math.Sin(a)}.Mul(*it.R))))
		}
		r.stroke(ring)
		return
	}
	w, d := 0.5, 0.5
	if it.Size != nil {
		w, d = *it.Size, *it.Size
	}
	if it.W != nil && it.D != nil {
		w, d = *it.W, *it.D
	}
	sn, cs := math.Sincos(rot)
	corner := func(x, y float64) pt {
		return c.Add(r.px(pt{X: x*cs - y*sn, Y: x*sn + y*cs}))
	}
	r.stroke([]pt{
		corner(
			-w/2,
			-d/2,
		),
		corner(w/2, -d/2),
		corner(w/2, d/2),
		corner(-w/2, d/2),
		corner(-w/2, -d/2),
	})
}

func (r *renderer) label(room plan.Room) {
	var at pt
	switch {
	case room.Label != nil:
		at = pt{X: room.Label[0], Y: room.Label[1]}
	case len(room.Polygon) > 0:
		at = centroid(room.Polygon)
	default:
		return
	}
	// vector lays text out with y up, as a CAD drawing is; flip it in and out.
	t := vector.Text{
		At:      pt{X: at.X, Y: -at.Y},
		Height:  r.o.TextHeight * r.ppm[1],
		Align:   vector.Center,
		VAlign:  vector.Middle,
		Content: room.Name,
	}
	out := t.Outline()
	if out == nil {
		return
	}
	p, _ := out.Transformed(vector.Scale(1, -1)).(*vector.Path)
	r.glyphs(p)
}

// centroid is the polygon's area centroid, or its vertex mean when it has
// no area.
func centroid(poly [][2]float64) pt {
	var a, cx, cy, mx, my float64
	for i, p := range poly {
		q := poly[(i+1)%len(poly)]
		k := p[0]*q[1] - q[0]*p[1]
		a += k
		cx += (p[0] + q[0]) * k
		cy += (p[1] + q[1]) * k
		mx += p[0]
		my += p[1]
	}
	if math.Abs(a) < 1e-9 {
		n := float64(len(poly))
		return pt{X: mx / n, Y: my / n}
	}
	return pt{X: cx / (3 * a), Y: cy / (3 * a)}
}

// stroke draws a polyline LineWidth wide as one quad per segment, each
// running half a width past its ends so joints don't notch.
func (r *renderer) stroke(pts []pt) {
	hw := LineWidth / 2
	quads := make([][]pt, 0, len(pts))
	for i := 1; i < len(pts); i++ {
		a, b := pts[i-1], pts[i]
		d := b.Sub(a)
		l := d.Len()
		if l == 0 {
			continue
		}
		u := d.Mul(hw / l)
		n := pt{X: -u.Y, Y: u.X}
		a, b = a.Sub(u), b.Add(u)
		quads = append(quads, []pt{a.Add(n), b.Add(n), b.Sub(n), a.Sub(n)})
	}
	r.fill(quads, true)
}

// fill paints polygons into the ink mask. The rasteriser sums signed
// coverage, so overlapping shapes wound opposite ways would cancel to
// white; orient makes every ring wind the same way. Glyphs skip it because
// their holes rely on the opposite winding.
func (r *renderer) fill(rings [][]pt, orient bool) { r.fillInto(r.mask, rings, orient) }

func (r *renderer) fillInto(dst *image.Alpha, rings [][]pt, orient bool) {
	bb := image.Rectangle{}
	first := true
	for _, ring := range rings {
		for _, p := range ring {
			q := image.Rect(
				int(math.Floor(p.X)),
				int(math.Floor(p.Y)),
				int(math.Ceil(p.X))+1,
				int(math.Ceil(p.Y))+1,
			)
			if first {
				bb, first = q, false
			} else {
				bb = bb.Union(q)
			}
		}
	}
	bb = bb.Intersect(dst.Rect)
	if bb.Empty() {
		return
	}
	z := xvector.NewRasterizer(bb.Dx(), bb.Dy())
	ox, oy := float64(bb.Min.X), float64(bb.Min.Y)
	for _, ring := range rings {
		if orient && area(ring) < 0 {
			ring = reversed(ring)
		}
		for i, p := range ring {
			x, y := float32(p.X-ox), float32(p.Y-oy)
			if i == 0 {
				z.MoveTo(x, y)
			} else {
				z.LineTo(x, y)
			}
		}
		z.ClosePath()
	}
	z.Draw(dst, bb, image.Opaque, image.Point{})
}

// glyphs fills a text outline, curves and all, in one pass so the counters
// of letters like "o" stay open.
func (r *renderer) glyphs(p *vector.Path) {
	b := p.Bounds()
	bb := image.Rect(
		int(
			math.Floor(b.Min.X),
		),
		int(math.Floor(b.Min.Y)),
		int(math.Ceil(b.Max.X))+1,
		int(math.Ceil(b.Max.Y))+1,
	).Intersect(r.mask.Rect)
	if bb.Empty() {
		return
	}
	z := xvector.NewRasterizer(bb.Dx(), bb.Dy())
	ox, oy := b2f(bb.Min.X), b2f(bb.Min.Y)
	for _, s := range p.Segs {
		switch s.Op {
		case vector.MoveTo:
			z.MoveTo(f32(s.P[0].X)-ox, f32(s.P[0].Y)-oy)
		case vector.LineTo:
			z.LineTo(f32(s.P[0].X)-ox, f32(s.P[0].Y)-oy)
		case vector.CubicTo:
			z.CubeTo(
				f32(s.P[0].X)-ox, f32(s.P[0].Y)-oy,
				f32(s.P[1].X)-ox, f32(s.P[1].Y)-oy,
				f32(s.P[2].X)-ox, f32(s.P[2].Y)-oy,
			)
		case vector.Close:
			z.ClosePath()
		}
	}
	z.Draw(r.mask, bb, image.Opaque, image.Point{})
}

func f32(v float64) float32 { return float32(v) }
func b2f(v int) float32     { return float32(v) }

func area(ring []pt) float64 {
	var a float64
	for i, p := range ring {
		q := ring[(i+1)%len(ring)]
		a += p.X*q.Y - q.X*p.Y
	}
	return a
}

func reversed(ring []pt) []pt {
	out := make([]pt, len(ring))
	for i, p := range ring {
		out[len(ring)-1-i] = p
	}
	return out
}

func (r *renderer) finish() *image.Gray {
	g := image.NewGray(r.mask.Rect)
	for i, a := range r.mask.Pix {
		if !r.o.AntiAlias {
			// Half coverage is the fairest hard edge: a 3.4 px wall comes
			// out 3 or 4 px, as a plotter would draw it.
			if a >= 128 {
				a = 255
			} else {
				a = 0
			}
		}
		g.Pix[i] = 255 - a
	}
	if r.o.Noise > 0 {
		// Test images, not secrets: the noise must repeat from the seed.
		src := rand.NewPCG(r.o.Seed, r.o.Seed^0x9e3779b97f4a7c15)
		rng := rand.New(src) //nolint:gosec // see above
		amp := int(r.o.Noise)
		for i, v := range g.Pix {
			n := int(v) + rng.IntN(2*amp+1) - amp
			g.Pix[i] = uint8(min(max(n, 0), 255)) //nolint:gosec // clamped
		}
	}
	if r.o.JPEG > 0 {
		g = jpegRoundTrip(g, min(r.o.JPEG, 100))
	}
	return g
}

func jpegRoundTrip(g *image.Gray, q int) *image.Gray {
	var buf bytes.Buffer
	// Encoding a valid in-memory image into a buffer cannot fail, and
	// neither can decoding what was just encoded; on the impossible error the
	// clean image is still a usable test input.
	if err := jpeg.Encode(&buf, g, &jpeg.Options{Quality: q}); err != nil {
		return g
	}
	img, err := jpeg.Decode(&buf)
	if err != nil {
		return g
	}
	out := image.NewGray(g.Rect)
	draw.Draw(out, out.Rect, img, img.Bounds().Min, draw.Src)
	return out
}
