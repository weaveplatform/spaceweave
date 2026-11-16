package cad

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/vector"
)

var (
	red   = vector.Color{R: 255}
	green = vector.Color{G: 255}
	blue  = vector.Color{B: 255}
)

// metricDoc is a document in metres, so Draw adds no unit notes and tests
// can look at geometry alone.
func metricDoc(model ...Entity) *Document {
	doc := NewDocument()
	doc.Units = Meters
	doc.Model = model
	return doc
}

func line(ax, ay, bx, by float64) *Line {
	return &Line{
		Common: Common{Layer: "0", Color: ByLayer},
		A:      Point{X: ax, Y: ay},
		B:      Point{X: bx, Y: by},
	}
}

func mustDraw(t *testing.T, doc *Document) *vector.Drawing {
	t.Helper()
	d, err := Draw(doc)
	if err != nil {
		t.Fatalf("Draw: %v", err)
	}
	return d
}

func checkBounds(t *testing.T, name string, r vector.Rect, lo, hi Point) {
	t.Helper()
	if r.Empty() || !nearPt(r.Min, lo) || !nearPt(r.Max, hi) {
		t.Errorf("%s: bounds %v..%v (empty %v), want %v..%v", name, r.Min, r.Max, r.Empty(), lo, hi)
	}
}

func paths(t *testing.T, d *vector.Drawing) []*vector.Path {
	t.Helper()
	out := make([]*vector.Path, 0, len(d.Items))
	for _, it := range d.Items {
		p, ok := it.(*vector.Path)
		if !ok {
			t.Fatalf("item %T, want *vector.Path", it)
		}
		out = append(out, p)
	}
	return out
}

func texts(t *testing.T, d *vector.Drawing) []*vector.Text {
	t.Helper()
	out := make([]*vector.Text, 0, len(d.Items))
	for _, it := range d.Items {
		x, ok := it.(*vector.Text)
		if !ok {
			t.Fatalf("item %T, want *vector.Text", it)
		}
		out = append(out, x)
	}
	return out
}

func hasNote(d *vector.Drawing, sub string) bool {
	for _, n := range d.Notes {
		if strings.Contains(n, sub) {
			return true
		}
	}
	return false
}

func TestDrawInsertTransform(t *testing.T) {
	tests := []struct {
		name   string
		base   Point
		in     Insert
		lo, hi Point
	}{
		{
			name: "rotated 90 and scaled 2",
			in:   Insert{At: Point{X: 10, Y: 5}, Scale: Point{X: 2, Y: 2}, Rotation: math.Pi / 2},
			lo:   Point{X: 10, Y: 5}, hi: Point{X: 10, Y: 7},
		},
		{
			// The base point lands on the insertion point, so a line from
			// the base runs back from it.
			name: "base point",
			base: Point{X: 1, Y: 0},
			in:   Insert{At: Point{X: 10, Y: 5}, Scale: Point{X: 2, Y: 2}, Rotation: math.Pi / 2},
			lo:   Point{X: 10, Y: 3}, hi: Point{X: 10, Y: 5},
		},
		{
			// A zero scale is a file that left the group out, not a
			// collapse to a point.
			name: "zero scale is one",
			in:   Insert{At: Point{X: 1, Y: 1}},
			lo:   Point{X: 1, Y: 1}, hi: Point{X: 2, Y: 1},
		},
		{
			name: "non-uniform scale",
			in:   Insert{Scale: Point{X: 3, Y: 1}},
			lo:   Point{X: 0, Y: 0}, hi: Point{X: 3, Y: 0},
		},
		{
			// The insertion point is in object coordinates, so a mirrored
			// insert at x=3 lands at x=-3 and runs leftwards from it.
			name: "mirrored",
			in:   Insert{Common: Common{Mirrored: true}, At: Point{X: 3, Y: 0}},
			lo:   Point{X: -4, Y: 0}, hi: Point{X: -3, Y: 0},
		},
	}
	for _, tt := range tests {
		doc := metricDoc()
		doc.Blocks["B"] = &Block{Name: "B", Base: tt.base, Entities: []Entity{line(0, 0, 1, 0)}}
		in := tt.in
		in.Block = "B"
		in.Layer = "0"
		in.Color = ByLayer
		doc.Model = []Entity{&in}
		d := mustDraw(t, doc)
		if len(d.Items) != 1 {
			t.Fatalf("%s: %d items, want 1", tt.name, len(d.Items))
		}
		checkBounds(t, tt.name, d.Bounds(), tt.lo, tt.hi)
	}
}

