package cadfront

import (
	"math"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
	"github.com/weaveplatform/spaceweave/internal/floorgraph"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// The documents below are in metres, drawn at 100 px a metre with y up
// the image from 1000: (x, y) lands on (100x, 1000 - 100y).
var metricView = vector.Affine{A: 100, D: -100, F: 1000}

func at(x, y float64) point { return metricView.Apply(point{X: x, Y: y}) }

func metricDoc(ents ...cad.Entity) *cad.Document {
	doc := cad.NewDocument()
	doc.Units = cad.Meters
	for _, name := range []string{"A-WALL", "A-GLAZ", "A-DOOR", "A-FURN", "A-GLAZ-WIND"} {
		doc.Layers[name] = &cad.Layer{Name: name, Color: 7}
	}
	doc.Model = ents
	return doc
}

func readMetric(t *testing.T, doc *cad.Document) floorgraph.Input {
	t.Helper()
	in, err := Read(doc, Options{View: metricView, PxPerMetre: 100, Exact: true})
	if err != nil {
		t.Fatal(err)
	}
	return in
}

func ln(layer string, ax, ay, bx, by float64) *cad.Line {
	return &cad.Line{Common: cad.Common{Layer: layer}, A: point{X: ax, Y: ay}, B: point{X: bx, Y: by}}
}

func arc(layer string, cx, cy, r, a0, a1 float64) *cad.Arc {
	return &cad.Arc{
		Common: cad.Common{Layer: layer}, Center: point{X: cx, Y: cy}, Radius: r,
		Start: a0 * math.Pi / 180, End: a1 * math.Pi / 180,
	}
}

func pl(layer string, closed bool, width float64, xy ...float64) *cad.Polyline {
	p := &cad.Polyline{Common: cad.Common{Layer: layer}, Closed: closed, Width: width}
	for i := 0; i+1 < len(xy); i += 2 {
		p.Vertices = append(p.Vertices, cad.Vertex{P: point{X: xy[i], Y: xy[i+1]}})
	}
	return p
}

// hwall is a horizontal wall's two faces at y, thick apart, from x0 to x1
// with gaps.
func hwall(y, x0, x1, thick float64, gaps ...[2]float64) []cad.Entity {
	var out []cad.Entity
	for _, side := range []float64{-thick / 2, thick / 2} {
		start := x0
		for _, g := range append(gaps, [2]float64{x1, x1}) {
			out = append(out, ln("A-WALL", start, y+side, g[0], y+side))
			start = g[1]
		}
	}
	return out
}

func vwall(x, y0, y1, thick float64, gaps ...[2]float64) []cad.Entity {
	out := hwall(0, y0, y1, thick, gaps...)
	for _, e := range out {
		l, _ := e.(*cad.Line)
		l.A, l.B = point{X: x - l.A.Y, Y: l.A.X}, point{X: x - l.B.Y, Y: l.B.X}
	}
	return out
}

func insert(layer, block string, x, y, rot float64) *cad.Insert {
	return &cad.Insert{
		Common: cad.Common{Layer: layer}, Block: block, At: point{X: x, Y: y},
		Scale: point{X: 1, Y: 1}, Rotation: rot * math.Pi / 180,
	}
}

func TestPolylineWalls(t *testing.T) {
	bulged := pl("A-WALL", true, 0, 70, 0, 70.4, 0, 70.4, 0.3, 70, 0.3)
	bulged.Vertices[0].Bulge = 0.1
	in := readMetric(t, metricDoc(
		pl("A-WALL", false, 0.2, 0, 0, 4, 0, 4, 3),
		pl("A-WALL", true, 0, 10, 0, 10.4, 0, 10.4, 0.3, 10, 0.3), // a column
		pl("A-GLAZ", false, 0.05, 20, 0, 22, 0),
		ln("A-GLAZ", 30, 0, 31, 0), // a lone pane
		ln("A-WALL", 40, 0, 40, 0.5),
		ln("A-WALL", 50, 0, 52, 0), // a single-line wall
		pl("A-WALL", true, 0, 60, 0, 62, 0, 62, 2, 60, 2), // a room, not a column
		bulged,
	))
	cases := []struct {
		a, b          point
		thick, weight float64
		glass         bool
	}{
		{at(0, 0), at(4, 0), 20, weightWidth, false},
		{at(4, 0), at(4, 3), 20, weightWidth, false},
		{at(10, 0.15), at(10.4, 0.15), 30, weightColumn, false},
		{at(20, 0), at(22, 0), 5, weightWidth, true},
		{at(30, 0), at(31, 0), 100 * thinGlass, weightThinGlass, true},
		{at(50, 0), at(52, 0), 100 * thinWall, weightThin, false},
		{at(60, 0), at(62, 0), 100 * thinWall, weightThin, false},
		{at(60, 2), at(62, 2), 100 * thinWall, weightThin, false},
		{at(60, 0), at(60, 2), 100 * thinWall, weightThin, false},
		{at(62, 0), at(62, 2), 100 * thinWall, weightThin, false},
		// The bulged rectangle's two straight sides 0.4 m apart are a wall.
		{at(70.2, 0), at(70.2, 0.3), 40, weightPair, false},
	}
	for _, c := range cases {
		w, ok := findWall(in, c.a, c.b, 0.01)
		if !ok || math.Abs(w.Thickness-c.thick) > 1e-6 || math.Abs(w.Weight-c.weight) > 1e-9 || w.Glass != c.glass {
			t.Errorf("wall %v–%v: %+v (found %v)", c.a, c.b, w, ok)
		}
	}
	if len(in.Walls) != len(cases) {
		t.Errorf("%d walls, want %d: %+v", len(in.Walls), len(cases), in.Walls)
	}
}

func doorBlock() *cad.Block {
	return &cad.Block{Name: "DOOR", Entities: []cad.Entity{ln("0", 0, 0, 0, 0.9), arc("0", 0, 0, 0.9, 0, 90)}}
}

func TestLooseDoors(t *testing.T) {
	ents := hwall(0, 0, 6, 0.2, [2]float64{2, 2.9}, [2]float64{4, 4.9})
	ents = append(ents, vwall(10, 0, 4, 0.1, [2]float64{1, 2})...)
	ents = append(ents, hwall(20, -1, 3, 0.2, [2]float64{0, 1.8})...)
	swing := pl("A-DOOR", false, 0, 10, 2, 11, 2, 10, 1)
	swing.Vertices[1].Bulge = -math.Tan(math.Pi / 8)
	mirrored := insert("A-DOOR", "DOOR", 1.8, 20, 0)
	mirrored.Scale.X = -1
	ents = append(ents,
		arc("A-DOOR", 2, 0, 0.9, 0, 90), // on a door layer: no leaf needed
		arc("A-WALL", 4.9, 0, 0.9, 90, 180), ln("A-WALL", 4.9, 0, 4.9, 0.9), // exploded on the wall layer
		arc("A-WALL", 40, 0, 0.9, 0, 90), // no leaf, not on a door layer: a curve
		arc("A-FURN", 50, 0, 0.9, 0, 90),
		swing,
		insert("A-DOOR", "DOOR", 0, 20, 0), mirrored, // two leaves meeting
	)
	doc := metricDoc(ents...)
	doc.Blocks["DOOR"] = doorBlock()
	in := readMetric(t, doc)
	cases := []struct {
		kind   string
		a, b   point
		weight float64
	}{
		{floorgraph.Door, at(2, 0), at(2.9, 0), weightLayerArc},
		{floorgraph.Door, at(4.9, 0), at(4, 0), weightLooseArc},
		{floorgraph.Door, at(10, 2), at(10, 1), weightLayerArc},
		{floorgraph.DoubleDoor, at(0, 20), at(1.8, 20), weightNamedDoor},
	}
	for _, c := range cases {
		op, ok := findOpening(in, c.a, c.b, 0.01)
		if !ok || op.Kind != c.kind || op.Weight != c.weight || !op.Leafed {
			t.Errorf("%s %v–%v: %+v (found %v)", c.kind, c.a, c.b, op, ok)
		}
	}
	if len(in.Openings) != len(cases) {
		t.Errorf("openings %+v", in.Openings)
	}
	// The exploded door's leaf is not a wall.
	for _, w := range in.Walls {
		if w.Weight == weightThin {
			t.Errorf("thin wall %+v", w)
		}
	}
}

func TestSpans(t *testing.T) {
	ents := hwall(0, 0, 8, 0.2, [2]float64{3, 4})
	ents = append(ents, vwall(30, 0, 5, 0.2, [2]float64{2, 3.2})...)
	ents = append(ents,
		insert("A-DOOR", "DOOR_SLIDING", 3.5, 0, 0),
		insert("0", "WINDOW", 20, 20, 0),
		insert("A-GLAZ-WIND", "NARROW", 6, 0, 0),
		ln("A-GLAZ-WIND", 29.9, 2, 29.9, 3.2), ln("A-GLAZ-WIND", 30.1, 2, 30.1, 3.2),
		ln("A-GLAZ-WIND", 29.9, 2, 30.1, 2), ln("A-GLAZ-WIND", 29.9, 3.2, 30.1, 3.2),
		ln("A-GLAZ-WIND", 40, 0, 41, 0),
	)
	doc := metricDoc(ents...)
	doc.Blocks["DOOR_SLIDING"] = &cad.Block{Name: "DOOR_SLIDING", Entities: []cad.Entity{
		pl("0", true, 0, -0.5, -0.05, 0.5, -0.05, 0.5, 0.05, -0.5, 0.05),
	}}
	doc.Blocks["WINDOW"] = &cad.Block{Name: "WINDOW", Entities: []cad.Entity{
		pl("0", true, 0, 0, 0, 1.2, 0, 1.2, 0.2, 0, 0.2),
	}}
	doc.Blocks["NARROW"] = &cad.Block{Name: "NARROW", Entities: []cad.Entity{
		pl("0", true, 0, -0.1, -0.3, 0.1, -0.3, 0.1, 0.3, -0.1, 0.3),
	}}
	in := readMetric(t, doc)
	cases := []struct {
		kind   string
		a, b   point
		weight float64
	}{
		{floorgraph.Door, at(3, 0), at(4, 0), weightSpan},
		{floorgraph.Window, at(20, 20.1), at(21.2, 20.1), weightLoneSpan},
		{floorgraph.Window, at(5.9, 0), at(6.1, 0), weightSpan},
		{floorgraph.Window, at(30, 2), at(30, 3.2), weightSpan},
	}
	for _, c := range cases {
		op, ok := findOpening(in, c.a, c.b, 0.01)
		if !ok || op.Kind != c.kind || op.Weight != c.weight || op.Leafed {
			t.Errorf("%s %v–%v: %+v (found %v)", c.kind, c.a, c.b, op, ok)
		}
	}
	if len(in.Openings) != len(cases) {
		t.Errorf("openings %+v", in.Openings)
	}
	// Glazing with no wall either side is the wall there, not a window in one.
	if w, ok := findWall(in, at(40, 0), at(41, 0), 0.01); !ok || !w.Glass {
		t.Errorf("unhosted glazing: %+v (found %v)", w, ok)
	}
}

func TestScaleFallbacks(t *testing.T) {
	// Nothing to judge a size by: a drawing is assumed to be an office
	// floor across.
	doc := metricDoc(&cad.Text{Common: cad.Common{Layer: "0"}, Content: "Lobby", Height: 0.3})
	in, err := Read(doc, Options{View: metricView})
	if err != nil {
		t.Fatal(err)
	}
	if !hasNote(in, "assume it is 30 m across") || len(in.Labels) != 1 || in.Coverage != 1 {
		t.Errorf("notes %q labels %+v coverage %v", in.Notes, in.Labels, in.Coverage)
	}

	empty := cad.NewDocument()
	in, err = Read(empty, Options{View: metricView})
	if err != nil || in.Coverage != -1 || !hasNote(in, "nothing in it gives a size") {
		t.Errorf("empty: %v, coverage %v, notes %q", err, in.Coverage, in.Notes)
	}

	paper := cad.NewDocument()
	paper.Paper = []cad.Entity{ln("0", 0, 0, 1, 0)}
	in, err = Read(paper, Options{View: metricView, PxPerMetre: 100})
	if err != nil || !hasNote(in, "paper layout") {
		t.Errorf("paper: %v, notes %q", err, in.Notes)
	}
}

func TestFurnitureEdges(t *testing.T) {
	tiny := insert("A-FURN", "CHAIR", 0, 0, 0)
	tiny.Scale = point{X: 1e-200, Y: 1e-200} // squashed to nothing: no size to give
	doc := metricDoc(
		insert("A-FURN", "HUGE_DESK", 0, 0, 0),
		insert("A-FURN", "TABLE", 10, 0, 0),
		insert("A-FURN", "STOOL", 20, 0, 0),
		insert("A-FURN", "WS", 30, 0, 0),
		insert("A-FURN", "EMPTY_DESK", 40, 0, 0),
		insert("A-FURN", "PLANT", 50, 0, 0),
		tiny,
		// Text in a furniture block is its own business. (Loose text on a
		// furniture layer would be too: see TestRoomNameText.)
		&cad.Text{Common: cad.Common{Layer: "0"}, Content: "Store", Height: 0.2, At: point{X: 60}},
	)
	rect := func(w, d float64) []cad.Entity { return []cad.Entity{pl("0", true, 0, 0, 0, w, 0, w, d, 0, d)} }
	doc.Blocks["HUGE_DESK"] = &cad.Block{Name: "HUGE_DESK", Entities: rect(100, 1)}
	doc.Blocks["TABLE"] = &cad.Block{Name: "TABLE", Entities: append(rect(2, 1),
		&cad.Text{Common: cad.Common{Layer: "0"}, Content: "T1", Height: 0.1, At: point{X: 0.2, Y: 0.2}})}
	doc.Blocks["STOOL"] = &cad.Block{Name: "STOOL", Entities: []cad.Entity{
		&cad.Circle{Common: cad.Common{Layer: "0"}, Radius: 0.2},
	}}
	doc.Blocks["WS"] = &cad.Block{Name: "WS", Entities: rect(1.4, 0.7)}
	doc.Blocks["EMPTY_DESK"] = &cad.Block{Name: "EMPTY_DESK"}
	doc.Blocks["PLANT"] = &cad.Block{Name: "PLANT", Entities: []cad.Entity{
		&cad.Ellipse{Common: cad.Common{Layer: "0"}, Major: point{X: 0.4}, Ratio: 0.5, End: 2 * math.Pi},
	}}
	doc.Blocks["CHAIR"] = &cad.Block{Name: "CHAIR", Entities: rect(0.5, 0.5)}
	in := readMetric(t, doc)
	types := map[string]int{}
	for _, it := range in.Furniture {
		types[it.Type]++
	}
	if types["desk"] != 2 || types["stool"] != 1 || types["plant"] != 1 || len(in.Furniture) != 4 {
		t.Errorf("furniture %v", types)
	}
	if !hasNote(in, "3 furniture blocks were left out for implausible sizes") {
		t.Errorf("notes %q", in.Notes)
	}
	if len(in.Labels) != 1 || in.Labels[0].Text != "Store" {
		t.Errorf("labels %+v, want only the loose text", in.Labels)
	}
}

func TestPlanFixture(t *testing.T) {
	// The converter's own fixture: walls drawn as polylines with a width,
	// unnamed DOOR blocks with swings on a DOORS layer, every entity kind.
	in, o := read(t, dxftest.Plan())
	width := 0
	for _, w := range in.Walls {
		if w.Weight == weightWidth && math.Abs(w.Thickness-0.2*o.PxPerMetre) < 1e-6 {
			width++
		}
	}
	if width != 4 {
		t.Errorf("%d walls of the outline's width, want 4: %+v", width, in.Walls)
	}
	doors := 0
	for _, op := range in.Openings {
		if op.Kind == floorgraph.Door {
			doors++
		}
	}
	if doors != 3 {
		t.Errorf("openings %+v, want the three door inserts", in.Openings)
	}
	texts := map[string]bool{}
	for _, l := range in.Labels {
		texts[l.Text] = true
	}
	if !texts["Open office"] || !texts["Meeting°"] || texts["12 000"] || texts["Title block"] {
		t.Errorf("labels %+v", in.Labels)
	}
}
