package vector

import (
	"image"
	"math"
	"testing"
)

func near(a, b, eps float64) bool { return math.Abs(a-b) <= eps }

func nearPt(a, b Point, eps float64) bool { return near(a.X, b.X, eps) && near(a.Y, b.Y, eps) }

func TestPointArithmetic(t *testing.T) {
	p, q := Point{X: 3, Y: 4}, Point{X: 1, Y: -2}
	if got := p.Add(q); got != (Point{X: 4, Y: 2}) {
		t.Errorf("Add = %v", got)
	}
	if got := p.Sub(q); got != (Point{X: 2, Y: 6}) {
		t.Errorf("Sub = %v", got)
	}
	if got := p.Mul(-2); got != (Point{X: -6, Y: -8}) {
		t.Errorf("Mul = %v", got)
	}
	if got := p.Len(); got != 5 {
		t.Errorf("Len = %v, want 5", got)
	}
}

func TestRectExtendUnionCenter(t *testing.T) {
	var r Rect
	if !r.Empty() {
		t.Fatal("zero Rect should be empty")
	}
	// A NaN or infinite point would poison every later Min/Max, so it must be
	// ignored rather than absorbed.
	for _, bad := range []Point{{X: math.NaN()}, {Y: math.Inf(1)}, {X: math.Inf(-1)}, {Y: math.NaN()}} {
		if !r.Extend(bad).Empty() {
			t.Errorf("Extend(%v) made the empty rect non-empty", bad)
		}
	}
	r = r.Extend(Point{X: 2, Y: 3})
	if r.Empty() || r.Min != r.Max || r.Min != (Point{X: 2, Y: 3}) {
		t.Fatalf("one-point rect = %+v", r)
	}
	r = r.Extend(Point{X: -1, Y: 7}).Extend(Point{X: 5, Y: 0}).Extend(Point{X: math.NaN(), Y: 100})
	if r.Min != (Point{X: -1, Y: 0}) || r.Max != (Point{X: 5, Y: 7}) {
		t.Errorf("rect = %v..%v, want (-1,0)..(5,7)", r.Min, r.Max)
	}
	if got := r.Size(); got != (Point{X: 6, Y: 7}) {
		t.Errorf("Size = %v", got)
	}
	if got := r.Center(); got != (Point{X: 2, Y: 3.5}) {
		t.Errorf("Center = %v", got)
	}

	if got := r.Union(Rect{}); got != r {
		t.Errorf("Union with empty changed the rect: %+v", got)
	}
	if got := (Rect{}).Union(r); got != r {
		t.Errorf("empty Union r = %+v, want %+v", got, r)
	}
	s := Rect{}.Extend(Point{X: 10, Y: -10})
	u := r.Union(s)
	if u.Min != (Point{X: -1, Y: -10}) || u.Max != (Point{X: 10, Y: 7}) {
		t.Errorf("Union = %v..%v", u.Min, u.Max)
	}
}

func TestAffineThenOrder(t *testing.T) {
	p := Point{X: 1, Y: 0}
	// Then must apply the receiver first: scale-then-move and move-then-scale
	// differ, and getting this backwards misplaces every block insert.
	if got := Scale(2, 1).Then(Translate(1, 0)).Apply(p); got != (Point{X: 3, Y: 0}) {
		t.Errorf("scale then translate = %v, want (3,0)", got)
	}
	if got := Translate(1, 0).Then(Scale(2, 1)).Apply(p); got != (Point{X: 4, Y: 0}) {
		t.Errorf("translate then scale = %v, want (4,0)", got)
	}

	m := Affine{A: 1.5, B: -0.3, C: 0.7, D: 2, E: 4, F: -6}
	n := Affine{A: -0.2, B: 1.1, C: 0.9, D: 0.4, E: -3, F: 8}
	for _, q := range []Point{{}, {X: 1}, {Y: 1}, {X: -2.5, Y: 3.25}} {
		want := n.Apply(m.Apply(q))
		if got := m.Then(n).Apply(q); !nearPt(got, want, 1e-12) {
			t.Errorf("m.Then(n).Apply(%v) = %v, want %v", q, got, want)
		}
	}
	if got := m.Then(Identity); got != m {
		t.Errorf("m.Then(Identity) = %+v", got)
	}
	if got := Identity.Then(m); got != m {
		t.Errorf("Identity.Then(m) = %+v", got)
	}
}