func TestDrawNestedInsert(t *testing.T) {
	// The inner insert's transform applies first, then the outer one's.
	doc := metricDoc(&Insert{Block: "outer", At: Point{X: 100, Y: 0}, Scale: Point{X: 10, Y: 10}})
	doc.Blocks["outer"] = &Block{Name: "outer", Entities: []Entity{
		&Insert{Block: "inner", At: Point{X: 1, Y: 0}, Rotation: math.Pi / 2},
	}}
	doc.Blocks["inner"] = &Block{Name: "inner", Entities: []Entity{line(0, 0, 1, 0)}}
	d := mustDraw(t, doc)
	checkBounds(t, "nested", d.Bounds(), Point{X: 110, Y: 0}, Point{X: 110, Y: 10})
}

func TestDrawMInsertArray(t *testing.T) {
	doc := metricDoc(
		&Insert{
			Block:     "B",
			Columns:   3,
			Rows:      2,
			ColumnGap: 10,
			RowGap:    20,
			Scale:     Point{X: 2, Y: 2},
		},
	)
	doc.Blocks["B"] = &Block{Name: "B", Entities: []Entity{line(0, 0, 1, 0)}}
	d := mustDraw(t, doc)
	ps := paths(t, d)
	if len(ps) != 6 {
		t.Fatalf("%d items, want 3x2", len(ps))
	}
	// Gaps are not scaled with the block: copies start 10 apart and are
	// 2 long.
	seen := map[Point]bool{}
	for _, p := range ps {
		seen[p.Segs[0].P[0]] = true
		if l := p.Segs[1].P[0].Sub(p.Segs[0].P[0]).Len(); !near(l, 2) {
			t.Errorf("copy is %v long, want 2", l)
		}
	}
	for _, x := range []float64{0, 10, 20} {
		for _, y := range []float64{0, 20} {
			if !seen[Point{X: x, Y: y}] {
				t.Errorf("no copy at (%v, %v); got %v", x, y, seen)
			}
		}
	}

	// The array follows the insert's rotation: columns run up the page
	// when it is turned a quarter.
	doc = metricDoc(&Insert{Block: "B", Columns: 2, ColumnGap: 10, Rotation: math.Pi / 2})
	doc.Blocks["B"] = &Block{Name: "B", Entities: []Entity{line(0, 0, 1, 0)}}
	d = mustDraw(t, doc)
	checkBounds(t, "rotated array", d.Bounds(), Point{X: 0, Y: 0}, Point{X: 0, Y: 11})
}

func TestDrawSelfInsertIsCycle(t *testing.T) {
	doc := metricDoc(&Insert{Block: "A"})
	doc.Blocks["A"] = &Block{Name: "A", Entities: []Entity{
		line(0, 0, 1, 0),
		&Insert{Block: "A", At: Point{X: 5, Y: 0}},
	}}
	d := mustDraw(t, doc)
	if len(d.Items) != 1 {
		t.Errorf("%d items, want the one line drawn once", len(d.Items))
	}
	if !hasNote(d, "1 block references that contain themselves") {
		t.Errorf("notes %q, want a cycle note", d.Notes)
	}

	// A cycle through two blocks, named in different case, is the same loop.
	doc = metricDoc(&Insert{Block: "P"})
	doc.Blocks["P"] = &Block{Name: "P", Entities: []Entity{line(0, 0, 1, 0), &Insert{Block: "q"}}}
	doc.Blocks["q"] = &Block{Name: "q", Entities: []Entity{line(0, 1, 1, 1), &Insert{Block: "p"}}}
	doc.Blocks["p"] = doc.Blocks["P"]
	d = mustDraw(t, doc)
	if len(d.Items) != 2 || !hasNote(d, "contain themselves") {
		t.Errorf("%d items, notes %q; want 2 items and a cycle note", len(d.Items), d.Notes)
	}
}

func TestDrawMaxDepth(t *testing.T) {
	const chain = MaxDepth + 4
	doc := metricDoc(&Insert{Block: "b0"})
	for i := range chain {
		name := "b" + string(rune('a'+i))
		if i == 0 {
			name = "b0"
		}
		next := "b" + string(rune('a'+i+1))
		doc.Blocks[name] = &Block{Name: name, Entities: []Entity{
			line(float64(i), 0, float64(i), 1),
			&Insert{Block: next},
		}}
	}
	d := mustDraw(t, doc)
	if len(d.Items) != MaxDepth {
		t.Errorf("%d lines drawn, want one per level up to %d", len(d.Items), MaxDepth)
	}
	if !hasNote(d, "1 block references nested over 16 deep") {
		t.Errorf("notes %q, want a depth note", d.Notes)
	}
}

