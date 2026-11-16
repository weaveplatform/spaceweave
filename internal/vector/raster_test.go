package vector

import (
	"errors"
	"image"
	"image/color"
	"math"
	"testing"

	"golang.org/x/image/vector"
)

// px is the grey level at a drawing point.
func px(r *Raster, p Point) uint8 {
	q := r.ToPixel(p)
	return r.Image.GrayAt(int(math.Floor(q.X)), int(math.Floor(q.Y))).Y
}

func line(a, b Point) *Path {
	p := &Path{Stroke: true}
	p.Polyline([]Point{a, b}, false)
	return p
}

func TestRasterizeHorizontalLine(t *testing.T) {
	d := &Drawing{Items: []Item{line(Point{X: 0, Y: 0}, Point{X: 100, Y: 0})}}
	r, err := Rasterize(d, Options{MaxSide: 200})
	if err != nil {
		t.Fatal(err)
	}
	// Margin 24 each side leaves 152 px for 100 units; a line has no height,
	// so the image is just the two margins tall.
	if got := r.Image.Bounds(); got.Dx() != 200 || got.Dy() != 48 {
		t.Fatalf("image %v, want 200x48", got)
	}
	if !near(r.PxPerUnit, 1.52, 1e-12) {
		t.Errorf("PxPerUnit = %v, want 1.52", r.PxPerUnit)
	}
	if got := r.ToPixel(Point{X: 50, Y: 0}); !nearPt(got, Point{X: 24 + 76, Y: 24}, 1e-9) {
		t.Errorf("ToPixel(50,0) = %v, want (100,24)", got)
	}
	for x := 28; x < 172; x++ {
		if v := r.Image.GrayAt(x, 23).Y; v > 100 {
			t.Fatalf("pixel (%d,23) on the line is %d, want ink", x, v)
		}
	}
	for y := range 48 {
		if y >= 22 && y <= 25 {
			continue
		}
		for x := range 200 {
			if v := r.Image.GrayAt(x, y).Y; v != 0xff {
				t.Fatalf("pixel (%d,%d) off the line is %d, want white", x, y, v)
			}
		}
	}
	// The margin either side of the line's ends stays white.
	if v := r.Image.GrayAt(10, 24).Y; v != 0xff {
		t.Errorf("left margin is %d", v)
	}
}

func TestRasterizeYUp(t *testing.T) {
	block := square(0, 90, 10)
	block.Fill, block.Stroke = true, false
	d := &Drawing{Items: []Item{square(0, 0, 100), block}}
	r, err := Rasterize(d, Options{MaxSide: 248})
	if err != nil {
		t.Fatal(err)
	}
	top, bottom := r.ToPixel(Point{X: 5, Y: 95}), r.ToPixel(Point{X: 5, Y: 5})
	if top.Y >= bottom.Y {
		t.Fatalf(
			"y=95 maps to row %v, y=5 to row %v; larger y should be higher up",
			top.Y,
			bottom.Y,
		)
	}
	if v := px(r, Point{X: 5, Y: 95}); v > 10 {
		t.Errorf("filled block in the top-left is %d", v)
	}
	if top.Y > float64(r.Image.Bounds().Dy())/4 {
		t.Errorf("top-left block drawn at row %v of %d", top.Y, r.Image.Bounds().Dy())
	}
	if v := px(r, Point{X: 5, Y: 5}); v != 0xff {
		t.Errorf("bottom-left inside the frame is %d, want white", v)
	}
	if v := px(r, Point{X: 95, Y: 95}); v != 0xff {
		t.Errorf("top-right inside the frame is %d, want white", v)
	}
}

func TestRasterizeClosedVersusOpen(t *testing.T) {
	pts := []Point{{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 100}, {X: 0, Y: 100}}
	for _, closed := range []bool{false, true} {
		p := &Path{Stroke: true}
		p.Polyline(pts, closed)
		r, err := Rasterize(&Drawing{Items: []Item{p}}, Options{MaxSide: 248})
		if err != nil {
			t.Fatal(err)
		}
		// The left side is only drawn by Close.
		left := px(r, Point{X: 0, Y: 50})
		if closed && left > 100 {
			t.Errorf("closed square's left side is %d, want ink", left)
		}
		if !closed && left != 0xff {
			t.Errorf("open path's missing side is %d, want white", left)
		}
		if v := px(r, Point{X: 100, Y: 50}); v > 100 {
			t.Errorf("closed=%v: right side is %d", closed, v)
		}
	}
}

