package cadfront

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/dxf"
	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
	"github.com/weaveplatform/spaceweave/internal/floorgraph"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// These fixtures imitate drawing conventions, not any drawing: layer names
// in the Uniclass 2015 / BS 1192 style, walls drawn as bundles of lines,
// curtain walling on a window layer, swings left as elliptical arcs,
// columns as circles, and a space-management export of outlines and
// written areas over units that are wrong.

func TestUniclassAndStructureLayers(t *testing.T) {
	for name, want := range map[string]role{
		"AR_EF_25_10-X_Existing Walls Internal":    roleWall,
		"Ss_25_10_35_35_Glazed Screen System":      roleGlass,
		"AR_EF_25_10_Glazed Partition":             roleGlass,
		"AR_EF_25_30-M_Internal Door":              roleDoor,
		"AR_EF_25_30-X_Window":                     roleWindow,
		"AR_EF_25_20-M_Structural Element":         roleWall,
		"AR_EF_20_Frames":                          roleWall,
		"AR_Ss_25_50_45_65_Projecting Shading":     roleNotWall,
		"AR_EF_35_10_Stairs":                       roleNotWall,
		"AR_Zz_20_10_70_65-T_Room Names":           roleNotWall,
		"AR_Ss_40_15-X_Existing Furniture":         roleFurniture,
		"AR_Pr_40_20-X_Sanitary Fitting":           roleFurniture,
		"S-COLS":                                   roleWall,
		"A-WALL-DIMS":                              roleNotWall,
		"A-GRID":                                   roleNotWall,
		"A-WALL-PATT":                              roleWall,
		"A-SHADING":                                roleNotWall,
		"AR_XX_99_Something Else":                  roleNone,
	} {
		if got := layerRole(name); got != want {
			t.Errorf("%q: role %d, want %d", name, got, want)
		}
	}
}

// wallNear finds a wall whose centreline runs from a to b (either way)
// within tol pixels.
func wallNear(in floorgraph.Input, a, b point, tol float64) (floorgraph.Segment, bool) {
	return findWall(in, a, b, tol)
}

func TestWallDrawnAsABundleOfLines(t *testing.T) {
	const layer = "AR_EF_25_10-X_Walls"
	d := dxftest.New(4).Layer(layer, 7, 0)
	// Finish, board, stud, board, finish: no two neighbours a wall apart,
	// 130 mm in all.
	for _, y := range []float64{0, 15, 35, 50, 65, 100, 130} {
		d.Add(dxftest.Line(layer, 0, y, 6000, y))
	}
	// A second wall so the drawing has a scale of rooms.
	d.Add(dxftest.Line(layer, 0, 3000, 6000, 3000), dxftest.Line(layer, 0, 3100, 6000, 3100))
	in, o := read(t, d)
	tol := 0.03 * o.PxPerMetre
	w, ok := wallNear(in, mm(o, 0, 65), mm(o, 6000, 65), tol)
	if !ok {
		t.Fatalf("no wall along the bundle's middle; walls %v", in.Walls)
	}
	if got := w.Thickness / o.PxPerMetre; math.Abs(got-0.13) > 0.01 {
		t.Errorf("thickness %.3f m, want the bundle's 0.13", got)
	}
	for _, s := range in.Walls {
		mid := s.A.Add(s.B).Mul(0.5)
		if y := mm(o, 0, 0).Y - mid.Y; y > -tol && y < 0.13*o.PxPerMetre+tol && s != w {
			t.Errorf("a second wall %v inside the bundle", s)
		}
	}
}