func TestAffineRotateFactorAngle(t *testing.T) {
	// Counter-clockwise: the x axis turns onto the y axis.
	if got := Rotate(math.Pi / 2).Apply(Point{X: 1}); !nearPt(got, Point{Y: 1}, 1e-12) {
		t.Errorf("Rotate(π/2)(1,0) = %v, want (0,1)", got)
	}
	if got := Translate(2, -3).Apply(Point{X: 1, Y: 1}); got != (Point{X: 3, Y: -2}) {
		t.Errorf("Translate = %v", got)
	}
	if got := Scale(2, 3).Apply(Point{X: 1, Y: 1}); got != (Point{X: 2, Y: 3}) {
		t.Errorf("Scale = %v", got)
	}
	for _, a := range []float64{0, 0.3, -1.2, 3} {
		if got := Rotate(a).Angle(); !near(got, a, 1e-12) {
			t.Errorf("Rotate(%v).Angle() = %v", a, got)
		}
		if got := Rotate(a).Factor(); !near(got, 1, 1e-12) {
			t.Errorf("Rotate(%v).Factor() = %v, want 1", a, got)
		}
	}
	// Factor is the geometric mean of the axis scales, sign ignored, so a
	// mirror keeps line widths.
	if got := Scale(2, 8).Factor(); got != 4 {
		t.Errorf("Scale(2,8).Factor = %v, want 4", got)
	}
	if got := Scale(-1, 1).Factor(); got != 1 {
		t.Errorf("mirror Factor = %v, want 1", got)
	}
	if got := Scale(-1, 1).Angle(); !near(got, math.Pi, 1e-12) {
		t.Errorf("x-mirror Angle = %v, want π", got)
	}
}

func TestColorLuma(t *testing.T) {
	if got := Black.Luma(); got != 0 {
		t.Errorf("black luma = %v", got)
	}
	if got := (Color{R: 255, G: 255, B: 255}).Luma(); !near(got, 1, 1e-12) {
		t.Errorf("white luma = %v", got)
	}
	r, g, b := Color{R: 255}.Luma(), Color{G: 255}.Luma(), Color{B: 255}.Luma()
	if g <= r || r <= b {
		t.Errorf("want green > red > blue, got %v %v %v", g, r, b)
	}
}

func ops(p *Path) []Op {
	out := make([]Op, len(p.Segs))
	for i, s := range p.Segs {
		out[i] = s.Op
	}
	return out
}

func equalOps(a, b []Op) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestPolyline(t *testing.T) {
	pts := []Point{{}, {X: 1}, {X: 1, Y: 1}}
	var open, closed, empty Path
	open.Polyline(pts, false)
	closed.Polyline(pts, true)
	empty.Polyline(nil, true)
	if got := ops(&open); !equalOps(got, []Op{MoveTo, LineTo, LineTo}) {
		t.Errorf("open ops = %v", got)
	}
	if got := ops(&closed); !equalOps(got, []Op{MoveTo, LineTo, LineTo, Close}) {
		t.Errorf("closed ops = %v", got)
	}
	if len(empty.Segs) != 0 {
		t.Errorf("closing no points added %v", ops(&empty))
	}
	for i, s := range open.Segs {
		if s.P[0] != pts[i] {
			t.Errorf("seg %d at %v, want %v", i, s.P[0], pts[i])
		}
	}
}

// onEllipse is how far p is from the unit level of the ellipse with centre c,
// semi-major major and the given ratio, in the ellipse's own frame.
func onEllipse(p, c, major Point, ratio float64) float64 {
	d := p.Sub(c)
	a := major.Len()
	ux, uy := major.X/a, major.Y/a
	u := d.X*ux + d.Y*uy
	v := -d.X*uy + d.Y*ux
	return math.Hypot(u/a, v/(a*ratio)) - 1
}

