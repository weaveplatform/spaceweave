package dxftest

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/dxf"
)

// small is a drawing with one of everything the builder writes.
func small() *Doc {
	return New(4).
		Layer("A", -1, 1).
		Block("B", 1.5, 2, Line("0", 0, 0, 1, 1)).
		Add(E("POINT", 10, 0.5))
}

// smallPairs is small() as code/value pairs, in file order.
var smallPairs = []any{
	999, "spaceweave dxftest",
	0, "SECTION", 2, "HEADER",
	9, "$ACADVER", 1, "AC1027",
	9, "$INSUNITS", 70, 4,
	0, "ENDSEC",
	0, "SECTION", 2, "TABLES", 0, "TABLE", 2, "LAYER",
	0, "LAYER", 2, "A", 70, 1, 62, -1,
	0, "ENDTAB", 0, "ENDSEC",
	0, "SECTION", 2, "BLOCKS",
	0, "BLOCK", 8, "0", 2, "B", 10, 1.5, 20, 2.0,
	0, "LINE", 8, "0", 10, 0.0, 20, 0.0, 11, 1.0, 21, 1.0,
	0, "ENDBLK", 0, "ENDSEC",
	0, "SECTION", 2, "ENTITIES",
	0, "POINT", 10, 0.5,
	0, "ENDSEC", 0, "EOF",
}

func TestBytes(t *testing.T) {
	// Spelled out rather than generated: right-aligned codes and CRLF are
	// what AutoCAD writes, and readers that get them wrong exist.
	want := strings.Join([]string{
		"999", "spaceweave dxftest",
		"  0", "SECTION", "  2", "HEADER",
		"  9", "$ACADVER", "  1", "AC1027",
		"  9", "$INSUNITS", " 70", "4",
		"  0", "ENDSEC",
		"  0", "SECTION", "  2", "TABLES", "  0", "TABLE", "  2", "LAYER",
		"  0", "LAYER", "  2", "A", " 70", "1", " 62", "-1",
		"  0", "ENDTAB", "  0", "ENDSEC",
		"  0", "SECTION", "  2", "BLOCKS",
		"  0", "BLOCK", "  8", "0", "  2", "B", " 10", "1.5", " 20", "2",
		"  0", "LINE", "  8", "0", " 10", "0", " 20", "0", " 11", "1", " 21", "1",
		"  0", "ENDBLK", "  0", "ENDSEC",
		"  0", "SECTION", "  2", "ENTITIES",
		"  0", "POINT", " 10", "0.5",
		"  0", "ENDSEC", "  0", "EOF",
	}, "\r\n") + "\r\n"
	if got := string(small().Bytes()); got != want {
		t.Errorf("Bytes:\n%q\nwant\n%q", got, want)
	}
}