func TestCurtainWallOnAWindowLayer(t *testing.T) {
	const wall, win = "AR_EF_25_10-X_Walls", "AR_EF_25_30-X_Window"
	d := dxftest.New(4).Layer(wall, 7, 0).Layer(win, 4, 0)
	// A room's three solid sides; the fourth, 8 m, is curtain walling:
	// frame, glass and frame lines, mullions every 1.5 m.
	d.Add(dxftest.Line(wall, 0, 0, 0, 5000), dxftest.Line(wall, 200, 200, 200, 5000))
	d.Add(dxftest.Line(wall, 8000, 0, 8000, 5000), dxftest.Line(wall, 7800, 200, 7800, 5000))
	d.Add(dxftest.Line(wall, 0, 0, 8000, 0), dxftest.Line(wall, 200, 200, 7800, 200))
	for _, y := range []float64{5000, 5050, 5100} {
		d.Add(dxftest.Line(win, 0, y, 8000, y))
	}
	for x := 1500.0; x < 8000; x += 1500 {
		d.Add(dxftest.Line(win, x, 5000, x, 5100))
	}
	in, o := read(t, d)
	var glass float64
	for _, s := range in.Walls {
		if s.Glass {
			glass += s.B.Sub(s.A).Len()
		}
	}
	if got := glass / o.PxPerMetre; got < 7 {
		t.Errorf("%.1f m of glass wall, want the 8 m curtain wall; walls %v", got, in.Walls)
	}
	for _, op := range in.Openings {
		if op.Kind == floorgraph.Window {
			t.Errorf("curtain walling read as a window: %v", op)
		}
	}
	res, err := floorgraph.Build(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Trace.Rooms) != 1 {
		t.Errorf("%d rooms, want the one the curtain wall closes", len(res.Trace.Rooms))
	}
}

func TestEllipticalSwingIsADoor(t *testing.T) {
	const wall, door = "AR_EF_25_10-X_Walls", "AR_EF_25_30-M_Internal Door"
	d := dxftest.New(4).Layer(wall, 7, 0).Layer(door, 3, 0)
	// A 100 mm partition along y = 0 with a 900 mm doorway at 2000–2900.
	for _, y := range []float64{-50, 50} {
		d.Add(dxftest.Line(wall, 0, y, 2000, y), dxftest.Line(wall, 2900, y, 6000, y))
	}
	// Hinged at (2000, 50), the leaf open along +y, the swing a quarter
	// ellipse (ratio 1) from the leaf's end round to the far jamb.
	d.Add(dxftest.Line(door, 2000, 50, 2000, 950))
	d.Add(dxftest.E("ELLIPSE", 8, door, 10, 2000.0, 20, 50.0, 30, 0.0,
		11, 900.0, 21, 0.0, 31, 0.0, 40, 1.0, 41, 0.0, 42, math.Pi/2))
	in, o := read(t, d)
	tol := 0.05 * o.PxPerMetre
	for _, op := range in.Openings {
		if op.Kind != floorgraph.Door {
			continue
		}
		hinge, other := op.A, op.B
		if !op.HingeA {
			hinge, other = op.B, op.A
		}
		if close2(hinge, mm(o, 2000, 0), tol) && close2(other, mm(o, 2900, 0), tol) && op.Leafed {
			// The leaf opens to +y, which is up the image.
			if op.Leaf.Y >= hinge.Y {
				t.Errorf("leaf side %v, want above the wall", op.Leaf)
			}
			return
		}
	}
	t.Errorf("no door hinged at the ellipse's centre: %v", in.Openings)
}

func TestNarrowLeafOnADoorLayer(t *testing.T) {
	const wall, door = "A-WALL", "A-DOOR"
	d := dxftest.New(4).Layer(wall, 7, 0).Layer(door, 3, 0)
	for _, y := range []float64{-50, 50} {
		d.Add(dxftest.Line(wall, 0, y, 2000, y), dxftest.Line(wall, 2500, y, 6000, y))
	}
	// A 500 mm cupboard leaf, narrower than a door is by its swing alone.
	d.Add(dxftest.Line(door, 2000, 50, 2000, 550), dxftest.Arc(door, 2000, 50, 500, 0, 90))
	in, o := read(t, d)
	for _, op := range in.Openings {
		if op.Kind == floorgraph.Door && math.Abs(op.B.Sub(op.A).Len()/o.PxPerMetre-0.5) < 0.05 {
			return
		}
	}
	t.Errorf("no 500 mm door: %v", in.Openings)
}