func cubicAt(p0, p1, p2, p3 Point, t float64) Point {
	u := 1 - t
	return p0.Mul(u * u * u).
		Add(p1.Mul(3 * u * u * t)).
		Add(p2.Mul(3 * u * t * t)).
		Add(p3.Mul(t * t * t))
}

func TestEllipseArcPieces(t *testing.T) {
	c := Point{X: 1, Y: 1}
	major := Point{X: 2, Y: 2} // tilted 45°
	const ratio = 0.5
	cases := []struct {
		name   string
		t0, t1 float64
		pieces int
	}{
		{"full turn", 0, 2 * math.Pi, 4},
		{"quarter", 0, math.Pi / 2, 1},
		{"just over a quarter", 0.1, 0.1 + math.Pi/2 + 0.01, 2},
		{"clockwise", math.Pi, -math.Pi / 3, 3},
		{"zero length", 1, 1, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var p Path
			p.EllipseArc(c, major, ratio, tc.t0, tc.t1, false)
			if p.Segs[0].Op != MoveTo {
				t.Fatalf("first op %v, want MoveTo", p.Segs[0].Op)
			}
			if got := len(p.Segs) - 1; got != tc.pieces {
				t.Fatalf("%d cubic pieces, want %d", got, tc.pieces)
			}
			pen := p.Segs[0].P[0]
			if e := onEllipse(pen, c, major, ratio); math.Abs(e) > 1e-12 {
				t.Errorf("start off the ellipse by %v", e)
			}
			for i, s := range p.Segs[1:] {
				if s.Op != CubicTo {
					t.Fatalf("piece %d op %v", i, s.Op)
				}
				if e := onEllipse(s.P[2], c, major, ratio); math.Abs(e) > 1e-12 {
					t.Errorf("piece %d end off the ellipse by %v", i, e)
				}
				// Pieces of at most 90° keep the cubic within a fraction of a
				// percent of the true curve; longer ones bulge visibly.
				for _, u := range []float64{0.25, 0.5, 0.75} {
					if e := onEllipse(
						cubicAt(pen, s.P[0], s.P[1], s.P[2], u),
						c,
						major,
						ratio,
					); math.Abs(
						e,
					) > 2e-3 {
						t.Errorf("piece %d at %v strays %v from the ellipse", i, u, e)
					}
				}
				pen = s.P[2]
			}
			s1, c1 := math.Sincos(tc.t1)
			minor := Point{X: -major.Y, Y: major.X}.Mul(ratio)
			want := c.Add(major.Mul(c1)).Add(minor.Mul(s1))
			if !nearPt(pen, want, 1e-9) {
				t.Errorf("ends at %v, want %v", pen, want)
			}
		})
	}
}

func TestEllipseArcJoin(t *testing.T) {
	var p Path
	// join on an empty path has nothing to continue, so it must still start a
	// subpath rather than emit a dangling LineTo.
	p.EllipseArc(Point{}, Point{X: 1}, 1, 0, math.Pi/2, true)
	if p.Segs[0].Op != MoveTo {
		t.Errorf("join on empty path starts with %v", p.Segs[0].Op)
	}
	n := len(p.Segs)
	p.EllipseArc(Point{X: 5}, Point{X: 1}, 1, 0, math.Pi/2, true)
	if p.Segs[n].Op != LineTo || p.Segs[n].P[0] != (Point{X: 6}) {
		t.Errorf("joined arc starts with %v at %v, want LineTo (6,0)", p.Segs[n].Op, p.Segs[n].P[0])
	}
	n = len(p.Segs)
	p.EllipseArc(Point{X: 5}, Point{X: 1}, 1, 0, math.Pi/2, false)
	if p.Segs[n].Op != MoveTo {
		t.Errorf("unjoined arc starts with %v", p.Segs[n].Op)
	}
}

