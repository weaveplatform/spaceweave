package cadfront

import (
	"bytes"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/dxf"
	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
	"github.com/weaveplatform/spaceweave/internal/floorgraph"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// testRaster is a small image, so the tests run fast; the scale only
// changes how many pixels a millimetre is.
var testRaster = vector.Options{MaxSide: 1224, Margin: 12}

func parse(t *testing.T, d *dxftest.Doc) *cad.Document {
	t.Helper()
	doc, err := dxf.Read(bytes.NewReader(d.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

// read frames a fixture as the converter would and reads it.
func read(t *testing.T, d *dxftest.Doc) (floorgraph.Input, Options) {
	t.Helper()
	doc := parse(t, d)
	o, err := Frame(doc, testRaster)
	if err != nil {
		t.Fatal(err)
	}
	in, err := Read(doc, o)
	if err != nil {
		t.Fatal(err)
	}
	return in, o
}

// mm maps fixture millimetres to image pixels.
func mm(o Options, x, y float64) point { return o.View.Apply(point{X: x, Y: y}) }

func close2(a, b point, tol float64) bool { return a.Sub(b).Len() <= tol }

func findWall(in floorgraph.Input, a, b point, tol float64) (floorgraph.Segment, bool) {
	for _, w := range in.Walls {
		if close2(w.A, a, tol) && close2(w.B, b, tol) || close2(w.A, b, tol) && close2(w.B, a, tol) {
			return w, true
		}
	}
	return floorgraph.Segment{}, false
}

func findOpening(in floorgraph.Input, a, b point, tol float64) (floorgraph.Opening, bool) {
	for _, op := range in.Openings {
		if close2(op.A, a, tol) && close2(op.B, b, tol) {
			return op, true
		}
	}
	return floorgraph.Opening{}, false
}

func hasNote(in floorgraph.Input, sub string) bool {
	return slices.ContainsFunc(in.Notes, func(n string) bool { return strings.Contains(n, sub) })
}

func TestOfficeWalls(t *testing.T) {
	in, o := read(t, dxftest.Office(4))
	ppm := o.PxPerMetre
	tol := 0.01 * ppm // a centimetre
	cases := []struct {
		name         string
		a, b         [2]float64
		thick        float64
		glass, heavy bool
	}{
		{"west, hatched", [2]float64{0, 0}, [2]float64{0, 8000}, 0.2, false, true},
		{"south, west of the entrance", [2]float64{0, 0}, [2]float64{2000, 0}, 0.2, false, false},
		{"south, from the entrance to the T", [2]float64{2900, 0}, [2]float64{6000, 0}, 0.2, false, false},
		{"south, east of the T", [2]float64{6000, 0}, [2]float64{12000, 0}, 0.2, false, false},
		{"north, between windows", [2]float64{2700, 8000}, [2]float64{6000, 8000}, 0.2, false, false},
		{"east, below the window", [2]float64{12000, 0}, [2]float64{12000, 1500}, 0.2, false, false},
		{"east, above the window to the T", [2]float64{12000, 2700}, [2]float64{12000, 4000}, 0.2, false, false},
		{"partition below its door", [2]float64{6000, 0}, [2]float64{6000, 2000}, 0.1, false, false},
		{"partition above its door", [2]float64{6000, 2900}, [2]float64{6000, 4000}, 0.1, false, false},
		{"partition north", [2]float64{6000, 4000}, [2]float64{6000, 8000}, 0.1, false, false},
		{"partition east, west of the double door", [2]float64{6000, 4000}, [2]float64{8000, 4000}, 0.1, false, false},
		{"glass west of its door", [2]float64{0, 4000}, [2]float64{4000, 4000}, 0.02, true, false},
		{"glass east of its door", [2]float64{4900, 4000}, [2]float64{6000, 4000}, 0.02, true, false},
	}
	for _, c := range cases {
		w, ok := findWall(in, mm(o, c.a[0], c.a[1]), mm(o, c.b[0], c.b[1]), tol)
		switch {
		case !ok:
			t.Errorf("%s: no wall %v–%v", c.name, c.a, c.b)
		case math.Abs(w.Thickness-c.thick*ppm) > tol || w.Glass != c.glass:
			t.Errorf("%s: thickness %.2f px glass %v, want %.2f px glass %v",
				c.name, w.Thickness, w.Glass, c.thick*ppm, c.glass)
		case c.heavy != (w.Weight > weightPair):
			t.Errorf("%s: weight %.2f", c.name, w.Weight)
		}
	}
	if len(in.Walls) != 18 {
		t.Errorf("%d walls, want the 18 pieces between openings and junctions", len(in.Walls))
	}
	if in.Scale.PxPerMetre != [2]float64{ppm, ppm} || !in.Scale.Exact || in.Scale.Evidence != "from the drawing's units" {
		t.Errorf("scale %+v", in.Scale)
	}
	if in.Size != o.Size || in.Size[0] == 0 {
		t.Errorf("size %v, want %v", in.Size, o.Size)
	}
	if in.Coverage < 0.95 || in.Coverage > 1 {
		t.Errorf("coverage %.2f, want nearly everything explained", in.Coverage)
	}
	if len(in.Notes) != 1 {
		t.Errorf("notes %q, want only the fridge's", in.Notes)
	}
}

func TestOfficeOpenings(t *testing.T) {
	in, o := read(t, dxftest.Office(4))
	tol := 0.01 * o.PxPerMetre
	leaf := 900 * math.Sqrt2 / 2 // an arc's midpoint off its hinge, along each axis
	cases := []struct {
		name   string
		kind   string
		a, b   [2]float64
		leaf   [2]float64
		leafed bool
	}{
		{"entrance", floorgraph.Door, [2]float64{2000, 0}, [2]float64{2900, 0},
			[2]float64{2000 + leaf, leaf}, true},
		{"partition door, turned", floorgraph.Door, [2]float64{6000, 2900}, [2]float64{6000, 2000},
			[2]float64{6000 + leaf, 2900 - leaf}, true},
		{"glass door, mirrored", floorgraph.Door, [2]float64{4900, 4000}, [2]float64{4000, 4000},
			[2]float64{4900 - leaf, 4000 + leaf}, true},
		{"double door", floorgraph.DoubleDoor, [2]float64{8000, 4000}, [2]float64{9800, 4000},
			[2]float64{8000 + leaf, 4000 + leaf}, true},
		{"north window", floorgraph.Window, [2]float64{1500, 8000}, [2]float64{2700, 8000}, [2]float64{}, false},
		{"north window east", floorgraph.Window, [2]float64{7500, 8000}, [2]float64{8700, 8000}, [2]float64{}, false},
		{"east window, turned", floorgraph.Window, [2]float64{12000, 1500}, [2]float64{12000, 2700}, [2]float64{}, false},
	}
	for _, c := range cases {
		op, ok := findOpening(in, mm(o, c.a[0], c.a[1]), mm(o, c.b[0], c.b[1]), tol)
		if !ok {
			t.Errorf("%s: no opening %v–%v in %+v", c.name, c.a, c.b, in.Openings)
			continue
		}
		if op.Kind != c.kind || op.Leafed != c.leafed || op.HingeA != (c.kind == floorgraph.Door) {
			t.Errorf("%s: %+v", c.name, op)
		}
		if c.leafed && !close2(op.Leaf, mm(o, c.leaf[0], c.leaf[1]), tol) {
			t.Errorf("%s: leaf at %v, want %v", c.name, op.Leaf, mm(o, c.leaf[0], c.leaf[1]))
		}
		if op.Weight < weightSpan {
			t.Errorf("%s: weight %.2f", c.name, op.Weight)
		}
	}
	if len(in.Openings) != len(cases) {
		t.Errorf("%d openings, want %d", len(in.Openings), len(cases))
	}
}

func TestOfficeLabelsAndFurniture(t *testing.T) {
	in, o := read(t, dxftest.Office(4))
	names := map[string]floorgraph.Label{}
	for _, l := range in.Labels {
		names[l.Text] = l
	}
	if len(in.Labels) != 4 {
		t.Errorf("labels %+v, want the four room names (no door tag, no dimension)", in.Labels)
	}
	for name, box := range dxftest.OfficeRooms {
		l, ok := names[name]
		if !ok {
			t.Errorf("no label %q", name)
			continue
		}
		lo, hi := mm(o, box[0], box[3]), mm(o, box[2], box[1])
		if l.At.X < lo.X || l.At.X > hi.X || l.At.Y < lo.Y || l.At.Y > hi.Y {
			t.Errorf("%q at %v, outside its room %v–%v", name, l.At, lo, hi)
		}
		if math.Abs(l.Height-0.25*o.PxPerMetre) > 0.01 {
			t.Errorf("%q height %.2f px, want 250 mm", name, l.Height)
		}
	}
	// "Meeting" is left-aligned at (1000, 7000): its middle is half its
	// estimated length on, and about half its height up.
	if want := mm(o, 1000+7*0.6*250/2, 7000+250*0.375); !close2(names["Meeting"].At, want, 0.5) {
		t.Errorf("Meeting at %v, want %v", names["Meeting"].At, want)
	}

	count := map[string]int{}
	for _, it := range in.Furniture {
		count[it.Type]++
	}
	want := map[string]int{"desk": 7, "chair": 11, "roundTable": 1, "sink": 1, "counter": 1, "sofa": 1, "plant": 1}
	if !reflect.DeepEqual(count, want) {
		t.Errorf("furniture %v, want %v", count, want)
	}
	if !hasNote(in, "1 furniture blocks had names that don't say") {
		t.Errorf("notes %q, want the fridge counted", in.Notes)
	}
	tol := 0.01 * o.PxPerMetre
	for _, it := range in.Furniture {
		at := point{X: it.X, Y: it.Y}
		switch {
		case it.Type == "roundTable":
			if *it.R != 0.6 || !close2(at, mm(o, 3000, 6000), tol) {
				t.Errorf("table %+v r %v", it, *it.R)
			}
		case it.Type == "counter":
			// Turned a quarter: its width runs up the image.
			if *it.W != 2.4 || *it.D != 0.6 || *it.Rot != 270 || !close2(at, mm(o, 11500, 1500), tol) {
				t.Errorf("counter %+v", it)
			}
		case it.Type == "sink":
			// No angle field, so its sides follow the image's axes.
			if *it.W != 0.5 || *it.D != 0.8 {
				t.Errorf("sink %v × %v, want 0.5 × 0.8", *it.W, *it.D)
			}
		case it.Type == "chair" && close2(at, mm(o, 7900, 6300), tol):
			// Mirrored and turned to face the desk, west of it.
			if *it.Facing != 180 || *it.Size != 0.5 {
				t.Errorf("director's chair facing %v", *it.Facing)
			}
		case it.Type == "chair" && close2(at, mm(o, 3000, 5100), tol):
			if *it.Facing != 270 {
				t.Errorf("chair south of the table faces %v, want up the image", *it.Facing)
			}
		case it.Type == "desk" && close2(at, mm(o, 1400, 1400), tol):
			if *it.W != 1.6 || *it.D != 0.8 || *it.Rot != 0 {
				t.Errorf("array desk %+v", it)
			}
		}
	}
}

func TestDeterministic(t *testing.T) {
	a, _ := read(t, dxftest.Office(4))
	b, _ := read(t, dxftest.Office(4))
	if !reflect.DeepEqual(a, b) {
		t.Error("two reads of the same drawing differ")
	}
	// Binary DXF is the same drawing.
	doc, err := dxf.Read(bytes.NewReader(dxftest.Office(4).Binary()))
	if err != nil {
		t.Fatal(err)
	}
	o, _ := Frame(doc, testRaster)
	c, err := Read(doc, o)
	if err != nil || !reflect.DeepEqual(a, c) {
		t.Errorf("binary read differs (err %v)", err)
	}
}

func TestUnitless(t *testing.T) {
	// Without $INSUNITS the converter assumes millimetres from the size,
	// which is a scale, but not an exact one.
	in, o := read(t, dxftest.Office(-1))
	if o.Exact || in.Scale.Exact || in.Scale.Evidence != "assumed from the drawing's size" || len(in.Furniture) == 0 {
		t.Errorf("scale %+v, %d furniture", in.Scale, len(in.Furniture))
	}

	// With no scale at all, door swings set the thresholds, and furniture,
	// whose sizes would be guesses, is left out.
	doc := parse(t, dxftest.Office(-1))
	o.PxPerMetre = 0
	in, err := Read(doc, o)
	if err != nil {
		t.Fatal(err)
	}
	if in.Scale.Known() || len(in.Furniture) != 0 || len(in.Walls) != 18 || len(in.Openings) != 7 {
		t.Errorf("scale %+v, %d furniture, %d walls, %d openings", in.Scale, len(in.Furniture), len(in.Walls), len(in.Openings))
	}
	if !hasNote(in, "door swings as 0.9 m") || !hasNote(in, "24 furniture blocks were left out") {
		t.Errorf("notes %q", in.Notes)
	}
}

func TestUnlayered(t *testing.T) {
	check := func(name string, in floorgraph.Input, o Options) {
		t.Helper()
		tol := 0.02 * o.View.Factor() * 1000
		op, ok := findOpening(in, mm(o, 5000, 1900), mm(o, 5000, 1000), tol)
		if !ok || op.Kind != floorgraph.Door || op.Weight != weightSwingBlock {
			t.Errorf("%s: openings %+v, want the swing block's door", name, in.Openings)
		}
		if _, ok := findWall(in, mm(o, 5000, 1900), mm(o, 5000, 6000), tol); !ok {
			t.Errorf("%s: no partition north of the door in %+v", name, in.Walls)
		}
		for _, w := range in.Walls {
			if w.Weight != weightGuessPair {
				t.Errorf("%s: wall weight %.2f, want a guess's", name, w.Weight)
			}
		}
		if len(in.Labels) != 2 || !hasNote(in, "No layer is named for walls") {
			t.Errorf("%s: labels %+v notes %q", name, in.Labels, in.Notes)
		}
	}
	in, o := read(t, dxftest.Unlayered(4))
	check("millimetres", in, o)

	// Unknown units and no door named as one: the commonest gap between
	// parallel lines is taken as a wall's thickness.
	o.PxPerMetre = 0
	in, err := Read(parse(t, dxftest.Unlayered(-1)), o)
	if err != nil {
		t.Fatal(err)
	}
	check("unitless", in, o)
	if !hasNote(in, "commonest gap between parallel lines") {
		t.Errorf("notes %q", in.Notes)
	}
}

func TestPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("times a large drawing")
	}
	d := dxftest.Grid(58, 58)
	doc := parse(t, d)
	lines := 0
	for _, e := range doc.Model {
		if _, ok := e.(*cad.Line); ok {
			lines++
		}
	}
	o, err := Frame(doc, testRaster)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	in, err := Read(doc, o)
	took := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d lines, %d entities: %d walls, %d openings, %d labels in %v",
		lines, len(doc.Model), len(in.Walls), len(in.Openings), len(in.Labels), took)
	if lines < 20000 || len(in.Openings) != 58*58 || len(in.Labels) != 58*58 {
		t.Errorf("%d lines, %d openings, %d labels", lines, len(in.Openings), len(in.Labels))
	}
	// A few hundred milliseconds on a laptop; the bound leaves room for a
	// loaded CI runner.
	if took > 5*time.Second && !raceDetector {
		t.Errorf("took %v", took)
	}
}

func TestReadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ground floor.dxf")
	if err := os.WriteFile(path, dxftest.Office(4).Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	in, err := ReadFile(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	full, err := Frame(parse(t, dxftest.Office(4)), vector.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if in.Title != "ground floor" || in.Size != full.Size || len(in.Walls) != 18 {
		t.Errorf("title %q size %v (want %v), %d walls", in.Title, in.Size, full.Size, len(in.Walls))
	}
	// Given options are used as they are.
	in, err = ReadFile(path, Options{View: full.View, Size: [2]int{1, 2}, Title: "GF"})
	if err != nil || in.Title != "GF" || in.Size != [2]int{1, 2} || in.Scale.Known() {
		t.Errorf("with options: %v, %q %v %+v", err, in.Title, in.Size, in.Scale)
	}

	if _, err := ReadFile(filepath.Join(dir, "missing.dxf"), Options{}); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing file: %v", err)
	}
	bad := filepath.Join(dir, "bad.dxf")
	if err := os.WriteFile(bad, []byte("not a drawing"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(bad, Options{}); err == nil || !strings.Contains(err.Error(), "read DXF") {
		t.Errorf("bad file: %v", err)
	}
	empty := filepath.Join(dir, "empty.dxf")
	if err := os.WriteFile(empty, dxftest.New(4).Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(empty, Options{}); !errors.Is(err, vector.ErrEmpty) {
		t.Errorf("empty drawing: %v", err)
	}
}

func TestReadErrors(t *testing.T) {
	doc := parse(t, dxftest.Office(4))
	if _, err := Read(doc, Options{}); !errors.Is(err, ErrView) {
		t.Errorf("zero view: %v", err)
	}
	huge := cad.NewDocument()
	huge.Blocks["B"] = &cad.Block{Name: "B"}
	huge.Model = []cad.Entity{&cad.Insert{Block: "B", Columns: 3000, Rows: 1000}}
	if _, err := Read(huge, Options{View: vector.Identity}); !errors.Is(err, cad.ErrTooBig) {
		t.Errorf("huge: %v", err)
	}
	if _, err := Frame(huge, testRaster); !errors.Is(err, cad.ErrTooBig) {
		t.Errorf("Frame huge: %v", err)
	}
}
