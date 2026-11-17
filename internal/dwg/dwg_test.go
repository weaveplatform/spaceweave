package dwg

import (
	"bytes"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/dwg/dwgtest"
	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// fixtures are the dxftest drawings every version is checked against.
var fixtures = map[string]func() *dxftest.Doc{
	"office":    func() *dxftest.Doc { return dxftest.Office(4) },
	"plan":      dxftest.Plan,
	"unlayered": func() *dxftest.Doc { return dxftest.Unlayered(6) },
	"grid":      func() *dxftest.Doc { return dxftest.Grid(30, 30) },
}

// TestMatchesDXF writes each fixture, as the DXF reader reads it, as DWG of
// every version and checks the DWG reader reads back the same drawing.
func TestMatchesDXF(t *testing.T) {
	for name, fixture := range fixtures {
		want := dwgtest.FromDXF(fixture())
		for _, v := range dwgtest.Versions {
			t.Run(name+"/"+v, func(t *testing.T) {
				got, err := Read(bytes.NewReader(dwgtest.File(v, want)))
				if err != nil {
					t.Fatal(err)
				}
				if got.Version != v || got.Skipped != 0 || len(got.Notes) != 0 {
					t.Errorf(
						"version %s, skipped %d, notes %q",
						got.Version,
						got.Skipped,
						got.Notes,
					)
				}
				sameDocument(t, got, want)
			})
		}
	}
}

// sameDocument compares what both readers fill in, to a micrometre: the
// writer stores some values (a text's direction) in another form.
func sameDocument(t *testing.T, got, want *cad.Document) {
	t.Helper()
	if got.Units != want.Units {
		t.Errorf("units %v, want %v", got.Units, want.Units)
	}
	if !equal(got.Layers, want.Layers) {
		t.Errorf("layers\n got %s\nwant %s", dump(got.Layers), dump(want.Layers))
	}
	sameEntities(t, "model", got.Model, want.Model)
	sameEntities(t, "paper", got.Paper, want.Paper)
	if len(got.Blocks) != len(want.Blocks) {
		t.Errorf("%d blocks, want %d", len(got.Blocks), len(want.Blocks))
	}
	for n, w := range want.Blocks {
		g := got.Blocks[n]
		if g == nil {
			t.Errorf("no block %q", n)
			continue
		}
		if g.Name != w.Name || !equal(g.Base, w.Base) {
			t.Errorf("block %q: %+v, want %+v", n, g, w)
		}
		sameEntities(t, "block "+n, g.Entities, w.Entities)
	}
}

func sameEntities(t *testing.T, where string, got, want []cad.Entity) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: %d entities, want %d", where, len(got), len(want))
		return
	}
	for i := range got {
		if g, w, ok := curvedHatches(got[i], want[i]); ok {
			// The writer flattens curved hatch edges (DXF's arcs become
			// Bézier curves, which DWG has no form for) to lines through
			// the same points; the edge forms are checked on their own.
			if gb, wb := g.Boundary.Bounds(), w.Boundary.Bounds(); !equal(
				[]cad.Point{gb.Min, gb.Max},
				[]cad.Point{wb.Min, wb.Max},
			) {
				t.Errorf(
					"%s[%d]: hatch bounds %+v, want %+v",
					where,
					i,
					g.Boundary.Bounds(),
					w.Boundary.Bounds(),
				)
			}
			continue
		}
		if !equal(got[i], want[i]) {
			t.Errorf("%s[%d]:\n got %s\nwant %s", where, i, dump(got[i]), dump(want[i]))
		}
	}
}

func dump(v any) string { return fmt.Sprintf("%T %+v", v, deref(v)) }

func deref(v any) any {
	r := reflect.ValueOf(v)
	if r.Kind() == reflect.Pointer && !r.IsNil() {
		return r.Elem().Interface()
	}
	return v
}

// equal is reflect.DeepEqual with floats rounded to 1e-6 and empty slices
// the same as nil.
func equal(a, b any) bool {
	return reflect.DeepEqual(normal(reflect.ValueOf(a)), normal(reflect.ValueOf(b)))
}

func normal(v reflect.Value) any {
	switch v.Kind() {
	case reflect.Float64:
		f := math.Round(v.Float()*1e6) / 1e6
		if f == 0 {
			f = 0 // no negative zero
		}
		return f
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return nil
		}
		return []any{v.Elem().Type().String(), normal(v.Elem())}
	case reflect.Slice, reflect.Array:
		if v.Len() == 0 {
			return nil
		}
		out := make([]any, v.Len())
		for i := range out {
			out[i] = normal(v.Index(i))
		}
		return out
	case reflect.Map:
		out := map[string]any{}
		for _, k := range v.MapKeys() {
			out[fmt.Sprint(k.Interface())] = normal(v.MapIndex(k))
		}
		return out
	case reflect.Struct:
		out := map[string]any{}
		for i := range v.NumField() {
			out[v.Type().Field(i).Name] = normal(v.Field(i))
		}
		return out
	default:
		return v.Interface()
	}
}

// TestExtras checks what the reader does with objects it doesn't draw or
// can't decode.
func TestExtras(t *testing.T) {
	for _, v := range dwgtest.Versions {
		t.Run(v, func(t *testing.T) {
			want := dwgtest.FromDXF(dxftest.Office(4))
			data := dwgtest.Drawing{Version: v, Doc: want, Extras: true}.Bytes()
			got, st, err := decode(data)
			if err != nil {
				t.Fatal(err)
			}
			if got.Skipped != 2 || st.skipped["LINE"] != 1 || st.skipped["class 599"] != 1 {
				t.Errorf("skipped %d: %v", got.Skipped, st.skipped)
			}
			note := "Not drawn: 1 3DFACE, 1 AEC_WALL, 1 POLYLINE."
			if len(got.Notes) != 1 || got.Notes[0] != note {
				t.Errorf("notes %q, want %q", got.Notes, note)
			}
			sameDocument(t, got, want)
		})
	}
}

func TestOfficeFixture(t *testing.T) {
	for _, v := range dwgtest.Versions {
		doc, err := Read(bytes.NewReader(dwgtest.Office(v)))
		if err != nil {
			t.Fatal(err)
		}
		if doc.Units != cad.Millimeters || len(doc.Model) == 0 ||
			!strings.HasPrefix(doc.Version, "AC10") {
			t.Errorf("%s: %+v", v, doc)
		}
	}
}

func curvedHatches(a, b cad.Entity) (*cad.Hatch, *cad.Hatch, bool) {
	g, ok1 := a.(*cad.Hatch)
	w, ok2 := b.(*cad.Hatch)
	if !ok1 || !ok2 {
		return nil, nil, false
	}
	for _, s := range w.Boundary.Segs {
		if s.Op == vector.CubicTo {
			return g, w, true
		}
	}
	return nil, nil, false
}
