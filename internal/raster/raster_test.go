package raster

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// plan draws a 400×300 test plan: a horizontal wall 8 px thick (rows 50–57,
// x 20–379), a vertical wall 6 px thick (cols 100–105, y 58–249), a thin 1 px
// line (row 200) and a solid 60 px block that is too thick to be a wall.
func plan(t *testing.T) *Source {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 400, 300))
	fill(img, image.Rect(0, 0, 400, 300), color.RGBA{250, 250, 250, 255})
	fill(img, image.Rect(20, 50, 380, 58), color.RGBA{10, 10, 10, 255})
	fill(img, image.Rect(100, 58, 106, 250), color.RGBA{20, 20, 20, 255})
	fill(img, image.Rect(150, 200, 350, 201), color.RGBA{0, 0, 0, 255})
	fill(img, image.Rect(300, 100, 360, 160), color.RGBA{0, 0, 0, 255})
	return decode(t, img)
}

func fill(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func decode(t *testing.T, img image.Image) *Source {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	s, err := Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDecodeFormats(t *testing.T) {
	s := plan(t)
	if w, h := s.Size(); w != 400 || h != 300 {
		t.Fatalf("size %d×%d", w, h)
	}
	if !s.Ink(25, 52) || s.Ink(5, 5) {
		t.Fatalf("ink detection wrong, threshold %d", s.Threshold)
	}

	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, s.Image, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	j, err := Decode(&jpg)
	if err != nil || !j.Ink(25, 53) || j.Ink(5, 5) {
		t.Fatalf("jpeg: %v", err)
	}

	w, err := Load(filepath.Join("testdata", "wall.webp"))
	if err != nil {
		t.Fatal(err)
	}
	if !w.Ink(30, 11) || w.Ink(30, 20) {
		t.Fatal("webp ink detection wrong")
	}
}

func TestTransparentBackgroundIsPaper(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 40, 40)) // fully transparent
	for x := 5; x < 35; x++ {
		img.Set(x, 20, color.Black)
	}
	s := decode(t, img)
	if s.Ink(10, 10) || !s.Ink(10, 20) {
		t.Fatal("transparent pixels must read as paper, not ink")
	}
}

func TestLoadErrors(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "missing.png")); err == nil {
		t.Fatal("want error for a missing file")
	}
	bad := filepath.Join(t.TempDir(), "bad.png")
	if err := os.WriteFile(bad, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(bad); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("want decode error, got %v", err)
	}
}

func TestOtsuOnUniformImage(t *testing.T) {
	g := image.NewGray(image.Rect(0, 0, 4, 4))
	for i := range g.Pix {
		g.Pix[i] = 200
	}
	if th := otsu(g); th != 127 {
		t.Fatalf("a single-tone image keeps the default threshold, got %d", th)
	}
}

func TestScan(t *testing.T) {
	s := plan(t)
	if got := s.ScanCol(103, 0, 299); len(got) != 1 || got[0] != (Run{Start: 50, End: 249}) {
		t.Fatalf("col 103: %+v", got)
	}
	if got := s.ScanRow(52, 399, 0); len(got) != 1 || got[0].Len() != 360 {
		t.Fatalf("row 52 reversed range: %+v", got)
	}
	if got := s.ScanRow(120, -50, 1000); len(got) != 2 || got[0] != (Run{Start: 100, End: 105}) || got[1] != (Run{Start: 300, End: 359}) {
		t.Fatalf("row 120 clamped: %+v", got)
	}
	if s.ScanRow(-1, 0, 10) != nil || s.ScanCol(400, 0, 10) != nil {
		t.Fatal("lines off the image have no runs")
	}

	stripes := image.NewRGBA(image.Rect(0, 0, 1000, 1))
	fill(stripes, stripes.Bounds(), color.RGBA{255, 255, 255, 255})
	for x := 0; x < 1000; x += 2 {
		stripes.SetRGBA(x, 0, color.RGBA{0, 0, 0, 255})
	}
	if got := decode(t, stripes).ScanRow(0, 0, 999); len(got) != MaxRuns {
		t.Fatalf("scan not capped: %d runs", len(got))
	}
}

func TestDetectWalls(t *testing.T) {
	s := plan(t)
	walls, err := s.DetectWalls(s.Image.Bounds(), WallLimits{MinThickness: 3, MaxThickness: 20, MinLength: 40, Max: 10})
	if err != nil {
		t.Fatal(err)
	}
	want := []Wall{
		{Axis: "h", From: [2]int{20, 54}, To: [2]int{379, 54}, Thickness: 8},
		{Axis: "v", From: [2]int{103, 50}, To: [2]int{103, 249}, Thickness: 6},
	}
	if len(walls) != len(want) {
		t.Fatalf("got %+v", walls)
	}
	for i := range want {
		if walls[i] != want[i] {
			t.Errorf("wall %d: got %+v want %+v", i, walls[i], want[i])
		}
	}

	if w, _ := s.DetectWalls(s.Image.Bounds(), WallLimits{MinThickness: 3, MaxThickness: 20, MinLength: 40, Max: 1}); len(w) != 1 {
		t.Fatalf("Max not applied: %d", len(w))
	}
	if w, _ := s.DetectWalls(s.Image.Bounds(), WallLimits{MinThickness: 1, MaxThickness: 1, MinLength: 40}); len(w) != 1 || w[0].From[1] != 200 {
		t.Fatalf("thin line only: %+v", w)
	}
	if _, err := s.DetectWalls(image.Rect(500, 500, 600, 600), WallLimits{}); !errors.Is(err, ErrEmptyRegion) {
		t.Fatalf("want ErrEmptyRegion, got %v", err)
	}
}