func TestDrawTooBig(t *testing.T) {
	// The array is refused from its size alone, before a single copy is
	// drawn, so this costs nothing to run.
	huge := &Insert{Block: "B", Columns: 2001, Rows: 1000}
	doc := metricDoc(line(0, 0, 1, 0), huge)
	doc.Blocks["B"] = &Block{Name: "B", Entities: []Entity{line(0, 0, 1, 0)}}
	if d, err := Draw(doc); !errors.Is(err, ErrTooBig) || d != nil {
		t.Errorf("Draw = %v, %v; want nil, ErrTooBig", d, err)
	}

	// The error must come out through every level of nesting, not just
	// stop the block it happened in.
	doc = metricDoc(&Insert{Block: "outer"})
	doc.Blocks["outer"] = &Block{Name: "outer", Entities: []Entity{huge, line(0, 0, 1, 0)}}
	doc.Blocks["B"] = &Block{Name: "B", Entities: []Entity{line(0, 0, 1, 0)}}
	if _, err := Draw(doc); !errors.Is(err, ErrTooBig) {
		t.Errorf("nested: err %v, want ErrTooBig", err)
	}
}

func TestDrawLayerVisibility(t *testing.T) {
	doc := metricDoc(
		&Line{Common: Common{Layer: "off"}, B: Point{X: 1}},
		&Line{Common: Common{Layer: "frozen"}, B: Point{X: 1}},
		&Insert{Common: Common{Layer: "frozen"}, Block: "B"},
		&Line{Common: Common{Layer: "walls"}, A: Point{X: 5}, B: Point{X: 6}},
		&Insert{Common: Common{Layer: "walls"}, Block: "B", At: Point{Y: 10}},
		&Line{Common: Common{Layer: "walls", Invisible: true}, B: Point{X: 100}},
	)
	doc.Layers["off"] = &Layer{Name: "off", Off: true}
	doc.Layers["frozen"] = &Layer{Name: "frozen", Frozen: true}
	doc.Layers["walls"] = &Layer{Name: "walls", Color: 7}
	// Inside the block, an entity on a frozen layer stays hidden even
	// though its insert is visible.
	doc.Blocks["B"] = &Block{Name: "B", Entities: []Entity{
		&Line{Common: Common{Layer: "0"}, B: Point{X: 1}},
		&Line{Common: Common{Layer: "frozen"}, B: Point{X: 50}},
	}}
	d := mustDraw(t, doc)
	if len(d.Items) != 2 {
		t.Fatalf("%d items, want the walls line and one block line", len(d.Items))
	}
	checkBounds(t, "visible", d.Bounds(), Point{X: 0, Y: 0}, Point{X: 6, Y: 10})
}

