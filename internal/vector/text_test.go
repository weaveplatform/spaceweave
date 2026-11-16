package vector

import (
	"math"
	"testing"
)

func outlineBounds(t *testing.T, tx *Text) Rect {
	t.Helper()
	p := tx.Outline()
	if p == nil {
		t.Fatalf("Outline(%q) is nil", tx.Content)
	}
	return p.Bounds()
}

func TestOutlineCapHeight(t *testing.T) {
	// "H" has flat tops and sits on the baseline, so its outline spans exactly
	// the cap height: Height is the cap height, not the em.
	tx := &Text{
		At:      Point{X: 10, Y: 20},
		Height:  10,
		Content: "H",
		Color:   Color{R: 200, G: 10, B: 10},
	}
	p := tx.Outline()
	if p == nil {
		t.Fatal("nil outline")
	}
	if !p.Fill || p.Stroke || p.Color != tx.Color {
		t.Errorf("outline style fill=%v stroke=%v color=%v", p.Fill, p.Stroke, p.Color)
	}
	b := p.Bounds()
	if !near(b.Min.Y, 20, 0.05) || !near(b.Max.Y, 30, 0.05) {
		t.Errorf("H spans y %v..%v, want 20..30", b.Min.Y, b.Max.Y)
	}
	if b.Min.X < 10 || b.Min.X > 12 {
		t.Errorf("left-aligned H starts at x=%v, want just right of 10", b.Min.X)
	}
}

func TestOutlineAlign(t *testing.T) {
	at := Point{X: 100, Y: 0}
	mk := func(a Align) *Text { return &Text{At: at, Height: 10, Content: "Hello", Align: a} }
	left := outlineBounds(t, mk(Left))
	center := outlineBounds(t, mk(Center))
	right := outlineBounds(t, mk(Right))
	w := mk(Left).TextWidth()

	// Each alignment moves the same glyphs by a fraction of the advance width.
	if d := left.Min.X - center.Min.X; !near(d, w/2, 1e-9) {
		t.Errorf("left-center shift %v, want %v", d, w/2)
	}
	if d := left.Min.X - right.Min.X; !near(d, w, 1e-9) {
		t.Errorf("left-right shift %v, want %v", d, w)
	}
	if left.Min.X < at.X || right.Max.X > at.X {
		t.Errorf("left starts at %v, right ends at %v; anchor %v", left.Min.X, right.Max.X, at.X)
	}
	if c := center.Center().X; !near(c, at.X, 0.5) {
		t.Errorf("centred text centre %v, want about %v", c, at.X)
	}
	if !near(left.Size().X, right.Size().X, 1e-9) || !near(left.Size().Y, center.Size().Y, 1e-9) {
		t.Error("alignment changed the glyph sizes")
	}
}

func TestOutlineVAlign(t *testing.T) {
	mk := func(v VAlign) Rect {
		return outlineBounds(t, &Text{At: Point{X: 0, Y: 50}, Height: 10, Content: "H", VAlign: v})
	}
	base, bottom, middle, top := mk(Baseline), mk(Bottom), mk(Middle), mk(Top)
	if !near(top.Max.Y, 50, 0.05) {
		t.Errorf("top-aligned H tops out at %v, want 50", top.Max.Y)
	}
	if !near(middle.Center().Y, 50, 0.05) {
		t.Errorf("middle-aligned H centred at %v, want 50", middle.Center().Y)
	}
	// Bottom puts the descender line on the anchor, so the baseline sits above
	// it by the font's descent: some, but well under half the cap height.
	lift := bottom.Min.Y - base.Min.Y
	if lift <= 0.5 || lift >= 5 {
		t.Errorf("bottom lifts the text by %v", lift)
	}
}

