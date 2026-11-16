package dxf

import (
	"math"
	"reflect"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

func TestCommon(t *testing.T) {
	l := one[*cad.Line](t, dxftest.E("LINE"))
	if l.Common != (cad.Common{Layer: "0", Color: cad.ByLayer}) {
		t.Errorf("defaults %+v, want layer 0 and BYLAYER", l.Common)
	}
	l = one[*cad.Line](t, dxftest.E("LINE", 8, "A", 62, 0, 420, 0xff8000, 60, 1, 230, -1.0))
	want := cad.Common{
		Layer:     "A",
		Color:     cad.ByBlock,
		RGB:       &vector.Color{R: 0xff, G: 0x80},
		Invisible: true,
		Mirrored:  true,
	}
	if !reflect.DeepEqual(l.Common, want) {
		t.Errorf("common %+v, want %+v", l.Common, want)
	}
	// An upward extrusion and visibility 0 change nothing.
	l = one[*cad.Line](t, dxftest.E("LINE", 60, 0, 230, 1.0))
	if l.Invisible || l.Mirrored {
		t.Errorf("common %+v", l.Common)
	}
}

func TestLine(t *testing.T) {
	l := one[*cad.Line](t, dxftest.Line("W", 1, 2, 3, 4))
	if l.A != (cad.Point{X: 1, Y: 2}) || l.B != (cad.Point{X: 3, Y: 4}) || l.Layer != "W" {
		t.Errorf("line %+v", l)
	}
}

func TestCircleAndArc(t *testing.T) {
	c := one[*cad.Circle](t, dxftest.E("CIRCLE", 10, 1.0, 20, 2.0, 40, 3.5))
	if c.Center != (cad.Point{X: 1, Y: 2}) || c.Radius != 3.5 {
		t.Errorf("circle %+v", c)
	}
	// The file holds degrees; the model holds radians.
	a := one[*cad.Arc](t, dxftest.E("ARC", 10, 1.0, 20, 2.0, 40, 3.0, 50, 90.0, 51, 180.0))
	if a.Center != (cad.Point{X: 1, Y: 2}) || a.Radius != 3 || !near(a.Start, math.Pi/2) ||
		!near(a.End, math.Pi) {
		t.Errorf("arc %+v", a)
	}
}

func TestEllipse(t *testing.T) {
	e := one[*cad.Ellipse](t, dxftest.E("ELLIPSE",
		10, 1.0, 20, 2.0, 11, 4.0, 21, 0.0, 40, 0.25, 41, 0.5, 42, 2.5, 230, -1.0))
	want := &cad.Ellipse{
		Common: cad.Common{Layer: "0", Color: cad.ByLayer}, // world coordinates: never mirrored
		Center: cad.Point{X: 1, Y: 2}, Major: cad.Point{X: 4}, Ratio: 0.25, Start: 0.5, End: 2.5,
	}
	if !reflect.DeepEqual(e, want) {
		t.Errorf("ellipse %+v, want %+v", e, want)
	}
	// Without an end parameter the ellipse is whole.
	e = one[*cad.Ellipse](t, dxftest.E("ELLIPSE", 11, 1.0, 40, 1.0))
	if e.Start != 0 || e.End != 2*math.Pi {
		t.Errorf("full ellipse runs %v to %v", e.Start, e.End)
	}
}

func TestLWPolyline(t *testing.T) {
	p := one[*cad.Polyline](t, dxftest.E("LWPOLYLINE", 90, 3, 70, 129, 43, 5.0,
		// A Y or bulge before any X has no vertex to land on.
		20, 99.0, 42, 9.0,
		10, 0.0, 20, 0.0, 42, 1.0,
		10, 10.0, 20, 0.0, 40, 2.0, 41, 7.0,
		10, 10.0, 20, 10.0, 42, -0.5))
	want := []cad.Vertex{
		{P: cad.Point{}, Bulge: 1},
		{P: cad.Point{X: 10}},
		{P: cad.Point{X: 10, Y: 10}, Bulge: -0.5},
	}
	if !reflect.DeepEqual(p.Vertices, want) || !p.Closed || p.Width != 7 {
		t.Errorf("lwpolyline %+v", p)
	}
	r := one[*cad.Polyline](t, dxftest.Rect("R", 0, 0, 2, 1))
	if !r.Closed || len(r.Vertices) != 4 || r.Vertices[2].P != (cad.Point{X: 2, Y: 1}) ||
		r.Width != 0 {
		t.Errorf("rect %+v", r)
	}
	open := one[*cad.Polyline](t, dxftest.E("LWPOLYLINE", 70, 128, 10, 1.0, 20, 1.0))
	if open.Closed {
		t.Error("flag 128 (linetype generation) read as closed")
	}
}

func TestPolyline(t *testing.T) {
	p := one[*cad.Polyline](t,
		dxftest.E("POLYLINE", 8, "P", 66, 1, 70, 1, 40, 3.0, 41, 4.0),
		dxftest.E("VERTEX", 10, 1.0, 20, 2.0, 42, 0.5),
		// A spline's frame control point is not on the curve.
		dxftest.E("VERTEX", 10, 50.0, 20, 50.0, 70, 16),
		dxftest.E("VERTEX", 10, 3.0, 20, 4.0, 70, 8),
		dxftest.E("SEQEND"),
	)
	want := []cad.Vertex{{P: cad.Point{X: 1, Y: 2}, Bulge: 0.5}, {P: cad.Point{X: 3, Y: 4}}}
	if !reflect.DeepEqual(p.Vertices, want) || !p.Closed || p.Width != 4 || p.Layer != "P" {
		t.Errorf("polyline %+v", p)
	}
}

func TestPolylineMeshesSkipped(t *testing.T) {
	for _, flags := range []int{16, 64, 17} {
		m := model(t,
			dxftest.E("POLYLINE", 66, 1, 70, flags),
			dxftest.E("VERTEX", 10, 1.0, 20, 2.0, 70, 64),
			dxftest.E("SEQEND"),
			dxftest.Line("0", 0, 0, 1, 1),
		)
		// The mesh goes, its vertices with it, and the line after survives.
		if len(m) != 1 {
			t.Errorf("flags %d: model %+v", flags, m)
			continue
		}
		if _, ok := m[0].(*cad.Line); !ok {
			t.Errorf("flags %d: got %T", flags, m[0])
		}
	}
}

func TestPolylineWithoutSEQEND(t *testing.T) {
	m := model(t,
		dxftest.E("POLYLINE"),
		dxftest.E("VERTEX", 10, 1.0),
		dxftest.E("VERTEX", 10, 2.0),
		dxftest.Line("0", 0, 0, 5, 5),
	)
	if len(m) != 2 {
		t.Fatalf("model %+v", m)
	}
	p, _ := m[0].(*cad.Polyline)
	if p == nil || len(p.Vertices) != 2 || p.Closed {
		t.Errorf("polyline %+v", m[0])
	}
	if l, _ := m[1].(*cad.Line); l == nil || l.B.X != 5 {
		t.Errorf("line %+v", m[1])
	}
}

func TestSpline(t *testing.T) {
	s := one[*cad.Spline](t, dxftest.E("SPLINE", 70, 11, 71, 2, 72, 6, 73, 3, 74, 2,
		21, 99.0, // no fit point yet
		40, 0.0, 40, 0.0, 40, 0.0, 40, 1.0, 40, 1.0, 40, 1.0,
		20, 99.0, // no control point yet
		10, 0.0, 20, 0.0, 41, 1.0,
		10, 5.0, 20, 5.0, 41, 0.5,
		10, 10.0, 20, 0.0, 41, 1.0,
		11, 0.0, 21, 0.0,
		11, 10.0, 21, 0.0))
	want := &cad.Spline{
		Common:  cad.Common{Layer: "0", Color: cad.ByLayer},
		Degree:  2,
		Knots:   []float64{0, 0, 0, 1, 1, 1},
		Control: []cad.Point{{}, {X: 5, Y: 5}, {X: 10}},
		Weights: []float64{1, 0.5, 1},
		Fit:     []cad.Point{{}, {X: 10}},
		Closed:  true,
	}
	if !reflect.DeepEqual(s, want) {
		t.Errorf("spline %+v, want %+v", s, want)
	}
	open := one[*cad.Spline](t, dxftest.E("SPLINE", 70, 8, 11, 1.0, 21, 2.0))
	if open.Closed || open.Degree != 0 || !reflect.DeepEqual(open.Fit, []cad.Point{{X: 1, Y: 2}}) {
		t.Errorf("fit-only spline %+v", open)
	}
}

func TestText(t *testing.T) {
	base := []any{10, 1.0, 20, 2.0, 11, 7.0, 21, 8.0, 40, 2.5, 41, 0.8, 50, 90.0, 1, "Room"}
	at := func(extra ...any) *cad.Text {
		t.Helper()
		return one[*cad.Text](t, dxftest.E("TEXT", append(append([]any{}, base...), extra...)...))
	}
	ins, align := cad.Point{X: 1, Y: 2}, cad.Point{X: 7, Y: 8}
	cases := []struct {
		name   string
		extra  []any
		h, v   int
		anchor cad.Point
	}{
		{"left baseline", nil, 0, 0, ins},
		{"centred", []any{72, 1}, 1, 0, align},
		{"top", []any{73, 3}, 0, 3, align},
		{"middle centre", []any{72, 4, 73, 0}, 4, 0, align},
		// Aligned and fit text stretch between both points; the insertion
		// point anchors them.
		{"aligned", []any{72, 3}, 3, 0, ins},
		{"fit", []any{72, 5, 73, 1}, 5, 1, ins},
	}
	for _, c := range cases {
		txt := at(c.extra...)
		if txt.Align != c.h || txt.VAlign != c.v || txt.At != c.anchor {
			t.Errorf(
				"%s: align %d/%d at %v, want %d/%d at %v",
				c.name,
				txt.Align,
				txt.VAlign,
				txt.At,
				c.h,
				c.v,
				c.anchor,
			)
		}
		if txt.Content != "Room" || txt.Height != 2.5 || txt.WidthFactor != 0.8 ||
			!near(txt.Rotation, math.Pi/2) {
			t.Errorf("%s: text %+v", c.name, txt)
		}
	}
	// Centred text without its alignment point falls back to the insertion
	// point rather than the origin.
	txt := one[*cad.Text](t, dxftest.E("TEXT", 10, 1.0, 20, 2.0, 72, 2, 1, "x"))
	if txt.At != ins {
		t.Errorf("no alignment point: at %v", txt.At)
	}
}

func TestMText(t *testing.T) {
	m := one[*cad.MText](t, dxftest.E("MTEXT", 10, 1.0, 20, 2.0, 40, 3.0, 50, 45.0, 71, 7,
		3, "first ", 1, "last", 3, "second "))
	// Chunks in code 3 come first, in order, whatever their position
	// relative to the final code 1.
	if m.Content != "first second last" || m.Attach != 7 || m.Height != 3 ||
		m.At != (cad.Point{X: 1, Y: 2}) || !near(m.Rotation, math.Pi/4) {
		t.Errorf("mtext %+v", m)
	}
	// The X-axis direction overrides the rotation angle.
	m = one[*cad.MText](t, dxftest.E("MTEXT", 50, 45.0, 11, 0.0, 21, 2.0, 1, "up"))
	if !near(m.Rotation, math.Pi/2) {
		t.Errorf("direction (0,2) gives rotation %v", m.Rotation)
	}
	// A zero direction is no direction: the angle stands.
	m = one[*cad.MText](t, dxftest.E("MTEXT", 50, 30.0, 11, 0.0, 21, 0.0, 1, "x"))
	if !near(m.Rotation, math.Pi/6) {
		t.Errorf("zero direction gives rotation %v", m.Rotation)
	}
}

func TestInsert(t *testing.T) {
	in := one[*cad.Insert](t, dxftest.E("INSERT", 8, "F", 2, "DESK", 10, 1.0, 20, 2.0))
	want := &cad.Insert{
		Common: cad.Common{Layer: "F", Color: cad.ByLayer},
		Block:  "DESK", At: cad.Point{X: 1, Y: 2}, Scale: cad.Point{X: 1, Y: 1},
	}
	if !reflect.DeepEqual(in, want) {
		t.Errorf("plain insert %+v, want %+v", in, want)
	}
	in = one[*cad.Insert](t, dxftest.E("INSERT", 2, "DESK", 41, -2.0, 42, 0.0, 50, 180.0,
		70, 3, 71, 2, 44, 10.0, 45, 20.0))
	if in.Scale != (cad.Point{X: -2, Y: 0}) || !near(in.Rotation, math.Pi) ||
		in.Columns != 3 || in.Rows != 2 || in.ColumnGap != 10 || in.RowGap != 20 {
		t.Errorf("array insert %+v", in)
	}
}

func TestInsertAttributes(t *testing.T) {
	in := one[*cad.Insert](
		t,
		dxftest.E("INSERT", 2, "ROOM", 66, 1, 10, 0.0, 20, 0.0),
		// An attribute keeps its vertical alignment in 74: 73 is its field
		// length and must not be read as alignment.
		dxftest.E(
			"ATTRIB",
			8,
			"TAGS",
			10,
			1.0,
			20,
			1.0,
			11,
			5.0,
			21,
			6.0,
			40,
			2.0,
			1,
			"R101",
			2,
			"NUMBER",
			70,
			1,
			72,
			1,
			73,
			9,
			74,
			2,
		),
		dxftest.E("ATTRIB", 62, 3, 10, 2.0, 20, 2.0, 1, "Kitchen", 70, 8),
		dxftest.E("SEQEND"),
	)
	if len(in.Attribs) != 2 {
		t.Fatalf("attribs %+v", in.Attribs)
	}
	a, b := in.Attribs[0], in.Attribs[1]
	if a.Content != "R101" || !a.Invisible || a.Align != 1 || a.VAlign != 2 ||
		a.At != (cad.Point{X: 5, Y: 6}) || a.Layer != "TAGS" || a.Height != 2 {
		t.Errorf("first attribute %+v", a)
	}
	if b.Content != "Kitchen" || b.Invisible || b.Color != 3 || b.At != (cad.Point{X: 2, Y: 2}) {
		t.Errorf("second attribute %+v", b)
	}
}

func TestInsertAttributesWithoutSEQEND(t *testing.T) {
	m := model(t,
		dxftest.E("INSERT", 2, "ROOM", 66, 1),
		dxftest.E("ATTRIB", 1, "a"),
		dxftest.Line("0", 0, 0, 1, 1),
	)
	if len(m) != 2 {
		t.Fatalf("model %+v", m)
	}
	if in, _ := m[0].(*cad.Insert); in == nil || len(in.Attribs) != 1 {
		t.Errorf("insert %+v", m[0])
	}
}

func TestInsertWithoutAttributesFlag(t *testing.T) {
	// Without 66 the ATTRIB is not the insert's; it is a stray entity of a
	// type that isn't drawn on its own.
	m := model(t, dxftest.E("INSERT", 2, "ROOM"), dxftest.E("ATTRIB", 1, "a"))
	if len(m) != 1 {
		t.Fatalf("model %+v", m)
	}
	if in, _ := m[0].(*cad.Insert); in == nil || len(in.Attribs) != 0 {
		t.Errorf("insert %+v", m[0])
	}
}

func TestSolidAndTrace(t *testing.T) {
	for _, kind := range []string{"SOLID", "TRACE"} {
		s := one[*cad.Solid](
			t,
			dxftest.E(kind, 10, 0.0, 20, 0.0, 11, 1.0, 21, 0.0, 12, 0.0, 22, 1.0, 13, 1.0, 23, 1.0),
		)
		want := [4]cad.Point{{}, {X: 1}, {Y: 1}, {X: 1, Y: 1}}
		if s.Corners != want {
			t.Errorf("%s corners %v, want %v", kind, s.Corners, want)
		}
	}
}

func TestDimension(t *testing.T) {
	d := one[*cad.Dimension](t, dxftest.E("DIMENSION", 8, "DIM", 2, "*D7"))
	if d.Block != "*D7" || d.Layer != "DIM" {
		t.Errorf("dimension %+v", d)
	}
}

// TestBinaryRangesMatchBuilder documents a mismatch between dxftest's binary
// writer and the reader's kind table: the builder writes codes 110-149 (and
// 160-179, 400-409, 440-469, 1004, 1010-1071) as strings, but binary DXF
// stores them as numbers, so the reader goes out of step.
func TestBinaryRangesMatchBuilder(t *testing.T) {
	readBoth(t, dxftest.New(4).Add(dxftest.E("LINE", 8, "A", 140, 2.5, 10, 1.0)))
}
