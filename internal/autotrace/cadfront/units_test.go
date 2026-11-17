package cadfront

import (
	"math"
	"reflect"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/floorgraph"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

func TestTokens(t *testing.T) {
	cases := map[string][]string{
		"A-WALL-FULL":   {"A", "WALL", "FULL"},
		"TaskChair02":   {"TASK", "CHAIR", "02"},
		"DOOR900":       {"DOOR", "900"},
		"xref|A-Glaz":   {"XREF", "A", "GLAZ"},
		"Tür_innen":     {"TÜR", "INNEN"},
		"  ":            nil,
		"MÖBEL Büro 1x": {"MÖBEL", "BÜRO", "1", "X"},
	}
	for in, want := range cases {
		if got := tokens(in); !reflect.DeepEqual(got, want) {
			t.Errorf("tokens(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLayerRole(t *testing.T) {
	cases := map[string]role{
		"A-WALL": roleWall, "S-WALL-CONC": roleWall, "MUR": roleWall, "Wände": roleWall, "x-wall-ext": roleWall,
		"A-GLAZ": roleGlass, "A-GLAZ-CWMG": roleGlass, "Curtainwall": roleGlass,
		"A-GLAZ-WIND": roleWindow, "FENSTER": roleWindow, "WINDOWS": roleWindow,
		"A-DOOR": roleDoor, "DR": roleDoor, "Türen": roleDoor, "PUERTAS": roleDoor,
		"A-FURN-FREE": roleFurniture, "MOBILIER": roleFurniture,
		"A-ANNO-DIMS": roleNotWall, "DEFPOINTS": roleNotWall, "A-FLOR-IDEN": roleNotWall, "GRID": roleNotWall,
		"0": roleNone, "DRAWING": roleNone, "L1": roleNone,
	}
	for in, want := range cases {
		if got := layerRole(in); got != want {
			t.Errorf("layerRole(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestBlockRole(t *testing.T) {
	cases := []struct {
		block, layer string
		want         role
	}{
		{"DOOR_SGL_900", "0", roleDoor},
		{"M_Single-Flush", "A-DOOR", roleDoor},
		{"WIN1200", "0", roleWindow},
		{"x", "A-GLAZ-WIND", roleWindow},
		{"TaskChair", "0", roleFurniture},
		{"FRIDGE", "A-FURN", roleFurniture},
		{"FRIDGE", "0", roleFurniture},
		{"Bed", "0", roleNone},
		{"*U12", "A-WALL", roleNone},
	}
	for _, c := range cases {
		if got := layerBlockRole(c.block, c.layer); got != c.want {
			t.Errorf("blockRole(%q, %q) = %v, want %v", c.block, c.layer, got, c.want)
		}
	}
}

func TestFurnitureType(t *testing.T) {
	cases := map[string]string{
		"Cupboard-2door": "cabinet", "Keyboard": "", "WHITEBOARD": "board", "Pin_Board": "board",
		"Arm_Chair": "armchair", "OfficeChair": "chair", "Desk1600": "desk", "WS-01": "desk",
		"MeetingTable": "table", "Sofa3": "sofa", "WC": "toilet", "Toilet_Pan": "toilet",
		"Basin": "basin", "WHB": "basin", "Kitchen Sink": "counter", "SINK": "sink", "Bathtub": "tub",
		"Tube": "", "Plant_pot": "plant", "Stair": "stairs", "Bed": "", "Copier": "copier",
		"Locker": "storage", "Hocker": "stool",
	}
	for in, want := range cases {
		if got := furnitureType(in); got != want {
			t.Errorf("furnitureType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPairRules(t *testing.T) {
	doc := metricDoc(
		// Three parallel lines: one wall with a line through it (a lining,
		// a cavity), not two walls side by side with a slit between.
		ln("A-WALL", 0, 0, 5, 0), ln("A-WALL", 0, 0.15, 5, 0.15), ln("A-WALL", 0, 0.3, 5, 0.3),
		// A face drawn in pieces, one of them twice, is one face.
		ln("A-WALL", 10, 0, 12, 0), ln("A-WALL", 10, 0, 12, 0), ln("A-WALL", 12, 0, 14, 0),
		ln("A-WALL", 10, 0.2, 14, 0.2),
		// Parallel lines a corridor apart are not a wall.
		ln("A-WALL", 20, 0, 25, 0), ln("A-WALL", 20, 1.5, 25, 1.5),
	)
	in := readMetric(t, doc)
	for _, w := range []struct {
		a, b  point
		thick float64
	}{
		{at(0, 0.15), at(5, 0.15), 30},
		{at(10, 0.1), at(14, 0.1), 20},
	} {
		if got, ok := findWall(in, w.a, w.b, 0.01); !ok || math.Abs(got.Thickness-w.thick) > 1e-6 {
			t.Errorf("wall %v–%v: %+v, %v", w.a, w.b, got, ok)
		}
	}
	paired := 0
	for _, w := range in.Walls {
		if w.Weight == weightPair {
			paired++
		}
	}
	if paired != 2 {
		t.Errorf("walls %+v, want two pairs", in.Walls)
	}
}

func TestNestedDoorBlock(t *testing.T) {
	// The swing sits in a block inside the door block, and the door block
	// in an unnamed group: the door is still the named block, whole.
	doc := metricDoc(hwall(0, 0, 4, 0.2, [2]float64{1, 2})...)
	doc.Model = append(doc.Model, insert("0", "GROUP", 0, 0, 0))
	doc.Blocks["GROUP"] = &cad.Block{Name: "GROUP", Entities: []cad.Entity{insert("A-DOOR", "DOOR-1", 1, 0, 0)}}
	doc.Blocks["DOOR-1"] = &cad.Block{Name: "DOOR-1", Entities: []cad.Entity{
		insert("0", "SWING", 0, 0, 0), ln("0", 0, 0, 0, 1),
	}}
	doc.Blocks["SWING"] = &cad.Block{Name: "SWING", Entities: []cad.Entity{arc("0", 0, 0, 1, 0, 90)}}
	in := readMetric(t, doc)
	if op, ok := findOpening(in, at(1, 0), at(2, 0), 0.01); !ok || op.Kind != floorgraph.Door {
		t.Errorf("openings %+v", in.Openings)
	}
	if in.Coverage != 1 {
		t.Errorf("coverage %v, want the door's parts all used", in.Coverage)
	}
}

func TestGeometryHelpers(t *testing.T) {
	if unit(point{}) != (point{}) {
		t.Error("unit of zero")
	}
	if _, ok := inverse(vector.Affine{A: 1, B: 2, C: 2, D: 4}); ok {
		t.Error("inverse of a singular transform")
	}
	m := vector.Rotate(0.3).Then(vector.Translate(5, -2)).Then(vector.Scale(2, -3))
	if inv, ok := inverse(m); !ok || inv.Apply(m.Apply(point{X: 1, Y: 7})).Sub(point{X: 1, Y: 7}).Len() > 1e-9 {
		t.Error("inverse doesn't undo")
	}
	if a := imageAngle(vector.Identity, point{X: 1, Y: -1e-9}); a != 0 {
		t.Errorf("imageAngle just short of a turn = %v, want 0", a)
	}
	if a := imageAngle(metricView, point{Y: 1}); a != 270 {
		t.Errorf("up the drawing is %v in the image, want 270", a)
	}
	if e := ccwEnd(1, 0); math.Abs(e-2*math.Pi) > 1e-12 {
		t.Errorf("ccwEnd = %v", e)
	}
	var p vector.Path
	p.MoveTo(point{})
	p.CubicTo(point{X: 1}, point{X: 1, Y: 1}, point{Y: 1})
	p.Close()
	p.MoveTo(point{X: 9})
	p.LineTo(point{X: 10})
	rs := rings(&p, vector.Identity)
	if len(rs) != 1 || !inside(rs, point{X: 0.3, Y: 0.5}) || inside(rs, point{X: 2, Y: 0.5}) {
		t.Errorf("rings %v", rs)
	}
	if len(pathPoints(&p)) != 6 {
		t.Errorf("pathPoints %v", pathPoints(&p))
	}
	spline := &cad.Spline{Degree: 1, Control: []point{{}, {X: 1}}}
	solid := &cad.Solid{Corners: [4]point{{}, {X: 1}, {Y: 1}, {X: 1, Y: 1}}}
	hatch := &cad.Hatch{Boundary: p}
	for _, e := range []cad.Entity{spline, solid, hatch} {
		if len(samples(e)) == 0 {
			t.Errorf("no samples of %T", e)
		}
	}
	if samples(&cad.Dimension{}) != nil {
		t.Error("samples of a dimension")
	}
	g := newGrid(1)
	g.add(0, point{}, point{X: 3})
	if got := g.near(point{X: 1}, 0.1); !reflect.DeepEqual(got, []int{0}) {
		t.Errorf("near = %v", got)
	}
	if got := g.near(point{}, 1e6); got != nil {
		t.Errorf("near over too many cells = %v, want nothing", got)
	}
}

func TestDegenerateEntities(t *testing.T) {
	// Zero-length lines, a closed polyline with a repeated vertex and
	// bulges between coincident points draw nothing a wall can be made of,
	// and nothing breaks on them.
	repeated := pl("A-WALL", true, 0, 0, 0, 0, 0, 0.3, 0)
	repeated.Vertices[0].Bulge = 0.5
	doc := metricDoc(ln("A-WALL", 1, 1, 1, 1), repeated, &cad.Text{Content: "", Height: 1})
	in := readMetric(t, doc)
	if len(in.Walls) != 0 || len(in.Labels) != 0 {
		t.Errorf("walls %+v labels %+v", in.Walls, in.Labels)
	}
}

func TestPocheAndPolylineGaps(t *testing.T) {
	// A solid between a wall's faces makes it surer.
	solid := &cad.Solid{Common: cad.Common{Layer: "A-WALL"}, Corners: [4]point{{X: 0, Y: -0.1}, {X: 3, Y: -0.1}, {X: 0, Y: 0.1}, {X: 3, Y: 0.1}}}
	in := readMetric(t, metricDoc(append(hwall(0, 0, 3, 0.2), solid)...))
	if len(in.Walls) != 1 || math.Abs(in.Walls[0].Weight-(weightPair+weightHatchBonus)) > 1e-9 {
		t.Errorf("walls %+v", in.Walls)
	}

	// Without units, walls drawn as closed polylines still give their
	// thickness: two 200-unit rooms' worth of outline.
	doc := cad.NewDocument()
	doc.Model = []cad.Entity{
		pl("A-WALL", true, 0, 0, 0, 5000, 0, 5000, 4000, 0, 4000),
		pl("A-WALL", true, 0, 200, 200, 4800, 200, 4800, 3800, 200, 3800),
	}
	in, err := Read(doc, Options{View: vector.Scale(0.1, -0.1)})
	if err != nil {
		t.Fatal(err)
	}
	if len(in.Walls) != 4 || !hasNote(in, "commonest gap") || math.Abs(in.Walls[0].Thickness-20) > 1e-9 {
		t.Errorf("walls %+v notes %q", in.Walls, in.Notes)
	}
}