func TestRoundColumnsAndBridgedGaps(t *testing.T) {
	const wall, cols = "A-WALL", "S-COLS"
	d := dxftest.New(4).Layer(wall, 7, 0).Layer(cols, 1, 0)
	// A wall broken for 300 mm by a riser: no door, so one wall.
	for _, y := range []float64{-50, 50} {
		d.Add(dxftest.Line(wall, 0, y, 3000, y), dxftest.Line(wall, 3300, y, 6000, y))
	}
	d.Add(dxftest.E("CIRCLE", 8, cols, 10, 3000.0, 20, 2000.0, 40, 200.0))
	in, o := read(t, d)
	tol := 0.02 * o.PxPerMetre
	// Every point along the wall's line is in some wall.
	for x := 0.0; x <= 6000; x += 50 {
		p := mm(o, x, 0)
		var in2 bool
		for _, s := range in.Walls {
			in2 = in2 || segDist(p, s.A, s.B) <= tol
		}
		if !in2 {
			t.Fatalf("the wall is open at x = %.0f mm: %v", x, in.Walls)
		}
	}
	var col bool
	for _, s := range in.Walls {
		mid := s.A.Add(s.B).Mul(0.5)
		col = col || close2(mid, mm(o, 3000, 2000), tol) && math.Abs(s.Thickness/o.PxPerMetre-0.4) < 0.01
	}
	if !col {
		t.Errorf("no 400 mm column at the circle: %v", in.Walls)
	}
}

func TestStatedArea(t *testing.T) {
	for s, want := range map[string]float64{
		"16.8 sf":     16.8 * sqFt,
		"1,204 SF":    1204 * sqFt,
		"12 m²":       12,
		"12.5m2":      12.5,
		"3 sq ft":     3 * sqFt,
		"40 sqm":      40,
		"Room 12 m²":  0,
		"4D-321":      0,
		"0 m2":        0,
		"12 metres":   0,
		"1.2.3 sf":    0,
		"99999999 m2": 99999999,
	} {
		got, ok := statedArea(s)
		if ok != (want > 0) || math.Abs(got-want) > 1e-9 {
			t.Errorf("%q: %v %v, want %v", s, got, ok, want)
		}
	}
}

// areaPlan is a space-management export: room outlines on one layer,
// workplace outlines on another, numbers and areas written inside, and
// $INSUNITS saying inches over coordinates in millimetres.
func areaPlan(withDesks bool) *dxftest.Doc {
	const rooms, nums, areas, desks = "XX Area Polygons", "XX Area Numbers", "XX Area SqFt", "XX Space Polygons"
	d := dxftest.New(1).Layer(rooms, 7, 0).Layer(nums, 7, 0).Layer(areas, 7, 0).Layer(desks, 5, 0)
	room := func(x0, y0, x1, y1 float64, num string) {
		d.Add(dxftest.Rect(rooms, x0, y0, x1, y1))
		d.Add(dxftest.Text(nums, x0+300, y1-500, 200, num))
		sf := (x1 - x0) * (y1 - y0) / 1e6 / sqFt
		d.Add(dxftest.Text(areas, x0+300, y1-900, 150, strconv.FormatFloat(sf, 'f', 1, 64)+" sf"))
	}
	// Two offices a 100 mm wall apart, and an open area below both that
	// meets them with no wall drawn.
	room(0, 3000, 4000, 6000, "R01")
	room(4100, 3000, 8000, 6000, "R02")
	room(0, 0, 8000, 3000, "OP01")
	if withDesks {
		for i := range 3 {
			x := 500 + float64(i)*2000
			d.Add(dxftest.Rect(desks, x, 500, x+1600, 1300))
			d.Add(dxftest.Text(nums, x+100, 800, 250, "D-"+string(rune('1'+i))))
			d.Add(dxftest.Text(areas, x+100, 600, 100, "13.8 sf"))
		}
	}
	return d
}

