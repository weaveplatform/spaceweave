package dwg

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/dwg/dwgtest"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// sink is a drawing with every entity the reader decodes, in the forms the
// DXF fixtures don't use: ellipses, splines, solids, bulged and wide
// polylines, true colours, aligned and rotated text, every insert scale
// form, and enough attributes, polylines and so on that the writer's
// alternative encodings all turn up.
func sink(v string) *cad.Document {
	doc := cad.NewDocument()
	doc.Units = cad.Meters
	red := &vector.Color{R: 200, G: 10, B: 20}
	doc.Layers["WALL"] = &cad.Layer{Name: "WALL", Color: 5, RGB: red}
	doc.Layers["OFF"] = &cad.Layer{Name: "OFF", Color: 3, Off: true}
	doc.Layers["FROZEN"] = &cad.Layer{Name: "FROZEN", Color: 4, Frozen: true, Off: true}
	if v == "AC1015" {
		doc.Layers["WALL"].RGB = nil // R2000 has no true colours
	}
	byLayer := cad.Common{Layer: "WALL", Color: cad.ByLayer}
	colours := []cad.Common{
		byLayer,
		{Layer: "OFF", Color: 1},
		{Layer: "0", Color: cad.ByBlock},
		{Layer: "WALL", Color: 7, Invisible: true},
		{Layer: "FROZEN", Color: cad.ByLayer, Mirrored: true},
	}
	if v != "AC1015" {
		colours = append(colours, cad.Common{Layer: "WALL", Color: cad.ByLayer, RGB: red})
	}
	var ents []cad.Entity
	for i, c := range colours {
		f := float64(i)
		pl := []cad.Vertex{
			{P: cad.Point{X: f, Y: 0}, Bulge: 0.5},
			{P: cad.Point{X: f + 10, Y: 1e-7}},
			{P: cad.Point{X: f + 10, Y: 1e9}},
		}
		ents = append(
			ents,
			&cad.Line{Common: c, A: cad.Point{X: f, Y: 1}, B: cad.Point{X: f + 1.5, Y: 1 + 1e-12}},
			&cad.Circle{Common: c, Center: cad.Point{X: f, Y: -3}, Radius: 2},
			&cad.Arc{Common: c, Center: cad.Point{X: f}, Radius: 1, Start: 0.1, End: 4},
			&cad.Polyline{Common: c, Vertices: pl, Closed: i%2 == 0, Width: f},
			&cad.Polyline{
				Common:   c,
				Vertices: []cad.Vertex{{P: cad.Point{X: 1, Y: f}}, {P: cad.Point{X: 2, Y: 3}}},
			},
			&cad.Polyline{
				Common:   c,
				Vertices: []cad.Vertex{{P: cad.Point{X: 1, Y: f}}, {P: cad.Point{X: 5, Y: 3}}},
				Closed:   true,
			},
			&cad.Solid{Common: c, Corners: [4]cad.Point{{X: f}, {X: 1}, {Y: 1}, {X: 1, Y: 1}}},
			&cad.Hatch{Common: c, Solid: i%2 == 0, Boundary: square(f)},
		)
		flat := c
		flat.Mirrored = false // ellipses and splines are in world coordinates
		ents = append(
			ents,
			&cad.Ellipse{
				Common: flat,
				Center: cad.Point{X: f},
				Major:  cad.Point{X: 3, Y: 1},
				Ratio:  0.5,
				Start:  0,
				End:    math.Pi,
			},
			&cad.Spline{
				Common: flat, Degree: 3, Knots: []float64{0, 0, 0, 0, 1, 1, 1, 1},
				Control: []cad.Point{{X: f}, {X: 1, Y: 2}, {X: 3, Y: 2}, {X: 4}},
				Weights: weights(i), Closed: i%2 == 1,
			},
			&cad.Spline{Common: flat, Degree: 3, Fit: []cad.Point{{X: f}, {X: 2, Y: 1}, {X: 5}}},
			&cad.MText{
				Common:   c,
				At:       cad.Point{X: f, Y: 9},
				Height:   2.5,
				Rotation: f / 3,
				Attach:   1 + i,
				Content:  "Room\\Pº",
			},
		)
		for j, al := range [][2]int{{0, 0}, {1, 2}, {3, 0}, {5, 0}, {2, 3}, {0, 1}} {
			ents = append(ents, &cad.Text{
				Common:      c,
				At:          cad.Point{X: f, Y: float64(j)},
				Height:      1 + f,
				Rotation:    f / 2,
				WidthFactor: 1 - f/10,
				Align:       al[0],
				VAlign:      al[1],
				Content:     "Café " + string(rune('A'+j)),
			})
		}
	}
	doc.Blocks["BOX"] = &cad.Block{Name: "BOX", Base: cad.Point{X: 1, Y: 2}, Entities: ents[:4]}
	doc.Blocks["*U7"] = &cad.Block{Name: "*U7", Entities: ents[4:6]}
	attr := func(s string, k int) *cad.Text {
		return &cad.Text{
			Common: cad.Common{Layer: "WALL", Color: cad.ByLayer, Invisible: k == 2},
			At:     cad.Point{X: float64(k)}, Height: 1, WidthFactor: 1, Content: s,
		}
	}
	for i, scale := range []cad.Point{{X: 1, Y: 1}, {X: 2, Y: 2}, {X: 1, Y: 3}, {X: 2, Y: -2}, {X: 0.5, Y: 1}} {
		in := &cad.Insert{
			Common:   byLayer,
			Block:    "BOX",
			At:       cad.Point{X: float64(i)},
			Scale:    scale,
			Rotation: 0.3 * float64(i),
		}
		switch i {
		case 1:
			in.Attribs = []*cad.Text{attr("A", 0), attr("B", 1), attr("C", 2), attr("D", 3)}
		case 2:
			in.Columns, in.Rows, in.ColumnGap, in.RowGap = 3, 2, 10, 20
		case 3:
			in.Attribs = []*cad.Text{attr("only", 1)}
			in.Mirrored = true
		}
		ents = append(ents, in)
	}
	ents = append(ents,
		&cad.Insert{Common: byLayer, Block: "*U7", Scale: cad.Point{X: 1, Y: 1}},
		&cad.Dimension{Common: byLayer, Block: "*U7"},
		&cad.Dimension{Common: byLayer, Block: "*U7"},
	)
	doc.Model = ents
	// Long enough that an R2007+ string stream's size needs a high word.
	long := &cad.MText{
		Common:  byLayer,
		Height:  1,
		Attach:  1,
		Content: strings.Repeat("long text ", 300),
	}
	doc.Paper = []cad.Entity{&cad.Line{Common: byLayer, B: cad.Point{X: 297, Y: 210}}, long}
	return doc
}

