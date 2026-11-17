package cadfront

import (
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
	"github.com/weaveplatform/spaceweave/internal/plan"
)

// itemAt finds the item of a type centred within tol pixels of p.
func itemAt(items []plan.Item, typ string, p point, tol float64) (plan.Item, bool) {
	for _, it := range items {
		if it.Type == typ && close2(point{X: it.X, Y: it.Y}, p, tol) {
			return it, true
		}
	}
	return plan.Item{}, false
}

// angleNear reports whether two angles in degrees are within tol, round
// the circle.
func angleNear(a, b, tol float64) bool {
	d := math.Mod(math.Abs(a-b), 360)
	return math.Min(d, 360-d) <= tol
}

func TestFurnishedByShape(t *testing.T) {
	in, o := read(t, dxftest.Furnished())
	ppm := o.PxPerMetre
	tol := 0.03 * ppm
	// Desks: w along rot, both in metres. The turned bench's desks run
	// at -30° in the image (y down).
	type desk struct {
		x, y, w, d, rot float64
	}
	var desks []desk
	s, c := math.Sincos(30 * math.Pi / 180)
	for row := range 2 {
		for col := range 3 {
			x, y := float64(col)*1200-1200, float64(row)*800-400
			desks = append(desks,
				desk{2000 + x*c - y*s, 2000 + x*s + y*c, 1.2, 0.8, 330},
				desk{2000 + x, 8000 + y, 1.2, 0.8, 0})
		}
	}
	for _, x := range []float64{7300, 8700} {
		for _, y := range []float64{7600, 8400} {
			desks = append(desks, desk{x, y, 1.4, 0.8, 0})
		}
	}
	desks = append(desks, desk{8000, 2000, 1.6, 0.8, 0}, desk{14000, 2400, 1.6, 0.8, 0})
	for _, want := range desks {
		it, ok := itemAt(in.Furniture, "desk", mm(o, want.x, want.y), tol)
		if !ok {
			t.Errorf("no desk at (%.0f, %.0f)", want.x, want.y)
			continue
		}
		if math.Abs(*it.W-want.w) > 0.01 || math.Abs(*it.D-want.d) > 0.01 || !angleNear(*it.Rot, want.rot, 1) {
			t.Errorf("desk at (%.0f, %.0f) is %.2f × %.2f at %.0f°, want %.2f × %.2f at %.0f°",
				want.x, want.y, *it.W, *it.D, *it.Rot, want.w, want.d, want.rot)
		}
	}

	// Chairs face what they sit at, in image degrees: 90 is down the
	// page, towards a lower drawing y.
	chairs := []struct {
		x, y, facing float64
		why          string
	}{
		{8000, 1300, 270, "turned to its desk, though drawn facing away"},
		{14000, 1700, 270, "nested with its desk"},
		{14000, 8900, 90, "at the meeting table"},
		{14900, 8000, 180, "at the meeting table"},
		{13100, 8000, 0, "at the meeting table"},
		{14000, 7100, 270, "at the meeting table"},
	}
	for _, want := range chairs {
		it, ok := itemAt(in.Furniture, "chair", mm(o, want.x, want.y), 0.1*ppm)
		switch {
		case !ok:
			t.Errorf("no chair at (%.0f, %.0f), %s", want.x, want.y, want.why)
		case !angleNear(*it.Facing, want.facing, 1):
			t.Errorf("chair at (%.0f, %.0f) %s faces %.0f°, want %.0f°", want.x, want.y, want.why, *it.Facing, want.facing)
		}
	}

	others := []struct {
		typ          string
		x, y         float64
		w, d, r, rot float64 // what is set
	}{
		{"roundTable", 14000, 8000, 0, 0, 0.6, 0},
		{"cabinet", 20400, 2225, 0.8, 0.45, 0, 0}, // the carcass, without the doors' swing
		{"storage", 20500, 5300, 1.0, 0.6, 0, 0},
		{"stool", 20000, 8000, 0, 0, 0, 0},
		{"roundTable", 22000, 8000, 0, 0, 0.4, 0},
		{"stool", 24000, 8000, 0, 0, 0, 0}, // a pouffe
		{"toilet", 2000, 11810, 0, 0, 0, 0},
		{"basin", 4000, 12000, 0, 0, 0, 0},
	}
	for _, want := range others {
		it, ok := itemAt(in.Furniture, want.typ, mm(o, want.x, want.y), tol)
		switch {
		case !ok:
			t.Errorf("no %s at (%.0f, %.0f)", want.typ, want.x, want.y)
		case want.w > 0 && (math.Abs(*it.W-want.w) > 0.01 || math.Abs(*it.D-want.d) > 0.01):
			t.Errorf("%s at (%.0f, %.0f) is %.2f × %.2f, want %.2f × %.2f", want.typ, want.x, want.y, *it.W, *it.D, want.w, want.d)
		case want.r > 0 && math.Abs(*it.R-want.r) > 0.01:
			t.Errorf("%s at (%.0f, %.0f) has r %.2f, want %.2f", want.typ, want.x, want.y, *it.R, want.r)
		}
	}
	// The WC's bowl points the way it is used, -y in the block: down
	// the page.
	if it, ok := itemAt(in.Furniture, "toilet", mm(o, 2000, 11810), tol); ok && !angleNear(*it.Facing, 90, 1) {
		t.Errorf("WC faces %.0f°, want 90°", *it.Facing)
	}

	// Nothing else: the hand dryer, the coat stand and the panel stay out.
	if n := len(in.Furniture); n != len(desks)+len(chairs)+len(others) {
		for _, it := range in.Furniture {
			t.Logf("%s at %.0f, %.0f", it.Type, it.X, it.Y)
		}
		t.Errorf("%d items, want %d", n, len(desks)+len(chairs)+len(others))
	}
	for _, note := range []string{
		"19 furniture blocks were typed by their shape",
		"2 furniture blocks had names that don't say what they are and shapes that don't either",
		"1 furniture blocks were left out as things the plan has no type for",
	} {
		if !hasNote(in, note) {
			t.Errorf("no note %q in %q", note, in.Notes)
		}
	}
	// Desk numbers are no room's name.
	for _, l := range in.Labels {
		if len(l.Text) > 1 && l.Text[:2] == "D-" {
			t.Errorf("desk number %q read as a label", l.Text)
		}
	}
}