func TestDrawColours(t *testing.T) {
	cyanRGB := vector.Color{R: 1, G: 2, B: 3}
	layerRGB := vector.Color{R: 9, G: 8, B: 7}
	doc := metricDoc()
	doc.Layers["L5"] = &Layer{Name: "L5", Color: 5}
	doc.Layers["true"] = &Layer{Name: "true", Color: 1, RGB: &layerRGB}
	doc.Blocks["B"] = &Block{Name: "B", Entities: []Entity{
		// BYBLOCK takes the insert's resolved colour.
		&Line{Common: Common{Layer: "L5", Color: ByBlock}},
		// Layer 0 inside a block takes the insert's layer.
		&Line{Common: Common{Layer: "0", Color: ByLayer}},
	}}
	tests := []struct {
		name string
		e    Entity
		want []vector.Color
	}{
		{name: "aci", e: &Line{Common: Common{Layer: "L5", Color: 1}}, want: []vector.Color{red}},
		{
			name: "bylayer",
			e:    &Line{Common: Common{Layer: "L5", Color: ByLayer}},
			want: []vector.Color{blue},
		},
		{
			name: "layer truecolour",
			e:    &Line{Common: Common{Layer: "true", Color: ByLayer}},
			want: []vector.Color{layerRGB},
		},
		{
			name: "entity truecolour",
			e:    &Line{Common: Common{Layer: "L5", Color: 1, RGB: &cyanRGB}},
			want: []vector.Color{cyanRGB},
		},
		{
			name: "byblock at top level",
			e:    &Line{Common: Common{Layer: "L5", Color: ByBlock}},
			want: []vector.Color{vector.Black},
		},
		// Colour numbers outside the ACI are damage; the layer decides.
		{
			name: "out of range",
			e:    &Line{Common: Common{Layer: "L5", Color: 300}},
			want: []vector.Color{blue},
		},
		{
			name: "negative",
			e:    &Line{Common: Common{Layer: "L5", Color: -2}},
			want: []vector.Color{blue},
		},
		// A layer the table doesn't list falls back to layer 0 (colour 7).
		{
			name: "unknown layer",
			e:    &Line{Common: Common{Layer: "ghost", Color: ByLayer}},
			want: []vector.Color{vector.Black},
		},
		{
			name: "insert colour and layer",
			e:    &Insert{Common: Common{Layer: "L5", Color: 3}, Block: "B"},
			want: []vector.Color{green, blue},
		},
		{
			name: "insert bylayer",
			e:    &Insert{Common: Common{Layer: "true", Color: ByLayer}, Block: "B"},
			want: []vector.Color{layerRGB, layerRGB},
		},
	}
	for _, tt := range tests {
		doc.Model = []Entity{tt.e}
		ps := paths(t, mustDraw(t, doc))
		if len(ps) != len(tt.want) {
			t.Fatalf("%s: %d items, want %d", tt.name, len(ps), len(tt.want))
		}
		for i, p := range ps {
			if p.Color != tt.want[i] {
				t.Errorf("%s: item %d colour %v, want %v", tt.name, i, p.Color, tt.want[i])
			}
		}
	}

	// A document with no layer table at all still draws, in black.
	bare := &Document{
		Units: Meters,
		Model: []Entity{&Line{Common: Common{Layer: "x", Color: ByLayer}, B: Point{X: 1}}},
	}
	ps := paths(t, mustDraw(t, bare))
	if len(ps) != 1 || ps[0].Color != vector.Black {
		t.Errorf("bare document drew %+v, want one black line", ps)
	}
}

func TestDrawPaperFallback(t *testing.T) {
	doc := metricDoc(&Line{Common: Common{Invisible: true}, B: Point{X: 1}})
	doc.Paper = []Entity{line(0, 0, 3, 0)}
	d := mustDraw(t, doc)
	if len(d.Items) != 1 || !hasNote(d, "drew the paper layout") {
		t.Errorf("%d items, notes %q; want the paper line and a note", len(d.Items), d.Notes)
	}

	// Model space with anything in it wins, and nothing is said.
	doc = metricDoc(line(0, 0, 1, 0))
	doc.Paper = []Entity{line(0, 0, 3, 0)}
	d = mustDraw(t, doc)
	checkBounds(t, "model", d.Bounds(), Point{}, Point{X: 1})
	if len(d.Notes) != 0 {
		t.Errorf("notes %q, want none", d.Notes)
	}

	// Both empty: nothing to draw and nothing to explain.
	d = mustDraw(t, metricDoc())
	if len(d.Items) != 0 || len(d.Notes) != 0 {
		t.Errorf("empty document gave %d items, notes %q", len(d.Items), d.Notes)
	}
}

func TestDrawNotes(t *testing.T) {
	doc := metricDoc(line(0, 0, 1, 0))
	doc.Skipped = 3
	doc.Notes = []string{"reader note"}
	d := mustDraw(t, doc)
	if !hasNote(d, "3 objects the reader couldn't decode") || !hasNote(d, "reader note") {
		t.Errorf("notes %q, want the skipped count and the reader's note", d.Notes)
	}
}