func weights(i int) []float64 {
	if i%2 == 0 {
		return nil
	}
	return []float64{1, 0.5, 0.5, 1}
}

func square(f float64) vector.Path {
	var p vector.Path
	cad.AddPolyline(
		&p,
		[]cad.Vertex{
			{P: cad.Point{X: f}},
			{P: cad.Point{X: f + 1}},
			{P: cad.Point{X: f + 1, Y: 1}},
		},
		true,
	)
	cad.AddPolyline(
		&p,
		[]cad.Vertex{
			{P: cad.Point{X: f + 0.2, Y: 0.1}},
			{P: cad.Point{X: f + 0.3, Y: 0.1}},
			{P: cad.Point{X: f + 0.3, Y: 0.2}},
		},
		true,
	)
	return p
}

// TestSink round-trips the sink drawing through every version.
func TestSink(t *testing.T) {
	for _, v := range dwgtest.Versions {
		t.Run(v, func(t *testing.T) {
			want := sink(v)
			for _, m := range []byte{0, 4} {
				data := dwgtest.Drawing{Version: v, Maintenance: m, Doc: want}.Bytes()
				got, err := Read(bytes.NewReader(data))
				if err != nil {
					t.Fatal(err)
				}
				if got.Skipped != 0 || len(got.Notes) != 0 {
					t.Errorf("skipped %d, notes %q", got.Skipped, got.Notes)
				}
				sameDocument(t, got, want)
			}
		})
	}
}