// encode writes pairs as binary DXF by hand, choosing each value's width
// from its Go type, independently of the builder's code ranges.
func encode(t *testing.T, pairs []any) []byte {
	t.Helper()
	b := bytes.NewBufferString("AutoCAD Binary DXF\r\n\x1a\x00")
	put := func(v any) {
		if err := binary.Write(b, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < len(pairs); i += 2 {
		code, _ := pairs[i].(int)
		if code == 999 {
			continue
		}
		put(uint16(code)) //nolint:gosec // test codes fit
		switch v := pairs[i+1].(type) {
		case string:
			b.WriteString(v)
			b.WriteByte(0)
		case float64:
			put(v)
		case int:
			put(int16(v)) //nolint:gosec // test values fit
		default:
			t.Fatalf("no encoding for %T", v)
		}
	}
	return b.Bytes()
}

func TestBinary(t *testing.T) {
	got, want := small().Binary(), encode(t, smallPairs)
	if !bytes.Equal(got, want) {
		t.Errorf("Binary:\n%q\nwant\n%q", got, want)
	}
}

func TestBinaryValueKinds(t *testing.T) {
	got := New(-1).Add(E("X",
		10, 1.5, 230, -1.0,
		62, -3, 280, 2, 370, 5,
		90, 70000, 420, 0xabcdef,
		290, 1,
		310, "ab",
		1, "hi",
	)).Binary()
	var b bytes.Buffer
	put := func(vs ...any) {
		for _, v := range vs {
			if err := binary.Write(&b, binary.LittleEndian, v); err != nil {
				t.Fatal(err)
			}
		}
	}
	put(uint16(0), []byte("X\x00"))
	put(uint16(10), 1.5, uint16(230), -1.0)
	put(uint16(62), int16(-3), uint16(280), int16(2), uint16(370), int16(5))
	put(uint16(90), int32(70000), uint16(420), int32(0xabcdef))
	put(uint16(290), uint8(1))
	put(uint16(310), uint8(2), []byte("ab"))
	put(uint16(1), []byte("hi\x00"))
	if !bytes.Contains(got, b.Bytes()) {
		t.Errorf("binary entity not found:\n%q\nin\n%q", b.Bytes(), got)
	}
	// Binary DXF has no comments, and no $INSUNITS was asked for.
	if bytes.Contains(got, []byte("spaceweave dxftest")) ||
		bytes.Contains(got, []byte("$INSUNITS")) {
		t.Errorf("unexpected content in %q", got)
	}
	if !bytes.HasPrefix(got, []byte(dxf.BinarySentinel)) {
		t.Errorf("no sentinel: %q", got[:min(len(got), 24)])
	}
}

func TestText(t *testing.T) {
	cases := map[any]string{
		12000.0: "12000",
		1e-7:    "0.0000001",
		-0.25:   "-0.25",
		42:      "42",
		"s":     "s",
	}
	for in, want := range cases {
		if got := text(in); got != want {
			t.Errorf("text(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestNew(t *testing.T) {
	if got := string(
		New(-1).Bytes(),
	); strings.Contains(got, "$INSUNITS") ||
		!strings.Contains(got, "$ACADVER") {
		t.Errorf("New(-1) header:\n%s", got)
	}
	if got := string(New(0).Bytes()); !strings.Contains(got, "$INSUNITS\r\n 70\r\n0\r\n") {
		t.Errorf("New(0) header:\n%s", got)
	}
}

func TestHelpers(t *testing.T) {
	e := E("CIRCLE", 40, 1.0)
	if e.Kind != "CIRCLE" || !reflect.DeepEqual(e.Pairs, []any{40, 1.0}) {
		t.Errorf("E = %+v", e)
	}
	r := Rect("W", 0, 1, 2, 3, 43, 5.0)
	want := E("LWPOLYLINE", 8, "W", 90, 4, 70, 1, 43, 5.0,
		10, 0.0, 20, 1.0, 10, 2.0, 20, 1.0, 10, 2.0, 20, 3.0, 10, 0.0, 20, 3.0)
	if !reflect.DeepEqual(r, want) {
		t.Errorf("Rect = %+v, want %+v", r, want)
	}
	l := Line("L", 1, 2, 3, 4)
	if want := E("LINE", 8, "L", 10, 1.0, 20, 2.0, 11, 3.0, 21, 4.0); !reflect.DeepEqual(l, want) {
		t.Errorf("Line = %+v, want %+v", l, want)
	}
}

func TestBuilderAccumulates(t *testing.T) {
	d := New(6).Layer("A", 1, 0).Layer("B", 2, 0).
		Block("X", 0, 0).Block("Y", 0, 0).
		Add(Line("A", 0, 0, 1, 0)).Add(Line("B", 0, 0, 0, 1), E("CIRCLE"))
	got, err := dxf.Read(bytes.NewReader(d.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Layers) != 3 || len(got.Blocks) != 2 || len(got.Model) != 3 ||
		got.Units != cad.Meters {
		t.Errorf("read back %+v", got)
	}
}

func TestPlanRoundTrips(t *testing.T) {
	p := Plan()
	a, err := dxf.Read(bytes.NewReader(p.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	b, err := dxf.Read(bytes.NewReader(p.Binary()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Error("Plan reads differently as ASCII and binary")
	}
	// The outer walls are the plan's advertised size.
	walls, _ := a.Model[0].(*cad.Polyline)
	if walls == nil || walls.Vertices[2].P != (cad.Point{X: PlanWidth, Y: PlanDepth}) {
		t.Errorf("outer walls %+v", a.Model[0])
	}
	if len(p.layers) != 6 || len(p.blocks) != 3 || len(p.entities) != 26 {
		t.Errorf(
			"Plan has %d layers, %d blocks, %d entities",
			len(p.layers),
			len(p.blocks),
			len(p.entities),
		)
	}
}