func TestCircle(t *testing.T) {
	var p Path
	p.Circle(Point{X: 10, Y: -5}, 3)
	if last := p.Segs[len(p.Segs)-1].Op; last != Close {
		t.Errorf("circle ends with %v, want Close", last)
	}
	b := p.Bounds()
	if !nearPt(b.Min, Point{X: 7, Y: -8}, 1e-9) || !nearPt(b.Max, Point{X: 13, Y: -2}, 1e-9) {
		t.Errorf("circle bounds %v..%v", b.Min, b.Max)
	}
}

func TestPathBoundsAndTransformed(t *testing.T) {
	p := &Path{Stroke: true, Width: 2, Color: Color{R: 9}}
	p.MoveTo(Point{X: 0, Y: 0})
	p.CubicTo(Point{X: 1, Y: 5}, Point{X: 2, Y: -1}, Point{X: 3, Y: 0})
	p.LineTo(Point{X: 3, Y: 2})
	p.Close()
	b := p.Bounds()
	// Control points count, so the box reaches y=5 though the curve doesn't.
	if b.Min != (Point{X: 0, Y: -1}) || b.Max != (Point{X: 3, Y: 5}) {
		t.Errorf("bounds %v..%v", b.Min, b.Max)
	}
	if !(&Path{}).Bounds().Empty() {
		t.Error("empty path has non-empty bounds")
	}

	m := Scale(2, 2).Then(Translate(10, 0))
	q, ok := p.Transformed(m).(*Path)
	if !ok {
		t.Fatal("Transformed did not return a *Path")
	}
	qb := q.Bounds()
	if qb.Min != m.Apply(b.Min) || qb.Max != m.Apply(b.Max) {
		t.Errorf("transformed bounds %v..%v", qb.Min, qb.Max)
	}
	if q.Width != 4 || !q.Stroke || q.Color != p.Color {
		t.Errorf("transformed style %+v", q)
	}
	// The copy must not share segments: transforming twice must not move the
	// original.
	if p.Segs[1].P[2] != (Point{X: 3, Y: 0}) || p.Width != 2 {
		t.Error("Transformed changed the original path")
	}
}

func TestTextBoundsAlign(t *testing.T) {
	base := Text{At: Point{X: 100, Y: 50}, Height: 10, Content: "abcd"}
	width := 4 * avgAdvance * 10.0
	for _, tc := range []struct {
		align      Align
		minX, maxX float64
	}{
		{Left, 100, 100 + width},
		{Center, 100 - width/2, 100 + width/2},
		{Right, 100 - width, 100},
	} {
		tx := base
		tx.Align = tc.align
		b := tx.Bounds()
		if !near(b.Min.X, tc.minX, 1e-9) || !near(b.Max.X, tc.maxX, 1e-9) {
			t.Errorf(
				"align %d: x %v..%v, want %v..%v",
				tc.align,
				b.Min.X,
				b.Max.X,
				tc.minX,
				tc.maxX,
			)
		}
	}
	for _, tc := range []struct {
		v          VAlign
		minY, maxY float64
	}{
		{Baseline, 50 - 2.5, 50 + 10},
		{Bottom, 50, 50 + 12.5},
		{Middle, 50 - 5, 50 + 7.5},
		{Top, 50 - 10, 50 + 2.5},
	} {
		tx := base
		tx.VAlign = tc.v
		b := tx.Bounds()
		if !near(b.Min.Y, tc.minY, 1e-9) || !near(b.Max.Y, tc.maxY, 1e-9) {
			t.Errorf("valign %d: y %v..%v, want %v..%v", tc.v, b.Min.Y, b.Max.Y, tc.minY, tc.maxY)
		}
	}

	wide := base
	wide.WidthFactor = 2
	if got, want := wide.Bounds().Size().X, 2*base.Bounds().Size().X; !near(got, want, 1e-9) {
		t.Errorf("WidthFactor 2 width %v, want %v", got, want)
	}

	up := base
	up.Angle = math.Pi / 2
	ub := up.Bounds()
	// Turned a quarter, the text runs up the page from its anchor.
	if !near(ub.Min.Y, 50, 1e-9) || !near(ub.Max.Y, 50+width, 1e-9) ||
		!near(ub.Size().X, 12.5, 1e-9) {
		t.Errorf("rotated bounds %v..%v", ub.Min, ub.Max)
	}
}

