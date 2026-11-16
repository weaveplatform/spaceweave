package synth

import (
	"bytes"
	"encoding/json"
	"image"
	"os"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/plan"
)

func fixture(t *testing.T, name string) *plan.Trace {
	t.Helper()
	b, err := os.ReadFile("../../plan/testdata/" + name + ".trace.json")
	if err != nil {
		t.Fatal(err)
	}
	var tr plan.Trace
	if err := json.Unmarshal(b, &tr); err != nil {
		t.Fatal(err)
	}
	return &tr
}

func ink(g *image.Gray, x, y int) bool { return g.GrayAt(x, y).Y < 128 }

// inkNear reports ink within d px of (x, y).
func inkNear(g *image.Gray, x, y, d int) bool {
	for yy := y - d; yy <= y+d; yy++ {
		for xx := x - d; xx <= x+d; xx++ {
			if (image.Point{xx, yy}).In(g.Rect) && ink(g, xx, yy) {
				return true
			}
		}
	}
	return false
}

func count(g *image.Gray, r image.Rectangle) int {
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if ink(g, x, y) {
				n++
			}
		}
	}
	return n
}

func TestRenderFixtures(t *testing.T) {
	for _, c := range []struct {
		name string
		size image.Point
	}{{"office", image.Point{791, 446}}, {"corporate", image.Point{2200, 1393}}} {
		tr := fixture(t, c.name)
		g := Render(tr, Options{Text: true, Furniture: true, AntiAlias: true})
		if g.Rect.Size() != c.size {
			t.Errorf("%s: size %v, want %v", c.name, g.Rect.Size(), c.size)
		}
		bare := Render(tr, Options{})
		full := count(g, g.Rect)
		if walls := count(bare, bare.Rect); walls == 0 || full <= walls {
			t.Errorf("%s: %d inked bare, %d with text and furniture", c.name, walls, full)
		}
	}
}

func TestWallBandsAndCorners(t *testing.T) {
	// 0.2 m at 40 px/m is 8 px; the default extension closes the corner.
	g := Render(New(40).Box(40, 40, 200, 120).Trace(), Options{})
	if !ink(g, 120, 36) || !ink(g, 120, 43) || ink(g, 120, 34) || ink(g, 120, 46) {
		t.Error("north wall is not 8 px about y = 40")
	}
	if !ink(g, 37, 37) {
		t.Error("corner not closed")
	}
	if g.Rect.Size() != (image.Point{240, 160}) {
		t.Errorf("size %v", g.Rect.Size())
	}
	sized := Render(
		New(40).Box(40, 40, 200, 120).Trace(),
		Options{Size: image.Point{100, 80}, Margin: 5},
	)
	if sized.Rect.Size() != (image.Point{100, 80}) {
		t.Errorf("size option ignored: %v", sized.Rect.Size())
	}
}

func TestWallOptions(t *testing.T) {
	zero := 0.0
	tr := New(40).
		Wall("glass", [2]float64{20, 20}, [2]float64{120, 20}).
		Wall("interior", [2]float64{20, 60}, [2]float64{120, 60}).
		Wall("interior", [2]float64{20, 100}, [2]float64{20, 100}). // no length: skipped
		Trace()
	tr.Walls[1].Thickness = 0.3
	tr.Walls[1].Ext = &zero
	tr.Calibration.PxPerMetre = [2]float64{} // uncalibrated: 40 px/m assumed
	g := Render(tr, Options{})
	if !ink(g, 70, 20) || ink(g, 70, 22) {
		t.Error("glass is not 2 px")
	}
	if !ink(g, 70, 55) || !ink(g, 70, 65) {
		t.Error("thickness override ignored")
	}
	if ink(g, 18, 60) {
		t.Error("ext 0 still extends the wall")
	}
}