func TestAreaPlanCorrectsItsUnitsAndHasRooms(t *testing.T) {
	in, o := read(t, areaPlan(true))
	// The header's inches would make 1 unit 25.4 mm; the written areas
	// say millimetres.
	wantPx := o.View.Factor() / 0.001
	if got := in.Scale.PxPerMetre[0]; math.Abs(got/wantPx-1) > 1e-9 {
		t.Errorf("scale %.4f px/m, want %.4f from the written areas (header said %.4f)",
			got, wantPx, o.PxPerMetre)
	}
	if in.Scale.Exact || !strings.Contains(in.Scale.Evidence, "areas written") {
		t.Errorf("scale %+v should say where it came from", in.Scale)
	}
	if !hasNote(in, "written areas were believed") || !hasNote(in, "no walls") {
		t.Errorf("notes %q", in.Notes)
	}
	desks := 0
	for _, it := range in.Furniture {
		if it.Type == "desk" && math.Abs(*it.W-1.6) < 0.01 && math.Abs(*it.D-0.8) < 0.01 {
			desks++
		}
	}
	if desks != 3 {
		t.Errorf("%d desks of 1.6 × 0.8 m, want 3: %+v", desks, in.Furniture)
	}
	for _, l := range in.Labels {
		if strings.HasSuffix(l.Text, " sf") || strings.HasPrefix(l.Text, "D-") {
			t.Errorf("label %q is an area or a desk's, not a room name", l.Text)
		}
	}
	res, err := floorgraph.Build(in)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, r := range res.Trace.Rooms {
		names[r.Name] = true
	}
	for _, n := range []string{"R01", "R02", "OP01"} {
		if !names[n] {
			t.Errorf("room %s missing; rooms %v", n, names)
		}
	}
	// The wall between the offices is one wall, 100 mm, on its centreline.
	tol := 0.03 * o.PxPerMetre
	w, ok := wallNear(in, mm(o, 4050, 3000), mm(o, 4050, 6000), tol)
	if !ok || math.Abs(w.Thickness/in.Scale.PxPerMetre[0]-0.1) > 0.01 {
		t.Errorf("wall between the offices %v %v", w, ok)
	}
}

func TestAreaPlanOnlyWithoutWalls(t *testing.T) {
	// A drawing with walls keeps its own: outlines on an area layer are
	// not walls there, and areas that agree with the units change nothing.
	d := dxftest.Office(4)
	d.Layer("A-AREA", 7, 0).Add(dxftest.Rect("A-AREA", 100, 100, 5900, 3900))
	d.Add(dxftest.Text("A-AREA-IDEN", 500, 500, 100, "22.04 m²"))
	d.Add(dxftest.Text("A-AREA-IDEN", 500, 700, 100, "22.04 m²"))
	d.Add(dxftest.Text("A-AREA-IDEN", 500, 900, 100, "22.04 m²"))
	in, o := read(t, d)
	if in.Scale.PxPerMetre[0] != o.PxPerMetre || !in.Scale.Exact {
		t.Errorf("scale %+v, want the drawing's %.4f", in.Scale, o.PxPerMetre)
	}
	if hasNote(in, "no walls") {
		t.Error("an office with walls isn't an area plan")
	}
	// Labels that disagree among themselves outvote nothing.
	r := &reader{}
	if u, n := r.areaUnit(nil); u != 0 || n != 0 {
		t.Errorf("no outlines: %v %v", u, n)
	}
}

func TestUncovered(t *testing.T) {
	got := uncovered(0, 10, [][2]float64{{6, 8}, {1, 3}, {2, 4}})
	want := [][2]float64{{0, 1}, {4, 6}, {8, 10}}
	if len(got) != len(want) {
		t.Fatalf("%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%v, want %v", got, want)
		}
	}
	if got := uncovered(0, 10, [][2]float64{{-1, 11}}); len(got) != 0 {
		t.Errorf("all covered: %v", got)
	}
}