func TestDrawUnits(t *testing.T) {
	tests := []struct {
		name    string
		units   Units
		long    float64
		unit    float64
		assumed bool
		note    string
	}{
		{name: "stated feet", units: Feet, long: 50, unit: 0.3048},
		{name: "stated mm", units: Millimeters, long: 1e6, unit: 0.001},
		{
			name:    "12 m building in mm",
			long:    12000,
			unit:    0.001,
			assumed: true,
			note:    "assumed millimetres",
		},
		{name: "30 m building in metres", long: 30, unit: 1, assumed: true, note: "assumed metres"},
		{
			name:    "5 m in centimetres",
			long:    500,
			unit:    0.01,
			assumed: true,
			note:    "assumed centimetres",
		},
		{name: "no plausible unit", long: 1e7, unit: 0, note: "size doesn't suggest any"},
		{name: "too small for any", long: 0.5, unit: 0, note: "size doesn't suggest any"},
	}
	for _, tt := range tests {
		doc := NewDocument()
		doc.Units = tt.units
		doc.Model = []Entity{line(0, 0, tt.long, tt.long/2)}
		d := mustDraw(t, doc)
		if d.Unit != tt.unit || d.Assumed != tt.assumed {
			t.Errorf(
				"%s: unit %v assumed %v, want %v %v",
				tt.name,
				d.Unit,
				d.Assumed,
				tt.unit,
				tt.assumed,
			)
		}
		if tt.note == "" && len(d.Notes) != 0 {
			t.Errorf("%s: notes %q, want none", tt.name, d.Notes)
		}
		if tt.note != "" && !hasNote(d, tt.note) {
			t.Errorf("%s: notes %q, want one saying %q", tt.name, d.Notes, tt.note)
		}
	}

	// Without units or anything drawn there is no size to guess from.
	d := mustDraw(t, NewDocument())
	if d.Unit != 0 || d.Assumed || len(d.Notes) != 0 {
		t.Errorf("empty unitless: unit %v assumed %v notes %q", d.Unit, d.Assumed, d.Notes)
	}
}

func TestDrawText(t *testing.T) {
	tests := []struct {
		name   string
		in     Text
		align  vector.Align
		valign vector.VAlign
	}{
		{name: "default", in: Text{}, align: vector.Left, valign: vector.Baseline},
		{
			name:   "centre middle",
			in:     Text{Align: 1, VAlign: 2},
			align:  vector.Center,
			valign: vector.Middle,
		},
		{name: "right top", in: Text{Align: 2, VAlign: 3}, align: vector.Right, valign: vector.Top},
		{
			name:   "centre bottom",
			in:     Text{Align: 1, VAlign: 1},
			align:  vector.Center,
			valign: vector.Bottom,
		},
		// Group 72 = 4 is "middle", centred both ways whatever group 73 says.
		{
			name:   "middle",
			in:     Text{Align: 4, VAlign: 3},
			align:  vector.Center,
			valign: vector.Middle,
		},
	}
	for _, tt := range tests {
		in := tt.in
		in.At = Point{X: 1, Y: 2}
		in.Height = 2.5
		in.Rotation = 0.3
		in.WidthFactor = 0.8
		in.Color = 1
		in.Content = "Room %%d"
		ts := texts(t, mustDraw(t, metricDoc(&in)))
		if len(ts) != 1 {
			t.Fatalf("%s: %d items, want 1", tt.name, len(ts))
		}
		got := ts[0]
		if got.Content != "Room °" || got.At != in.At || !near(got.Height, 2.5) ||
			!near(got.Angle, 0.3) ||
			got.WidthFactor != 0.8 ||
			got.Color != red {
			t.Errorf("%s: got %+v", tt.name, got)
		}
		if got.Align != tt.align || got.VAlign != tt.valign {
			t.Errorf(
				"%s: align %v/%v, want %v/%v",
				tt.name,
				got.Align,
				got.VAlign,
				tt.align,
				tt.valign,
			)
		}
	}

	// Blank text and text with no height draw nothing.
	d := mustDraw(t, metricDoc(
		&Text{Height: 1, Content: "  "},
		&Text{Height: 1, Content: "%%u"},
		&Text{Height: 0, Content: "x"},
	))
	if len(d.Items) != 0 {
		t.Errorf("blank texts drew %d items", len(d.Items))
	}
}