// TestFurnishedIsStable reads the fixture with its entities reversed:
// nested blocks expand the same, in the same order.
func TestFurnishedIsStable(t *testing.T) {
	a, _ := read(t, dxftest.Furnished())
	doc := parse(t, dxftest.Furnished())
	slices.Reverse(doc.Model)
	o, err := Frame(doc, testRaster)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Read(doc, o)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.Furniture, b.Furniture) {
		t.Errorf("furniture depends on entity order:\n%v\n%v", a.Furniture, b.Furniture)
	}
}

func TestNameNumbers(t *testing.T) {
	cases := []struct {
		name  string
		sizes []float64
		count int
	}{
		{"BB_4_1400", []float64{1.4}, 4},
		{"Bench-6-1600x800", []float64{1.6, 0.8}, 6},
		{"Desk_1200x0800", []float64{1.2}, 0}, // a leading zero is a code, not a size
		{"WS 1", nil, 0},                      // one is no count
		{"T20_99999", nil, 0},                 // too many to be anything
		{"A12x200", nil, 12},                  // 200 mm is too small to be furniture's side
	}
	for _, c := range cases {
		sizes, count := nameNumbers(c.name)
		if !reflect.DeepEqual(sizes, c.sizes) || count != c.count {
			t.Errorf("nameNumbers(%q) = %v, %d; want %v, %d", c.name, sizes, count, c.sizes, c.count)
		}
	}
}

func TestFurnitureWords(t *testing.T) {
	types := map[string]string{
		"Armario_2P": "cabinet", "PED-3": "cabinet", "Spind": "storage", "LKR_6": "storage",
		"Poltrona": "armchair", "Loveseat": "sofa", "Pouf": "stool", "Sedia": "chair",
		"Scrivania": "desk", "WKST_1600": "desk", "BENCH_4": "bench", "Tavolo 8p": "table",
		"Fridge": "counter", "Urinal": "basin", "Pianta": "plant",
		"Floor Lamp": "", "Wheelchair turning": "", "Coat_Bench": "bench",
	}
	for in, want := range types {
		if got := furnitureType(in); got != want {
			t.Errorf("furnitureType(%q) = %q, want %q", in, got, want)
		}
	}
	for name, want := range map[string]bool{
		"Coat_Stand": true, "Recycling_Bins": true, "Acoustic Panel": true, "Phone_Booth": true,
		"MONITOR_ARM": true, "Desk_Lamp": true, "A$C0ZQC": false, "Cabinet": false, "Bin_Desk": true,
	} {
		if got := notFurniture(name); got != want {
			t.Errorf("notFurniture(%q) = %v, want %v", name, got, want)
		}
	}
	for layer, want := range map[string]bool{
		"A-Pr_40_20-Sanitary": true, "A-Pr_40_20_Fittings": true, "P-SANI-FIXT": true, "A-Ss_40_15": false, "PR-40": false,
		// A WC layer isn't a furniture layer, so an unnamed block on it is
		// never guessed by shape, which is all sanitary changes; a client
		// profile that wants its blocks read as fittings says sanitary.
		"WC": false,
	} {
		if got := sanitaryLayer(layer); got != want {
			t.Errorf("sanitaryLayer(%q) = %v, want %v", layer, got, want)
		}
	}
	for layer, want := range map[string]bool{
		"A-Zz_20_10_90-Desk Numbers": true, "WS-NUM": true, "I-WORKSTATION-TAG": true, "A-ANNO-TEXT": false,
	} {
		if got := deskNumberLayer(layer); got != want {
			t.Errorf("deskNumberLayer(%q) = %v, want %v", layer, got, want)
		}
	}
}