func TestRasterizeInkIsBlack(t *testing.T) {
	// A yellow layer has a luma near 0.93; drawn as itself it would vanish
	// under the tracer's ink threshold.
	p := line(Point{X: 0, Y: 0}, Point{X: 100, Y: 0})
	p.Color = Color{R: 255, G: 255}
	p.Width = 3
	r, err := Rasterize(&Drawing{Items: []Item{p}}, Options{MaxSide: 148})
	if err != nil {
		t.Fatal(err)
	}
	if v := px(r, Point{X: 50, Y: 0}); v != 0 {
		t.Errorf("yellow stroke is %d, want black", v)
	}
}

func TestRasterizeFillShade(t *testing.T) {
	for _, tc := range []struct {
		c    Color
		want uint8
	}{
		{Color{R: 230, G: 230, B: 230}, 230}, // pale room fill stays pale
		{Color{R: 100, G: 100, B: 100}, 40},  // dark fill becomes ink
		{Black, 0},
	} {
		sq := square(0, 0, 100)
		sq.Stroke, sq.Fill, sq.Color = false, true, tc.c
		r, err := Rasterize(&Drawing{Items: []Item{sq}}, Options{MaxSide: 248})
		if err != nil {
			t.Fatal(err)
		}
		if v := px(r, Point{X: 50, Y: 50}); int(v) < int(tc.want)-1 || int(v) > int(tc.want)+1 {
			t.Errorf("fill %v shaded %d, want %d", tc.c, v, tc.want)
		}
		if v := r.Image.GrayAt(5, 5).Y; v != 0xff {
			t.Errorf("fill %v: margin is %d", tc.c, v)
		}
	}
}

func TestRasterizeFillHole(t *testing.T) {
	// Both rings wound the same way, as CAD hatches often are: the inner ring
	// must still be a hole.
	p := &Path{Fill: true}
	p.Polyline([]Point{{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 100}, {X: 0, Y: 100}}, true)
	p.Polyline([]Point{{X: 30, Y: 30}, {X: 70, Y: 30}, {X: 70, Y: 70}, {X: 30, Y: 70}}, true)
	r, err := Rasterize(&Drawing{Items: []Item{p}}, Options{MaxSide: 248})
	if err != nil {
		t.Fatal(err)
	}
	if v := px(r, Point{X: 50, Y: 50}); v != 0xff {
		t.Errorf("hole is %d, want white", v)
	}
	if v := px(r, Point{X: 15, Y: 50}); v != 0 {
		t.Errorf("fill around the hole is %d, want black", v)
	}
}

func TestRasterizeMaxSide(t *testing.T) {
	d := &Drawing{Items: []Item{line(Point{X: 0, Y: 0}, Point{X: 1000, Y: 500})}}
	r, err := Rasterize(d, Options{MaxSide: 300})
	if err != nil {
		t.Fatal(err)
	}
	b := r.Image.Bounds()
	if b.Dx() != 300 || b.Dy() != 174 {
		t.Errorf("image %dx%d, want 300x174", b.Dx(), b.Dy())
	}
	if !near(r.PxPerUnit, 0.252, 1e-12) {
		t.Errorf("PxPerUnit %v, want 0.252", r.PxPerUnit)
	}
	// The drawing's far corner lands one margin in from the image's corner.
	if got := r.ToPixel(Point{X: 1000, Y: 0}); !nearPt(got, Point{X: 276, Y: 150}, 1e-9) {
		t.Errorf("ToPixel(1000,0) = %v", got)
	}
}