func TestTextTransformed(t *testing.T) {
	tx := &Text{At: Point{X: 3, Y: 4}, Height: 2, Content: "A", Angle: 0.2}

	scaled, ok := tx.Transformed(Scale(3, 3).Then(Rotate(0.5))).(*Text)
	if !ok {
		t.Fatal("Transformed did not return a *Text")
	}
	if !near(scaled.Height, 6, 1e-12) || !near(scaled.Angle, 0.7, 1e-12) {
		t.Errorf("scaled+rotated height %v angle %v", scaled.Height, scaled.Angle)
	}
	if want := Scale(3, 3).Then(Rotate(0.5)).Apply(tx.At); !nearPt(scaled.At, want, 1e-12) {
		t.Errorf("At = %v, want %v", scaled.At, want)
	}

	// A plain rotation may leave text upside down; only mirrors are righted.
	flipped, _ := tx.Transformed(Rotate(math.Pi)).(*Text)
	if math.Cos(flipped.Angle) > -0.9 {
		t.Errorf("rotating by π should leave text upside down, angle %v", flipped.Angle)
	}

	cases := []struct {
		name string
		m    Affine
	}{
		{"mirror x", Scale(-1, 1)},
		{"mirror y", Scale(1, -1)},
		{"mirror and turn", Scale(-1, 1).Then(Rotate(0.4))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q, _ := tx.Transformed(tc.m).(*Text)
			if math.Cos(q.Angle) < 0 {
				t.Errorf("mirrored text reads upside down: angle %v", q.Angle)
			}
			if !nearPt(q.At, tc.m.Apply(tx.At), 1e-12) {
				t.Errorf("At = %v", q.At)
			}
			if !near(q.Height, 2, 1e-12) {
				t.Errorf("Height = %v", q.Height)
			}
			if q.Content != "A" {
				t.Errorf("Content = %q", q.Content)
			}
		})
	}
	if tx.At != (Point{X: 3, Y: 4}) || tx.Angle != 0.2 {
		t.Error("Transformed changed the original text")
	}
}

func TestImageBoundsTransformed(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 10, 20))
	// Pixel y runs down, drawing y up: flip and lift by the image height.
	im := &Image{Img: img, M: Scale(0.5, -0.5).Then(Translate(0, 10))}
	b := im.Bounds()
	if b.Min != (Point{X: 0, Y: 0}) || b.Max != (Point{X: 5, Y: 10}) {
		t.Errorf("bounds %v..%v", b.Min, b.Max)
	}
	moved, ok := im.Transformed(Translate(5, 1)).(*Image)
	if !ok {
		t.Fatal("Transformed did not return an *Image")
	}
	mb := moved.Bounds()
	if mb.Min != (Point{X: 5, Y: 1}) || mb.Max != (Point{X: 10, Y: 11}) {
		t.Errorf("moved bounds %v..%v", mb.Min, mb.Max)
	}
	if moved.Img != im.Img {
		t.Error("Transformed should share the pixels")
	}
}

func TestDrawingAddBounds(t *testing.T) {
	var d Drawing
	if !d.Bounds().Empty() {
		t.Error("empty drawing has bounds")
	}
	a := &Path{}
	a.Polyline([]Point{{X: 0, Y: 0}, {X: 1, Y: 1}}, false)
	b := &Path{}
	b.Polyline([]Point{{X: -3, Y: 2}, {X: -2, Y: 5}}, false)
	d.Add(a, &Path{}, b)
	if len(d.Items) != 3 {
		t.Fatalf("%d items", len(d.Items))
	}
	r := d.Bounds()
	if r.Min != (Point{X: -3, Y: 0}) || r.Max != (Point{X: 1, Y: 5}) {
		t.Errorf("bounds %v..%v", r.Min, r.Max)
	}
}