// furnDoc is a drawing in metres with one block, drawn as given, inserted
// on a furniture layer at the origin.
func furnDoc(name, layer string, ents ...cad.Entity) *cad.Document {
	doc := metricDoc(insert(layer, name, 0, 0, 0))
	doc.Blocks[name] = &cad.Block{Name: name, Entities: ents}
	return doc
}

func TestFurnitureRules(t *testing.T) {
	box := func(w, d float64) cad.Entity { return pl("0", true, 0, 0, 0, w, 0, w, d, 0, d) }
	circle := func(r float64) cad.Entity { return &cad.Circle{Common: cad.Common{Layer: "0"}, Radius: r} }
	strokes := func(n int, w, d float64) []cad.Entity {
		out := []cad.Entity{box(w, d)}
		for i := range n {
			x := w * float64(i+1) / float64(n+1)
			out = append(out, ln("0", x, 0, x, d/10))
		}
		return out
	}
	cases := []struct {
		name, block, layer string
		ents               []cad.Entity
		want               []string // the types placed
		note               string
	}{
		{"a bench seat, too shallow for desks", "BENCH", "A-FURN", []cad.Entity{box(2, 0.45)}, nil,
			"the plan has no type for"},
		{"a bench deep enough for two rows", "BENCH_2_1200", "A-FURN", []cad.Entity{box(1.2, 1.6)},
			[]string{"desk", "desk"}, ""},
		{"a big round table", "A$C1", "A-FURN", []cad.Entity{circle(0.75)}, []string{"roundTable"}, ""},
		{"a round too big for any table", "A$C1", "A-FURN", []cad.Entity{circle(1.5)}, nil, "shapes that don't either"},
		{"a dot too small for a stool", "A$C1", "A-FURN", []cad.Entity{circle(0.1)}, nil, "shapes that don't either"},
		{"a table named for its size", "A$C1_850x850", "A-FURN", strokes(50, 0.85, 0.85), []string{"desk"}, ""},
		{"a detailed armchair", "A$C1", "A-FURN", strokes(60, 0.9, 0.85), []string{"armchair"}, ""},
		{"a sofa's worth of strokes", "A$C1", "A-FURN", strokes(400, 2.0, 0.9), nil, "shapes that don't either"},
		{"a door swing with no carcass", "A$C1", "A-FURN", []cad.Entity{
			arc("0", 0, 0, 0.4, 0, 90), ln("0", 0, 0, 0, 0.4), ln("0", 0, 0, 0.4, 0),
		}, nil, "shapes that don't either"},
		{"a pouffe drawn square", "A$C1", "A-FURN", []cad.Entity{box(0.45, 0.45)}, []string{"stool"}, ""},
		{"a basin on a sanitary layer", "A$C1", "P-SANI", []cad.Entity{
			&cad.Ellipse{Common: cad.Common{Layer: "0"}, Major: point{X: 0.25}, Ratio: 0.8, End: 2 * math.Pi},
		}, []string{"basin"}, ""},
		{"a box on a sanitary layer", "A$C1", "P-SANI", []cad.Entity{box(0.3, 0.3)}, nil, "shapes that don't either"},
		{"a named table, round", "TABLE", "A-FURN", []cad.Entity{arc("0", 0, 0, 0.5, 0, 180), arc("0", 0, 0, 0.5, 180, 360)},
			[]string{"roundTable"}, ""},
		{"a coat stand", "COAT_STAND", "A-FURN", []cad.Entity{circle(0.2)}, nil, "the plan has no type for"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := readMetric(t, furnDoc(c.block, c.layer, c.ents...))
			var got []string
			for _, it := range in.Furniture {
				got = append(got, it.Type)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("placed %v, want %v", got, c.want)
			}
			if c.note != "" && !hasNote(in, c.note) {
				t.Errorf("no note %q in %q", c.note, in.Notes)
			}
		})
	}
}