func TestDrawTextInInsertAndMirrored(t *testing.T) {
	doc := metricDoc(&Insert{
		Block:    "B",
		At:       Point{X: 10, Y: 0},
		Scale:    Point{X: 2, Y: 2},
		Rotation: math.Pi / 2,
		// Attributes are already where they sit in the drawing; the
		// insert's transform must not move them again.
		Attribs: []*Text{
			{Common: Common{Layer: "0"}, At: Point{X: 50, Y: 50}, Height: 1, Content: "A-101"},
		},
	})
	doc.Blocks["B"] = &Block{Name: "B", Entities: []Entity{
		&Text{At: Point{X: 1, Y: 0}, Height: 1, Content: "label"},
		// Mirrored text moves but stays readable.
		&Text{Common: Common{Mirrored: true}, At: Point{X: 1, Y: 0}, Height: 1, Content: "mirror"},
	}}
	ts := texts(t, mustDraw(t, doc))
	if len(ts) != 3 {
		t.Fatalf("%d items, want 3", len(ts))
	}
	if l := ts[0]; !nearPt(l.At, Point{X: 10, Y: 2}) || !near(l.Height, 2) ||
		!near(l.Angle, math.Pi/2) {
		t.Errorf("block text %+v, want at (10, 2) height 2 angle 90°", l)
	}
	if m := ts[1]; !nearPt(m.At, Point{X: 10, Y: -2}) || !near(m.Height, 2) {
		t.Errorf("mirrored text %+v, want at (10, -2) height 2", m)
	}
	if a := ts[2]; a.Content != "A-101" || a.At != (Point{X: 50, Y: 50}) || a.Height != 1 {
		t.Errorf("attribute %+v, want untransformed at (50, 50)", a)
	}

	// A text mirrored at the top level keeps reading left to right.
	ts = texts(
		t,
		mustDraw(
			t,
			metricDoc(
				&Text{Common: Common{Mirrored: true}, At: Point{X: 3}, Height: 1, Content: "x"},
			),
		),
	)
	if len(ts) != 1 || !nearPt(ts[0].At, Point{X: -3}) || math.Cos(ts[0].Angle) < 0.99 {
		t.Errorf("mirrored text %+v, want at (-3, 0) reading rightwards", ts)
	}
}

func TestDrawMText(t *testing.T) {
	const gap = 5.0 / 3 // MTEXT's line spacing for a height of 1
	tests := []struct {
		name   string
		attach int
		rot    float64
		align  vector.Align
		at     []Point
	}{
		{
			name:   "top left",
			attach: 1,
			align:  vector.Left,
			at:     []Point{{X: 0, Y: 0}, {X: 0, Y: -gap}},
		},
		{
			name:   "unset is top left",
			attach: 0,
			align:  vector.Left,
			at:     []Point{{X: 0, Y: 0}, {X: 0, Y: -gap}},
		},
		{
			name:   "out of range is top left",
			attach: 12,
			align:  vector.Left,
			at:     []Point{{X: 0, Y: 0}, {X: 0, Y: -gap}},
		},
		{
			// Two lines of height 1 span 1 + gap; the middle attachment
			// puts half of that above the anchor.
			name: "middle centre", attach: 5, align: vector.Center,
			at: []Point{{X: 0, Y: (1 + gap) / 2}, {X: 0, Y: (1+gap)/2 - gap}},
		},
		{
			name:   "bottom right",
			attach: 9,
			align:  vector.Right,
			at:     []Point{{X: 0, Y: 1 + gap}, {X: 0, Y: 1}},
		},
		{
			name: "rotated", attach: 1, rot: math.Pi / 2, align: vector.Left,
			at: []Point{{X: 0, Y: 0}, {X: gap, Y: 0}},
		},
	}
	for _, tt := range tests {
		doc := metricDoc(&MText{
			Common: Common{Color: 5}, Height: 1, Rotation: tt.rot, Attach: tt.attach,
			Content: `{\fArial;Room}\PB-1`,
		})
		ts := texts(t, mustDraw(t, doc))
		if len(ts) != 2 {
			t.Fatalf("%s: %d items, want 2", tt.name, len(ts))
		}
		for i, x := range ts {
			if !nearPt(x.At, tt.at[i]) || x.Align != tt.align || x.VAlign != vector.Top ||
				!near(x.Angle, tt.rot) || !near(x.Height, 1) || x.Color != blue {
				t.Errorf("%s: line %d %+v, want at %v align %v", tt.name, i, x, tt.at[i], tt.align)
			}
		}
		if ts[0].Content != "Room" || ts[1].Content != "B-1" {
			t.Errorf("%s: contents %q %q", tt.name, ts[0].Content, ts[1].Content)
		}
	}

	// A blank line keeps its space but draws nothing.
	ts := texts(t, mustDraw(t, metricDoc(&MText{Height: 1, Content: `A\P\PB`})))
	if len(ts) != 2 || !nearPt(ts[1].At, Point{Y: -2 * gap}) {
		t.Errorf("blank line: %+v, want B two lines down", ts)
	}

	d := mustDraw(t, metricDoc(&MText{Height: 1, Content: `{\H2;}\P `}, &MText{Content: "x"}))
	if len(d.Items) != 0 {
		t.Errorf("blank or zero-height mtext drew %d items", len(d.Items))
	}
}