func TestDoorSwingsAndHinges(t *testing.T) {
	// The south wall of a box runs right to left, so a is its far end: the
	// hinge must follow the coordinates, not the wall's direction.
	for _, c := range []struct {
		swing, hinge string
		leaf         image.Point // a point on the open leaf
		notLeaf      image.Point
	}{
		{"N", "a", image.Point{100, 125}, image.Point{136, 125}},
		{"N", "b", image.Point{136, 125}, image.Point{100, 125}},
		{"S", "a", image.Point{100, 155}, image.Point{100, 125}},
	} {
		b := New(40).Box(40, 40, 240, 140)
		tr := b.Trace()
		tr.Walls[2].Openings = []plan.Opening{Door(100, 136, c.swing, c.hinge)}
		g := Render(tr, Options{Size: image.Point{300, 200}})
		if ink(g, 118, 140) {
			t.Errorf("%s/%s: the gap is inked", c.swing, c.hinge)
		}
		if !inkNear(g, c.leaf.X, c.leaf.Y, 1) || inkNear(g, c.notLeaf.X, c.notLeaf.Y, 1) {
			t.Errorf("%s/%s: leaf not at %v", c.swing, c.hinge, c.leaf)
		}
	}
	tr := New(40).Wall("interior", [2]float64{100, 20}, [2]float64{100, 200},
		DoubleDoor(80, 152, "E"), Gap(160, 180)).Trace()
	g := Render(tr, Options{})
	// Two leaves standing east from both jambs, nothing in a plain gap.
	if !inkNear(g, 120, 80, 1) || !inkNear(g, 120, 152, 1) || inkNear(g, 80, 80, 1) {
		t.Error("double door leaves")
	}
	if inkNear(g, 100, 170, 1) || inkNear(g, 110, 170, 3) {
		t.Error("plain gap is not empty")
	}
}

func TestWindows(t *testing.T) {
	tr := New(40).
		Wall("interior", [2]float64{20, 40}, [2]float64{220, 40}, Window(80, 140)).
		Wall("exterior", [2]float64{20, 100}, [2]float64{220, 100}, Window(80, 140)).
		Trace()
	tr.Walls[1].Thickness = 0.4 // 16 px: room for a middle pane line
	g := Render(tr, Options{})
	col := func(y0, y1 int) int { return count(g, image.Rect(110, y0, 111, y1)) }
	if n := col(36, 45); n < 2 || n > 4 {
		t.Errorf("thin wall window: %d inked px across, want two hairlines", n)
	}
	if !ink(g, 110, 100) || col(91, 110) < 3 {
		t.Errorf("thick wall window has no middle line")
	}
}

func TestTextAndFurniture(t *testing.T) {
	r, sz, w, d, rot := 0.5, 0.6, 1.6, 0.8, 30.0
	label := [2]float64{100, 60}
	tr := New(40).
		Box(20, 20, 380, 280).
		Room("Lab", [2]float64{20, 20}, [2]float64{200, 20}, [2]float64{200, 120}, [2]float64{20, 120}).
		Room("Store", [2]float64{200, 150}, [2]float64{300, 150}, [2]float64{300, 250}, [2]float64{200, 250}).
		Room("Flat", [2]float64{0, 0}, [2]float64{10, 0}).
		Room("None").
		Item(plan.Item{Type: "roundTable", X: 300, Y: 80, R: &r}).
		Item(plan.Item{Type: "chair", X: 60, Y: 220, Size: &sz}).
		Item(plan.Item{Type: "desk", X: 150, Y: 200, W: &w, D: &d, Rot: &rot}).
		Item(plan.Item{Type: "stool", X: 340, Y: 220}).
		Trace()
	tr.Rooms[0].Label = &label
	bare := Render(tr, Options{})
	g := Render(tr, Options{Text: true, Furniture: true})
	if count(bare, image.Rect(80, 50, 120, 70)) != 0 || count(g, image.Rect(80, 50, 120, 70)) == 0 {
		t.Error("label not drawn at its label point")
	}
	if count(g, image.Rect(235, 190, 265, 210)) == 0 {
		t.Error("label not drawn at the centroid")
	}
	if !inkNear(g, 320, 80, 1) || inkNear(g, 300, 80, 3) {
		t.Error("round table is not a circle outline")
	}
	if !inkNear(g, 72, 220, 1) || !inkNear(g, 350, 220, 1) {
		t.Error("chair or stool missing")
	}
	if count(g, image.Rect(110, 170, 190, 230)) == 0 {
		t.Error("desk missing")
	}
	// A label outside the image is skipped, not a panic.
	off := [2]float64{-500, -500}
	tr.Rooms[0].Label = &off
	_ = Render(tr, Options{Text: true})
}