func TestOutlineWidthFactorAndAngle(t *testing.T) {
	plain := outlineBounds(t, &Text{Height: 10, Content: "HHH"})
	zero := outlineBounds(t, &Text{Height: 10, Content: "HHH", WidthFactor: 0})
	wide := outlineBounds(t, &Text{Height: 10, Content: "HHH", WidthFactor: 2})
	if plain != zero {
		t.Error("WidthFactor 0 should mean 1")
	}
	if r := wide.Size().X / plain.Size().X; !near(r, 2, 1e-9) {
		t.Errorf("WidthFactor 2 stretches by %v", r)
	}
	if !near(wide.Size().Y, plain.Size().Y, 1e-9) {
		t.Error("WidthFactor changed the height")
	}

	up := outlineBounds(t, &Text{Height: 10, Content: "HHH", Angle: math.Pi / 2})
	if !near(up.Size().X, plain.Size().Y, 1e-6) || !near(up.Size().Y, plain.Size().X, 1e-6) {
		t.Errorf("quarter-turned size %v, plain %v", up.Size(), plain.Size())
	}
	// Turned counter-clockwise, the cap tops point to -x.
	if up.Max.X > 0.05 {
		t.Errorf("quarter-turned text reaches x=%v, want ≤ 0", up.Max.X)
	}
}

func TestOutlineNothingToDraw(t *testing.T) {
	for _, tx := range []*Text{
		{Height: 10, Content: ""},
		{Height: 0, Content: "A"},
		{Height: -1, Content: "A"},
		{Height: 10, Content: "   "}, // spaces have advance but no contours
	} {
		if p := tx.Outline(); p != nil {
			t.Errorf("Outline(%q, h=%v) = %d segs, want nil", tx.Content, tx.Height, len(p.Segs))
		}
	}
}

func TestOutlineMissingGlyphFallsBack(t *testing.T) {
	// Go Regular has no CJK; a missing glyph is drawn as "?" so the tracer
	// still sees that text was there.
	missing := &Text{Height: 10, Content: "漢"}
	q := &Text{Height: 10, Content: "?"}
	if a, b := outlineBounds(t, missing), outlineBounds(t, q); a != b {
		t.Errorf("missing glyph bounds %v..%v, ? %v..%v", a.Min, a.Max, b.Min, b.Max)
	}
	if missing.TextWidth() != q.TextWidth() {
		t.Error("missing glyph advance differs from ?")
	}
}

func TestOutlineSubpathsClosed(t *testing.T) {
	// Every contour must be closed or the rasterizer joins the last point of
	// one glyph to the first of the next.
	p := (&Text{Height: 10, Content: "ab"}).Outline()
	moves, open := 0, false
	for i, s := range p.Segs {
		switch s.Op {
		case MoveTo:
			if open {
				t.Fatalf("segment %d starts a contour while the last is open", i)
			}
			moves++
			open = true
		case Close:
			open = false
		case LineTo, CubicTo:
			if !open {
				t.Fatalf("segment %d draws outside a contour", i)
			}
		}
	}
	// a and b have two contours each.
	if moves != 4 || open {
		t.Errorf("%d contours, last open=%v", moves, open)
	}
}

func TestTextWidth(t *testing.T) {
	w := func(s string, h, f float64) float64 {
		return (&Text{Content: s, Height: h, WidthFactor: f}).TextWidth()
	}
	if w("", 10, 0) != 0 {
		t.Error("empty text has width")
	}
	a, ab, abc := w("a", 10, 0), w("ab", 10, 0), w("abc", 10, 0)
	if a <= 0 || ab <= a || abc <= ab {
		t.Errorf("widths do not grow: %v %v %v", a, ab, abc)
	}
	if !near(w("abc", 20, 0), 2*abc, 1e-9) {
		t.Error("width not proportional to height")
	}
	if !near(w("abc", 10, 1.5), 1.5*abc, 1e-9) {
		t.Error("width not proportional to WidthFactor")
	}
	// Proportional font: an m is wider than an i.
	if w("mmmm", 10, 0) <= w("iiii", 10, 0) {
		t.Error("TextWidth ignores glyph advances")
	}
	// A negative height (mirrored by an import) still has a positive length.
	if got := w("abc", -10, 0); !near(got, abc, 1e-9) {
		t.Errorf("negative height width %v, want %v", got, abc)
	}
}