func TestDrawHatchAndSolid(t *testing.T) {
	var boundary vector.Path
	boundary.Polyline([]Point{{X: 0, Y: 0}, {X: 4, Y: 0}, {X: 4, Y: 3}}, true)
	solid := &Hatch{Common: Common{Color: 1}, Solid: true, Boundary: boundary}
	pattern := &Hatch{Common: Common{Color: 1}, Boundary: boundary}
	empty := &Hatch{Common: Common{Color: 1}, Solid: true}
	quad := &Solid{
		Common:  Common{Color: 3},
		Corners: [4]Point{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 0, Y: 1}, {X: 1, Y: 1}},
	}
	ps := paths(t, mustDraw(t, metricDoc(solid, pattern, empty, quad)))
	if len(ps) != 3 {
		t.Fatalf("%d items, want two hatches and a solid", len(ps))
	}
	if p := ps[0]; !p.Fill || p.Stroke || p.Color != red || len(p.Segs) != len(boundary.Segs) {
		t.Errorf("solid hatch %+v, want a red fill of the boundary", p)
	}
	if p := ps[1]; !p.Fill || p.Stroke || p.Color != patternShade {
		t.Errorf("pattern hatch %+v, want a light fill", p)
	}
	// Drawing must not write its fill flags back into the document.
	if solid.Boundary.Fill || solid.Boundary.Stroke {
		t.Error("Draw changed the hatch's own boundary")
	}
	// DXF lists a solid's third and fourth corners crosswise; drawn in file
	// order the square would be a bow tie.
	q := ps[2]
	want := []Point{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 1}}
	if !q.Fill || !q.Stroke || q.Color != green || len(q.Segs) != 5 ||
		q.Segs[4].Op != vector.Close {
		t.Fatalf("solid %+v, want a closed green filled quad", q)
	}
	for i, w := range want {
		if q.Segs[i].P[0] != w {
			t.Errorf("solid corner %d = %v, want %v", i, q.Segs[i].P[0], w)
		}
	}
}

func TestDrawCurves(t *testing.T) {
	tests := []struct {
		name   string
		e      Entity
		lo, hi Point
	}{
		{
			name: "circle",
			e:    &Circle{Center: Point{X: 2, Y: 3}, Radius: 1},
			lo:   Point{X: 1, Y: 2},
			hi:   Point{X: 3, Y: 4},
		},
		{
			// Object coordinates with a downward extrusion run right to
			// left, so the centre's X flips.
			name: "mirrored circle",
			e:    &Circle{Common: Common{Mirrored: true}, Center: Point{X: 2, Y: 3}, Radius: 1},
			lo:   Point{X: -3, Y: 2},
			hi:   Point{X: -1, Y: 4},
		},
		{
			name: "arc first quadrant", e: &Arc{Radius: 1, Start: 0, End: math.Pi / 2},
			lo: Point{X: 0, Y: 0}, hi: Point{X: 1, Y: 1},
		},
		{
			// An end angle below the start wraps through 0.
			name: "arc across zero", e: &Arc{Radius: 1, Start: 3 * math.Pi / 2, End: 0},
			lo: Point{X: 0, Y: -1}, hi: Point{X: 1, Y: 0},
		},
		{
			name: "mirrored arc",
			e: &Arc{
				Common: Common{Mirrored: true},
				Center: Point{X: 5},
				Radius: 1,
				Start:  0,
				End:    math.Pi / 2,
			},
			lo: Point{X: -6, Y: 0},
			hi: Point{X: -5, Y: 1},
		},
		{
			name: "full ellipse",
			e:    &Ellipse{Major: Point{X: 2}, Ratio: 0.5, Start: 0, End: 2 * math.Pi},
			lo:   Point{X: -2, Y: -1},
			hi:   Point{X: 2, Y: 1},
		},
		{
			name: "ellipse across zero",
			e:    &Ellipse{Major: Point{X: 2}, Ratio: 0.5, Start: 3 * math.Pi / 2, End: 0},
			lo:   Point{X: 0, Y: -1},
			hi:   Point{X: 2, Y: 0},
		},
		{
			name: "polyline bulge",
			e:    &Polyline{Vertices: []Vertex{{P: Point{}, Bulge: 1}, {P: Point{X: 2}}}},
			lo:   Point{X: 0, Y: -1},
			hi:   Point{X: 2, Y: 0},
		},
		{
			name: "spline",
			e:    &Spline{Degree: 1, Control: []Point{{X: 0, Y: 0}, {X: 3, Y: 0}, {X: 3, Y: 2}}},
			lo:   Point{X: 0, Y: 0},
			hi:   Point{X: 3, Y: 2},
		},
	}
	for _, tt := range tests {
		d := mustDraw(t, metricDoc(tt.e))
		if len(d.Items) != 1 {
			t.Fatalf("%s: %d items, want 1", tt.name, len(d.Items))
		}
		checkBounds(t, tt.name, d.Bounds(), tt.lo, tt.hi)
	}

	// An equal start and end angle is a whole circle, not nothing.
	ps := paths(t, mustDraw(t, metricDoc(&Arc{Radius: 1, Start: 1, End: 1})))
	if len(ps) != 1 {
		t.Fatal("arc with equal angles drew nothing")
	}
	segs := ps[0].Segs
	start := Point{X: math.Cos(1), Y: math.Sin(1)}
	if len(segs) != 5 || !nearPt(segs[0].P[0], start) || !nearPt(segs[4].P[2], start) {
		t.Errorf("full arc segs %+v, want four quarters from and back to %v", segs, start)
	}
}