// TestFurnitureUnitless: in a drawing whose units aren't known, shape says
// nothing (a chair's size is meaningless), so unnamed blocks stay out.
func TestFurnitureUnitless(t *testing.T) {
	doc := furnDoc("A$C1", "A-FURN", pl("0", true, 0, 0, 0, 1600, 0, 1600, 800, 0, 800))
	doc.Units = cad.Unitless
	in, err := Read(doc, Options{View: metricView, PxPerMetre: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(in.Furniture) != 0 || !hasNote(in, "shapes that don't either") {
		t.Errorf("placed %v; notes %q", in.Furniture, in.Notes)
	}
}

func TestFurnitureDrawings(t *testing.T) {
	box := func(w, d float64) cad.Entity { return pl("0", true, 0, 0, 0, w, 0, w, d, 0, d) }
	bulged := pl("0", true, 0, -0.5, 0, 0.5, 0)
	bulged.Vertices[0].Bulge, bulged.Vertices[1].Bulge = 1, 1
	text := func(layer, s string, x, y float64) cad.Entity {
		return &cad.Text{Common: cad.Common{Layer: layer}, Content: s, Height: 0.1, At: point{X: x, Y: y}}
	}
	cases := []struct {
		name  string
		layer string
		ents  []cad.Entity
		model []cad.Entity // beside the insert, in the drawing
		want  []string
	}{
		{"a bench drawn along y, numbered", "A-FURN",
			[]cad.Entity{box(1.6, 2.4), ln("0", 0.8, 0, 0.8, 2.4), ln("0", 0, 1.2, 1.6, 1.2)},
			[]cad.Entity{
				text("A-DESK-NUM", "1", 0.3, 0.5), text("A-DESK-NUM", "2", 1.1, 0.5),
				text("A-DESK-NUM", "3", 0.3, 1.7), text("A-DESK-NUM", "4", 1.1, 1.7),
				text("A-DESK-NUM", "Hot desks", 5, 5), text("A-DESK-NUM", "Numbered by facilities, 2024", 5, 6),
				&cad.MText{Common: cad.Common{Layer: "A-DESK-NUM"}, Content: " ", Height: 0.1},
			},
			[]string{"desk", "desk", "desk", "desk"}},
		{"a round table drawn as a bulged outline", "A-FURN", []cad.Entity{bulged}, nil, []string{"roundTable"}},
		{"a desk drawn as an outline back to its start, with an open bracket", "A-FURN", []cad.Entity{
			pl("0", false, 0, 0, 0, 1.6, 0, 1.6, 0.8, 0, 0.8, 0, 0),
			pl("0", false, 0, 0.1, 0.1, 0.5, 0.1, 0.5, 0.3, 0.1, 0.3),
			pl("0", true, 0, 0.6, 0.1, 1.0, 0.1, 0.9, 0.3, 0.7, 0.3),
		}, nil, []string{"desk"}},
		{"a pouffe drawn solid", "A-FURN", []cad.Entity{
			&cad.Solid{Common: cad.Common{Layer: "0"}, Corners: [4]point{{}, {X: 0.45}, {X: 0.45, Y: 0.45}, {Y: 0.45}}},
			&cad.Hatch{Common: cad.Common{Layer: "0"}, Solid: true},
		}, nil, []string{"stool"}},
		{"a cupboard whose door swings out sideways", "A-FURN", []cad.Entity{
			box(0.45, 0.8), arc("0", 0.45, 0, 0.4, 0, 90), ln("0", 0.45, 0, 0.85, 0),
		}, nil, []string{"cabinet"}},
		{"a ring on a sanitary layer", "P-SANI", []cad.Entity{
			&cad.Circle{Common: cad.Common{Layer: "0"}, Radius: 0.75},
		}, nil, nil},
		{"a lone line", "A-FURN", []cad.Entity{ln("0", 0, 0, 1, 0)}, nil, nil},
		{"a chair in a frame that is nothing", "A-FURN", []cad.Entity{
			insert("0", "CHAIR", 0.3, 0.3, 0), ln("0", 0, 0, 0.1, 0),
		}, nil, []string{"chair"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := furnDoc("A$C1", c.layer, c.ents...)
			doc.Model = append(doc.Model, c.model...)
			doc.Blocks["CHAIR"] = &cad.Block{Name: "CHAIR", Entities: []cad.Entity{box(0.5, 0.5)}}
			in := readMetric(t, doc)
			var got []string
			for _, it := range in.Furniture {
				got = append(got, it.Type)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("placed %v, want %v", got, c.want)
			}
		})
	}
}