func segDist(p, a, b point) float64 {
	d := b.Sub(a)
	f := math.Max(0, math.Min(1, dot(p.Sub(a), d)/dot(d, d)))
	return p.Sub(a.Add(d.Mul(f))).Len()
}

func TestAreaPlanIsDeterministic(t *testing.T) {
	a, _ := read(t, areaPlan(true))
	b, _ := read(t, areaPlan(true))
	if !reflect.DeepEqual(a, b) {
		t.Error("two reads of the same area plan differ")
	}
	// Without desk outlines there are no desks, and the rooms still close.
	c, _ := read(t, areaPlan(false))
	if len(c.Furniture) != 0 || len(c.Walls) == 0 {
		t.Errorf("%d furniture, %d walls", len(c.Furniture), len(c.Walls))
	}
}

func FuzzStatedArea(f *testing.F) {
	for _, s := range []string{"16.8 sf", "1,204 m²", "12m2", "", "sf", "9,99 sq ft"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if v, ok := statedArea(s); ok && !(v > 0) {
			t.Errorf("%q: %v", s, v)
		}
	})
}

// FuzzRead: whatever a DXF holds, reading it doesn't panic.
func FuzzRead(f *testing.F) {
	f.Add(dxftest.Office(4).Bytes())
	f.Add(areaPlan(true).Bytes())
	f.Fuzz(func(t *testing.T, b []byte) {
		doc, err := dxf.Read(bytes.NewReader(b))
		if err != nil {
			return
		}
		o, err := Frame(doc, vector.Options{MaxSide: 256})
		if err != nil {
			return
		}
		_, _ = Read(doc, o)
	})
}

func TestPartlyPairedWallKeepsItsSingleLineContinuation(t *testing.T) {
	const layer = "A-WALL"
	d := dxftest.New(4).Layer(layer, 7, 0)
	d.Add(dxftest.Line(layer, 0, 0, 6000, 0), dxftest.Line(layer, 0, 100, 2000, 100))
	d.Add(dxftest.Line(layer, 0, 3000, 6000, 3000), dxftest.Line(layer, 0, 3100, 6000, 3100))
	in, o := readContinuations(t, d)
	if _, ok := wallNear(in, mm(o, 2000, 0), mm(o, 6000, 0), .11*o.PxPerMetre); !ok {
		t.Fatal("pairing the first two metres discarded the four-metre wall continuation")
	}
}

func TestPairedContinuationStillRecognizesADoorLeaf(t *testing.T) {
	const layer = "A-WALL"
	d := dxftest.New(4).Layer(layer, 7, 0)
	for _, y := range []float64{-50, 50} {
		d.Add(dxftest.Line(layer, -2000, y, 0, y), dxftest.Line(layer, 900, y, 3000, y))
	}
	// The leaf runs beside a short frame stroke. Pairing that stroke must
	// not lose the leaf provenance on the remaining 700 mm continuation.
	d.Add(dxftest.Line(layer, 0, 50, 0, 950), dxftest.Line(layer, 100, 50, 100, 250))
	d.Add(dxftest.Line(layer, 0, 50, 0, 150))
	d.Add(dxftest.Arc(layer, 0, 50, 900, 0, 90))
	in, o := readContinuations(t, d)
	if _, ok := wallNear(in, mm(o, 0, 250), mm(o, 0, 950), .05*o.PxPerMetre); ok {
		t.Fatal("a partly paired door leaf was emitted as a wall continuation")
	}
}

func readContinuations(t *testing.T, d *dxftest.Doc) (floorgraph.Input, Options) {
	t.Helper()
	doc := parse(t, d)
	o, err := Frame(doc, vector.Options{})
	if err != nil { t.Fatal(err) }
	path := filepath.Join(t.TempDir(), "continuations.dxf")
	if err := os.WriteFile(path, d.Bytes(), 0o600); err != nil { t.Fatal(err) }
	in, err := ReadFile(path, Options{WallContinuations: true})
	if err != nil { t.Fatal(err) }
	return in, o
}