func TestDrawPolylineAndSplineDetails(t *testing.T) {
	doc := metricDoc(&Insert{Block: "B", Scale: Point{X: 2, Y: 2}})
	doc.Blocks["B"] = &Block{Name: "B", Entities: []Entity{
		&Polyline{
			Vertices: []Vertex{{P: Point{}}, {P: Point{X: 1}}, {P: Point{X: 1, Y: 1}}},
			Closed:   true,
			Width:    0.5,
		},
		&Polyline{},
		&Spline{
			Degree:  1,
			Closed:  true,
			Control: []Point{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}},
		},
		// Fit points already run the whole way round a closed curve;
		// closing again would draw a stray chord.
		&Spline{Closed: true, Fit: []Point{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}}},
		&Spline{Control: []Point{{X: 1, Y: 1}}},
	}}
	ps := paths(t, mustDraw(t, doc))
	if len(ps) != 3 {
		t.Fatalf("%d items, want polyline and two splines", len(ps))
	}
	// Width is a length in the block and scales with it.
	if !near(ps[0].Width, 1) || ps[0].Segs[len(ps[0].Segs)-1].Op != vector.Close {
		t.Errorf(
			"polyline width %v last op %v, want 1 and closed",
			ps[0].Width,
			ps[0].Segs[len(ps[0].Segs)-1].Op,
		)
	}
	if last := ps[1].Segs[len(ps[1].Segs)-1]; last.Op != vector.Close {
		t.Errorf("closed control spline ends with %v, want Close", last.Op)
	}
	if last := ps[2].Segs[len(ps[2].Segs)-1]; last.Op == vector.Close {
		t.Error("closed fit-point spline was closed again")
	}
}

func TestDrawDimension(t *testing.T) {
	doc := metricDoc(
		&Dimension{Common: Common{Layer: "dims", Color: ByLayer}, Block: "*D1"},
		&Dimension{Block: "missing"},
	)
	doc.Layers["dims"] = &Layer{Name: "dims", Color: 1}
	// Dimension blocks are stored in drawing coordinates, so they are
	// drawn where they are, in the dimension's colour when BYBLOCK.
	doc.Blocks["*D1"] = &Block{Name: "*D1", Entities: []Entity{
		&Line{Common: Common{Color: ByBlock}, A: Point{X: 3, Y: 4}, B: Point{X: 8, Y: 4}},
	}}
	ps := paths(t, mustDraw(t, doc))
	if len(ps) != 1 {
		t.Fatalf("%d items, want the dimension line", len(ps))
	}
	checkBounds(t, "dimension", ps[0].Bounds(), Point{X: 3, Y: 4}, Point{X: 8, Y: 4})
	if ps[0].Color != red {
		t.Errorf("colour %v, want the dimension layer's red", ps[0].Color)
	}
}

// strange is an entity type Draw doesn't know, as a future reader might add.
type strange struct{ Common }

func TestDrawSkipsUnknownAndMissing(t *testing.T) {
	d := mustDraw(t, metricDoc(
		&strange{},
		&Insert{Block: "nowhere"},
		&Line{Common: Common{Invisible: true}, B: Point{X: 1}},
		&Circle{Common: Common{Invisible: true}, Radius: 1},
	))
	if len(d.Items) != 0 {
		t.Errorf("drew %d items, want none", len(d.Items))
	}
}