func TestCentroid(t *testing.T) {
	c := centroid([][2]float64{{0, 0}, {4, 0}, {4, 2}, {0, 2}})
	if c.X != 2 || c.Y != 1 {
		t.Errorf("rectangle centroid %v", c)
	}
	c = centroid([][2]float64{{0, 0}, {4, 0}})
	if c.X != 2 || c.Y != 0 {
		t.Errorf("degenerate centroid %v", c)
	}
}

func TestFinishing(t *testing.T) {
	tr := fixture(t, "office")
	hard := Render(tr, Options{Text: true})
	for _, v := range hard.Pix {
		if v != 0 && v != 255 {
			t.Fatalf("hard edges have grey %d", v)
		}
	}
	soft := Render(tr, Options{Text: true, AntiAlias: true})
	grey := 0
	for _, v := range soft.Pix {
		if v != 0 && v != 255 {
			grey++
		}
	}
	if grey == 0 {
		t.Error("anti-aliasing has no grey")
	}
	a := Render(tr, Options{Noise: 20, Seed: 3, JPEG: 60})
	b := Render(tr, Options{Noise: 20, Seed: 3, JPEG: 60})
	c := Render(tr, Options{Noise: 20, Seed: 4, JPEG: 60})
	if !bytes.Equal(a.Pix, b.Pix) || bytes.Equal(a.Pix, c.Pix) {
		t.Error("noise is not a function of the seed")
	}
	if bytes.Equal(a.Pix, hard.Pix) {
		t.Error("noise changed nothing")
	}
}

func TestShapesOffTheImage(t *testing.T) {
	tr := New(40).Wall("interior", [2]float64{-400, -400}, [2]float64{-300, -400}).Trace()
	g := Render(tr, Options{Size: image.Point{50, 50}})
	if count(g, g.Rect) != 0 {
		t.Error("a wall off the image left ink")
	}
}

func TestBuilder(t *testing.T) {
	b := New(50)
	b.Wall("interior", [2]float64{0, 0}, [2]float64{10, 0})
	tr := b.Trace()
	b.Wall("interior", [2]float64{0, 0}, [2]float64{0, 10})
	if len(tr.Walls) != 1 || tr.Walls[0].ID != "w1" || len(b.Trace().Walls) != 2 {
		t.Error("Trace is not a snapshot")
	}
	if tr.Calibration.PxPerMetre != [2]float64{50, 50} {
		t.Errorf("calibration %v", tr.Calibration)
	}
	if o := Door(1, 2, "N", "b"); o.Kind != "door" || o.Hinge != "b" || o.Swing != "N" {
		t.Errorf("door %+v", o)
	}
	if o := DoubleDoor(1, 2, "S"); o.Kind != "double-door" || o.Swing != "S" {
		t.Errorf("double door %+v", o)
	}
	if Window(1, 2).Kind != "window" || Gap(1, 2).Kind != "opening" {
		t.Error("kinds")
	}
	if problems := plan.Check(fixtureBox(t)); len(problems) > 0 {
		t.Errorf("a built trace fails plan.Check: %v", problems)
	}
}

func fixtureBox(t *testing.T) *plan.Trace {
	t.Helper()
	return New(40).Box(0, 0, 400, 300).
		Room("Hall", [2]float64{0, 0}, [2]float64{400, 0}, [2]float64{400, 300}, [2]float64{0, 300}).
		Trace()
}