func TestCrop(t *testing.T) {
	s := plan(t)

	small, err := Crop(s.Image, image.Rect(90, 40, 140, 80), 10)
	if err != nil {
		t.Fatal(err)
	}
	if small.Scale != MaxZoom {
		t.Fatalf("small crops zoom to %v, got %v", MaxZoom, small.Scale)
	}
	if b := small.Image.Bounds(); b.Dx() != margin+200 || b.Dy() != margin+160 {
		t.Fatalf("small crop size %v", b)
	}
	// Source pixel (100, 60) is inside the vertical wall; it must land on ink.
	x, y := small.At(100.5, 60.5)
	if c := small.Image.RGBAAt(int(x), int(y)); c.R > 60 {
		t.Fatalf("wall pixel not dark after crop: %v", c)
	}
	if c := small.Image.RGBAAt(margin-3, margin+16); c == white {
		t.Fatal("expected a ruler tick in the left margin")
	}

	big, err := Crop(s.Image, image.Rect(-100, -100, 5000, 5000), 0)
	if err != nil {
		t.Fatal(err)
	}
	if big.Region != s.Image.Bounds() || big.Scale != float64(MaxEdge-margin)/400 {
		t.Fatalf("region clipped to image and scaled: %v %v", big.Region, big.Scale)
	}

	wide := image.NewRGBA(image.Rect(0, 0, 4000, 100))
	v, err := Crop(wide, wide.Bounds(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if v.Image.Bounds().Dx() != MaxEdge {
		t.Fatalf("large crops fit MaxEdge, got %d", v.Image.Bounds().Dx())
	}

	if _, err := Crop(s.Image, image.Rect(500, 500, 600, 600), 0); !errors.Is(err, ErrEmptyRegion) {
		t.Fatalf("want ErrEmptyRegion, got %v", err)
	}
}

func TestTickHelpers(t *testing.T) {
	if tickStep(100) != 1 || tickStep(1) != 100 || tickStep(0.001) != 10000 {
		t.Fatal("tickStep")
	}
	for _, c := range []struct{ v, step, want int }{{7, 5, 10}, {10, 5, 10}, {-7, 5, -5}, {0, 5, 0}} {
		if got := ceilTo(c.v, c.step); got != c.want {
			t.Errorf("ceilTo(%d, %d) = %d, want %d", c.v, c.step, got, c.want)
		}
	}
}

func TestOverlay(t *testing.T) {
	s := plan(t)
	red := color.RGBA{214, 39, 40, 255}
	out := Overlay(s.Image, []Shape{
		{Points: []Point{{20, 150}, {380, 150}}, Width: 6, Color: red},
		{Points: []Point{{200, 220}, {260, 220}, {260, 280}}, Closed: true, Width: 2, Color: red, Opacity: 1, Text: "Room", At: Point{230, 250}},
		{Points: []Point{{0, 0}}, Width: 2, Color: red}, // a single point draws nothing
	})
	if c := out.RGBAAt(200, 150); c.R < 180 || c.G > 120 {
		t.Fatalf("wall stroke missing: %v", c)
	}
	if c := out.RGBAAt(230, 250); c == out.RGBAAt(230, 230) && c.R > 250 {
		t.Fatal("label not drawn")
	}
	// The closing edge (260,280)→(200,220) passes through (230,250)'s diagonal.
	if c := out.RGBAAt(220, 240); c != red {
		t.Fatalf("closing edge missing: %v", c)
	}
	if c := out.RGBAAt(5, 5); c.R < 240 {
		t.Fatalf("paper should stay light: %v", c)
	}
	if distToSegment(1, 1, Point{0, 0}, Point{0, 0}) != 1.4142135623730951 {
		t.Fatal("zero-length segment distance")
	}
}

func TestFitAndEncode(t *testing.T) {
	s := plan(t)
	same, k := Fit(s.Image, 1000)
	if k != 1 || same != s.Image {
		t.Fatal("small images are not resized")
	}
	small, k := Fit(s.Image, 200)
	if k != 0.5 || small.Bounds().Dx() != 200 || small.Bounds().Dy() != 150 {
		t.Fatalf("fit: %v %v", k, small.Bounds())
	}
	b, err := EncodePNG(small)
	if err != nil || !bytes.HasPrefix(b, []byte("\x89PNG")) {
		t.Fatalf("encode: %v", err)
	}
	if _, err := EncodePNG(image.NewRGBA(image.Rect(0, 0, 0, 0))); err == nil {
		t.Fatal("want error encoding an empty image")
	}
}