func TestRasterizeDefaults(t *testing.T) {
	r, err := Rasterize(&Drawing{Items: []Item{line(Point{}, Point{X: 1})}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Image.Bounds(); got.Dx() != DefaultMaxSide || got.Dy() != 2*DefaultMargin {
		t.Errorf("default image %v", got)
	}
	if want := float64(DefaultMaxSide - 2*DefaultMargin); r.PxPerUnit != want {
		t.Errorf("PxPerUnit %v, want %v", r.PxPerUnit, want)
	}
}

func TestOptionsWithDefaults(t *testing.T) {
	o := Options{MaxSide: 40, Margin: 30, MinStroke: -1}.withDefaults()
	// A margin bigger than the image would leave no room for the drawing.
	if o.Margin != 10 || o.MaxSide != 40 || o.MinStroke != DefaultMinStroke {
		t.Errorf("withDefaults = %+v", o)
	}
	o = Options{Margin: 5, MinStroke: 3}.withDefaults()
	if o.Margin != 5 || o.MinStroke != 3 || o.MaxSide != DefaultMaxSide {
		t.Errorf("withDefaults kept = %+v", o)
	}
}

func TestRasterizeMaxPixelsShrinks(t *testing.T) {
	d := &Drawing{Items: []Item{square(0, 0, 100)}}
	free, err := Rasterize(d, Options{MaxSide: 1000})
	if err != nil {
		t.Fatal(err)
	}
	capped, err := Rasterize(d, Options{MaxSide: 1000, MaxPixels: 250_000})
	if err != nil {
		t.Fatal(err)
	}
	fb, cb := free.Image.Bounds(), capped.Image.Bounds()
	if fb.Dx()*fb.Dy() != 1_000_000 {
		t.Fatalf("uncapped image %v", fb)
	}
	if cb.Dx()*cb.Dy() >= fb.Dx()*fb.Dy()/3 {
		t.Errorf("capped image %v barely smaller than %v", cb, fb)
	}
	// The cap is met closely, not overshot: the margins don't shrink, so
	// the scale drops by a little more than half.
	if n := cb.Dx() * cb.Dy(); n > 250_000 || n < 245_000 {
		t.Errorf("capped image has %d pixels, want just under 250000", n)
	}
	if capped.PxPerUnit >= free.PxPerUnit/2 {
		t.Errorf("capped scale %v, want under half of %v", capped.PxPerUnit, free.PxPerUnit)
	}
	// The frame still lands on its pixels at the reduced scale.
	if v := px(capped, Point{X: 0, Y: 50}); v > 100 {
		t.Errorf("frame at capped scale is %d", v)
	}
}

func TestRasterizeMaxPixelsIsACap(t *testing.T) {
	// fit scales by sqrt(MaxPixels / (w*h)) with w and h including the fixed
	// margins; the margins don't shrink, so the result overshoots. Here
	// 524x524 = 274,576 px for a cap of 250,000.
	d := &Drawing{Items: []Item{square(0, 0, 100)}}
	r, err := Rasterize(d, Options{MaxSide: 1000, MaxPixels: 250_000})
	if err != nil {
		t.Fatal(err)
	}
	if b := r.Image.Bounds(); b.Dx()*b.Dy() > 250_000 {
		t.Errorf("image %dx%d = %d px, over the 250,000 cap", b.Dx(), b.Dy(), b.Dx()*b.Dy())
	}
}

func TestRasterizeTinyMaxPixels(t *testing.T) {
	// Even an absurd cap yields a usable image rather than a zero-size one.
	r, err := Rasterize(
		&Drawing{Items: []Item{square(0, 0, 100)}},
		Options{MaxSide: 1000, MaxPixels: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	if b := r.Image.Bounds(); b.Dx() < 1 || b.Dy() < 1 {
		t.Errorf("image %v", b)
	}
}

func TestRasterizeEmpty(t *testing.T) {
	for name, d := range map[string]*Drawing{
		"no items":    {},
		"empty paths": {Items: []Item{&Path{}, &Path{Stroke: true}}},
	} {
		if _, err := Rasterize(d, Options{}); !errors.Is(err, ErrEmpty) {
			t.Errorf("%s: err = %v, want ErrEmpty", name, err)
		}
	}
}

func darkRows(r *Raster, x int) int {
	n := 0
	for y := range r.Image.Bounds().Dy() {
		if r.Image.GrayAt(x, y).Y < 128 {
			n++
		}
	}
	return n
}

func TestRasterizeMinStroke(t *testing.T) {
	// A hairline a thousandth of a pixel wide must still come out as ink.
	p := line(Point{X: 0, Y: 0}, Point{X: 100, Y: 0})
	p.Width = 0.001
	d := &Drawing{Items: []Item{p}}
	thin, err := Rasterize(d, Options{MaxSide: 148})
	if err != nil {
		t.Fatal(err)
	}
	thick, err := Rasterize(d, Options{MaxSide: 148, MinStroke: 6})
	if err != nil {
		t.Fatal(err)
	}
	tn, kn := darkRows(thin, 74), darkRows(thick, 74)
	if tn < 1 || tn > 2 {
		t.Errorf("default min stroke gives %d dark rows, want 1–2", tn)
	}
	if kn < 5 || kn > 7 {
		t.Errorf("MinStroke 6 gives %d dark rows, want about 6", kn)
	}
}

func TestRasterizeRoundCaps(t *testing.T) {
	// A thick line has round caps: ink continues past its end by half its
	// width, which a plain quad would not give.
	p := line(Point{X: 0, Y: 0}, Point{X: 100, Y: 0})
	p.Width = 10
	r, err := Rasterize(&Drawing{Items: []Item{p}}, Options{MaxSide: 148})
	if err != nil {
		t.Fatal(err)
	}
	end := r.ToPixel(Point{X: 100, Y: 0})
	if v := r.Image.GrayAt(int(end.X)+3, int(end.Y)).Y; v > 50 {
		t.Errorf("3 px past the end is %d, want the cap's ink", v)
	}
	if v := r.Image.GrayAt(int(end.X)+8, int(end.Y)).Y; v != 0xff {
		t.Errorf("8 px past the end is %d, want white", v)
	}
	// Width is in drawing units, scaled to pixels: 10 units at 1 px/unit.
	if n := darkRows(r, 74); n < 9 || n > 11 {
		t.Errorf("10-unit line is %d rows thick", n)
	}
}

func TestRasterizeText(t *testing.T) {
	d := &Drawing{Items: []Item{
		&Text{At: Point{X: 10, Y: 10}, Height: 60, Content: "H", Color: Color{R: 255, G: 255}},
		&Text{At: Point{X: 50, Y: 50}, Height: 10, Content: ""}, // nothing to outline
		square(0, 0, 100),
	}}
	r, err := Rasterize(d, Options{MaxSide: 148})
	if err != nil {
		t.Fatal(err)
	}
	// The H's left stem runs up from the anchor; its middle between the
	// stems is white above and below the crossbar.
	stem := (&Text{Height: 60, Content: "H"}).Outline().Bounds()
	x := 10 + stem.Min.X + 2
	if v := px(r, Point{X: x, Y: 40}); v != 0 {
		t.Errorf("H stem is %d, want black ink", v)
	}
	if v := px(r, Point{X: 10 + stem.Center().X, Y: 15}); v != 0xff {
		t.Errorf("between the H's stems is %d, want white", v)
	}
	if v := px(r, Point{X: 10 + stem.Center().X, Y: 10 + stem.Center().Y}); v != 0 {
		t.Errorf("H crossbar is %d, want ink", v)
	}
}

func TestRasterizeOnlyText(t *testing.T) {
	r, err := Rasterize(
		&Drawing{Items: []Item{&Text{Height: 10, Content: "I"}}},
		Options{MaxSide: 100},
	)
	if err != nil {
		t.Fatal(err)
	}
	if darkRows(r, r.Image.Bounds().Dx()/2) == 0 {
		t.Error("a drawing of only text has no ink")
	}
}

func TestRasterizeTrimmedStray(t *testing.T) {
	items := append(grid(40), line(Point{X: 1e6, Y: 1e6}, Point{X: 1e6 + 1, Y: 1e6}))
	r, err := Rasterize(&Drawing{Items: items}, Options{MaxSide: 200})
	if err != nil {
		t.Fatal(err)
	}
	if r.Trimmed != 1 {
		t.Errorf("Trimmed = %d, want 1", r.Trimmed)
	}
	if r.Bounds.Max.X != 19 || r.Bounds.Max.Y != 7 {
		t.Errorf("Bounds %v..%v, want the cluster", r.Bounds.Min, r.Bounds.Max)
	}
	// The cluster fills the image rather than shrinking to a dot.
	if !near(r.PxPerUnit, 152.0/19, 1e-9) {
		t.Errorf("PxPerUnit %v", r.PxPerUnit)
	}
}

// halfBlack is a w×h image whose top half (smaller pixel y) is black.
func halfBlack(rect image.Rectangle) *image.Gray {
	img := image.NewGray(rect)
	mid := (rect.Min.Y + rect.Max.Y) / 2
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if y >= mid {
				img.SetGray(x, y, color.Gray{Y: 0xff})
			}
		}
	}
	return img
}

func TestRasterizeImage(t *testing.T) {
	// Pixel rows run down, drawing y up: row 0 lands at y=40.
	im := &Image{Img: halfBlack(image.Rect(0, 0, 4, 4)), M: Scale(10, -10).Then(Translate(0, 40))}
	r, err := Rasterize(&Drawing{Items: []Item{im}}, Options{MaxSide: 140})
	if err != nil {
		t.Fatal(err)
	}
	if v := px(r, Point{X: 20, Y: 35}); v > 10 {
		t.Errorf("top of image is %d, want black", v)
	}
	if v := px(r, Point{X: 20, Y: 5}); v < 245 {
		t.Errorf("bottom of image is %d, want white", v)
	}
	if v := r.Image.GrayAt(2, 2).Y; v != 0xff {
		t.Errorf("margin is %d", v)
	}
}

func TestRasterizeSubImage(t *testing.T) {
	// Image.Bounds and the SVG writer (png.Encode) both treat pixel
	// coordinates as 0..width, but drawImage passes Img.Bounds() to
	// x/image/draw, which reads absolute pixel coordinates, so an image whose
	// bounds don't start at (0,0) lands M(min) away from where Bounds says.
	// No reader produces such an image today (decoded images start at 0).
	im := &Image{
		Img: halfBlack(image.Rect(100, 100, 104, 104)),
		M:   Scale(10, -10).Then(Translate(0, 40)),
	}
	r, err := Rasterize(&Drawing{Items: []Item{im}}, Options{MaxSide: 140})
	if err != nil {
		t.Fatal(err)
	}
	if v := px(r, Point{X: 20, Y: 35}); v > 10 {
		t.Errorf("top of image is %d, want black", v)
	}
}

func TestFlatten(t *testing.T) {
	segs := []Seg{
		// A LineTo with no subpath open starts one at the pen, the origin.
		{Op: LineTo, P: [3]Point{{X: 1, Y: 0}}},
		{Op: Close},
		// Close with nothing open is ignored.
		{Op: Close},
		// After a Close the pen is back at the subpath's start.
		{Op: LineTo, P: [3]Point{{X: 0, Y: 5}}},
		{Op: MoveTo, P: [3]Point{{X: 10, Y: 0}}},
		{Op: CubicTo, P: [3]Point{{X: 10, Y: 10}, {X: 20, Y: 10}, {X: 20, Y: 0}}},
	}
	pls := flatten(segs, 0.25)
	if len(pls) != 3 {
		t.Fatalf("%d polylines, want 3", len(pls))
	}
	if !pls[0].closed || len(pls[0].pts) != 2 || pls[0].pts[0] != (Point{}) {
		t.Errorf("first polyline %+v", pls[0])
	}
	if pls[1].closed || pls[1].pts[0] != (Point{}) || pls[1].pts[1] != (Point{X: 0, Y: 5}) {
		t.Errorf("second polyline %+v", pls[1])
	}
	curve := pls[2].pts
	if curve[0] != (Point{X: 10, Y: 0}) || !nearPt(curve[len(curve)-1], Point{X: 20, Y: 0}, 1e-12) {
		t.Errorf("curve runs %v..%v", curve[0], curve[len(curve)-1])
	}
	if len(curve) < 4 {
		t.Errorf("curve flattened to only %d points", len(curve))
	}
	// Each chord's midpoint stays within the tolerance of the curve's peak
	// height, 7.5 at x=15.
	peak := 0.0
	for _, p := range curve {
		peak = math.Max(peak, p.Y)
	}
	if peak < 7.5-0.25 || peak > 7.5 {
		t.Errorf("flattened peak %v, want within 0.25 of 7.5", peak)
	}
}

func TestAppendCubicCapsPieces(t *testing.T) {
	// A curve kilometres across at sub-pixel tolerance would otherwise ask
	// for millions of points.
	pts := appendCubic(nil, Point{}, Point{Y: 1e9}, Point{X: 1e9, Y: 1e9}, Point{X: 1e9}, 0.25)
	if len(pts) != 256 {
		t.Errorf("%d points, want the cap of 256", len(pts))
	}
	// A straight cubic needs a single piece.
	pts = appendCubic(nil, Point{}, Point{X: 1}, Point{X: 2}, Point{X: 3}, 0.25)
	if len(pts) != 1 || pts[0] != (Point{X: 3}) {
		t.Errorf("straight cubic = %v", pts)
	}
}

func TestRingHelpers(t *testing.T) {
	ccw := []Point{{X: 0, Y: 0}, {X: 2, Y: 0}, {X: 2, Y: 2}, {X: 0, Y: 2}}
	cw := reversed(ccw)
	if area(ccw) != 4 || area(cw) != -4 {
		t.Errorf("areas %v %v", area(ccw), area(cw))
	}
	if cw[0] != ccw[3] || cw[3] != ccw[0] {
		t.Errorf("reversed = %v", cw)
	}
	if area(positive(cw)) != 4 || area(positive(ccw)) != 4 {
		t.Error("positive did not give a positive ring")
	}
	if !inside(Point{X: 1, Y: 1}, ccw) || !inside(Point{X: 1, Y: 1}, cw) {
		t.Error("centre not inside")
	}
	if inside(Point{X: 3, Y: 1}, ccw) || inside(Point{X: 1, Y: -1}, ccw) {
		t.Error("outside point reported inside")
	}
	if q := quad(Point{X: 1, Y: 1}, Point{X: 1, Y: 1}, 2); q != nil {
		t.Errorf("zero-length quad = %v", q)
	}
	if q := quad(Point{}, Point{X: 0, Y: -4}, 1); area(q) != 8 {
		t.Errorf("quad area %v, want 8 and positive", area(q))
	}
	d := disc(Point{X: 5, Y: 5}, 2)
	if len(d) != 12 || area(d) <= 0 {
		t.Errorf("disc has %d points, area %v", len(d), area(d))
	}
	for _, p := range d {
		if !near(p.Sub(Point{X: 5, Y: 5}).Len(), 2, 1e-12) {
			t.Errorf("disc point %v off the circle", p)
		}
	}
}

func TestNormalise(t *testing.T) {
	outer := polyline{pts: []Point{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}, {X: 0, Y: 10}}}
	hole := polyline{
		pts: []Point{{X: 3, Y: 3}, {X: 3, Y: 7}, {X: 7, Y: 7}, {X: 7, Y: 3}},
	} // clockwise
	island := polyline{pts: []Point{{X: 4, Y: 4}, {X: 6, Y: 4}, {X: 6, Y: 6}, {X: 4, Y: 6}}}
	degenerate := polyline{pts: []Point{{X: 1, Y: 1}, {X: 2, Y: 2}}}
	rings := normalise([]polyline{outer, hole, island, degenerate})
	if len(rings) != 3 {
		t.Fatalf("%d rings, want 3 (the two-point one dropped)", len(rings))
	}
	// Even-odd by nesting: outer and island positive, the hole negative,
	// whatever way they were wound.
	if area(rings[0]) <= 0 || area(rings[1]) >= 0 || area(rings[2]) <= 0 {
		t.Errorf("areas %v %v %v, want + - +", area(rings[0]), area(rings[1]), area(rings[2]))
	}
}

func TestNormaliseManyRings(t *testing.T) {
	// Past maxNesting the quadratic nesting test is skipped and every ring is
	// simply made positive.
	pls := make([]polyline, maxNesting+1)
	for i := range pls {
		x := float64(i) * 3
		pls[i] = polyline{
			pts: []Point{{X: x, Y: 0}, {X: x, Y: 1}, {X: x + 1, Y: 1}, {X: x + 1, Y: 0}},
		}
	}
	rings := normalise(pls)
	if len(rings) != len(pls) {
		t.Fatalf("%d rings", len(rings))
	}
	for i, r := range rings {
		if area(r) <= 0 {
			t.Fatalf("ring %d area %v", i, area(r))
		}
	}
}

func TestFillShade(t *testing.T) {
	for _, tc := range []struct {
		c    Color
		want uint8
	}{
		{Color{R: 255, G: 255, B: 255}, 255},
		{Black, 0},
		{Color{R: 200, G: 200, B: 200}, 200},
		{Color{R: 128, G: 128, B: 128}, 51}, // below 0.6, darkened to ink
	} {
		if got := fillShade(tc.c); got != tc.want {
			t.Errorf("fillShade(%v) = %d, want %d", tc.c, got, tc.want)
		}
	}
}

func TestFillOutsideCanvas(t *testing.T) {
	canvas := image.NewGray(image.Rect(0, 0, 10, 10))
	for i := range canvas.Pix {
		canvas.Pix[i] = 0xff
	}
	p := square(50, 50, 5)
	p.Fill = true
	fill(canvas, p)
	for i, v := range canvas.Pix {
		if v != 0xff {
			t.Fatalf("pixel %d painted by an off-canvas fill", i)
		}
	}
}

func TestFillClipped(t *testing.T) {
	// A fill straddling the canvas edge is painted where it overlaps, at the
	// right place, even though its own rasterizer starts at the clip.
	canvas := image.NewGray(image.Rect(0, 0, 10, 10))
	for i := range canvas.Pix {
		canvas.Pix[i] = 0xff
	}
	p := square(-5, 4, 7) // covers x -5..2, y 4..11
	p.Fill = true
	fill(canvas, p)
	if v := canvas.GrayAt(1, 6).Y; v != 0 {
		t.Errorf("inside the clipped fill is %d", v)
	}
	if v := canvas.GrayAt(3, 6).Y; v != 0xff {
		t.Errorf("right of the fill is %d", v)
	}
	if v := canvas.GrayAt(1, 2).Y; v != 0xff {
		t.Errorf("above the fill is %d", v)
	}
}

func TestOverlaps(t *testing.T) {
	if overlaps(Rect{}, 10, 10) {
		t.Error("empty rect overlaps")
	}
	in := Rect{}.Extend(Point{X: -5, Y: -5}).Extend(Point{X: 1, Y: 1})
	out := Rect{}.Extend(Point{X: 11, Y: 0}).Extend(Point{X: 20, Y: 5})
	if !overlaps(in, 10, 10) || overlaps(out, 10, 10) {
		t.Error("overlaps wrong")
	}
}

func TestAddRingIgnoresDegenerate(t *testing.T) {
	z := vector.NewRasterizer(4, 4)
	addRing(z, []Point{{}, {X: 4, Y: 4}})
	mask := image.NewAlpha(image.Rect(0, 0, 4, 4))
	z.Draw(mask, mask.Bounds(), image.Opaque, image.Point{})
	for _, a := range mask.Pix {
		if a != 0 {
			t.Fatal("a two-point ring was drawn")
		}
	}
}

func TestRasterizeSinglePoint(t *testing.T) {
	// A drawing with no extent has no scale to fit; it must still render to
	// a margin-only image with the point in its middle, not divide by zero.
	p := line(Point{X: 7, Y: 7}, Point{X: 7, Y: 7})
	r, err := Rasterize(&Drawing{Items: []Item{p}}, Options{MaxSide: 100})
	if err != nil {
		t.Fatal(err)
	}
	if b := r.Image.Bounds(); b.Dx() != 48 || b.Dy() != 48 {
		t.Errorf("image %v, want 48x48", b)
	}
	if math.IsInf(r.PxPerUnit, 0) || math.IsNaN(r.PxPerUnit) || r.PxPerUnit <= 0 {
		t.Errorf("PxPerUnit %v", r.PxPerUnit)
	}
	if got := r.ToPixel(Point{X: 7, Y: 7}); got != (Point{X: 24, Y: 24}) {
		t.Errorf("ToPixel = %v, want (24,24)", got)
	}
}
