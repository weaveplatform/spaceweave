// Package dxftest writes DXF files for tests, so no third-party drawings
// need to live in the repository.
package dxftest

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
)

// Ent is one entity: its type and its group code / value pairs.
type Ent struct {
	Kind  string
	Pairs []any
}

// E builds an entity from alternating group codes and values.
func E(kind string, pairs ...any) Ent { return Ent{Kind: kind, Pairs: pairs} }

// Doc is a DXF file being built.
type Doc struct {
	header, layers []Ent
	blocks         []block
	entities       []Ent
}

type block struct {
	name string
	base [2]float64
	ents []Ent
}

// New starts a drawing in the given $INSUNITS (4 is millimetres; -1 leaves
// the variable out).
func New(units int) *Doc {
	d := &Doc{}
	d.header = append(d.header, E("$ACADVER", 1, "AC1027"))
	if units >= 0 {
		d.header = append(d.header, E("$INSUNITS", 70, units))
	}
	return d
}

// Layer adds a layer. A negative colour turns it off; flags 1 freezes it.
func (d *Doc) Layer(name string, color, flags int) *Doc {
	d.layers = append(d.layers, E("LAYER", 2, name, 70, flags, 62, color))
	return d
}

// Block adds a block definition.
func (d *Doc) Block(name string, x, y float64, ents ...Ent) *Doc {
	d.blocks = append(d.blocks, block{name, [2]float64{x, y}, ents})
	return d
}

// Add adds entities to model space (or paper space, with group 67 = 1).
func (d *Doc) Add(ents ...Ent) *Doc {
	d.entities = append(d.entities, ents...)
	return d
}

type sink interface {
	pair(code int, v any)
}

func (d *Doc) write(s sink) {
	ent := func(e Ent) {
		s.pair(0, e.Kind)
		for i := 0; i+1 < len(e.Pairs); i += 2 {
			s.pair(e.Pairs[i].(int), e.Pairs[i+1]) //nolint:forcetypeassert // test fixture
		}
	}
	s.pair(999, "spaceweave dxftest")
	s.pair(0, "SECTION")
	s.pair(2, "HEADER")
	for _, h := range d.header {
		s.pair(9, h.Kind)
		s.pair(h.Pairs[0].(int), h.Pairs[1]) //nolint:forcetypeassert // test fixture
	}
	s.pair(0, "ENDSEC")
	s.pair(0, "SECTION")
	s.pair(2, "TABLES")
	s.pair(0, "TABLE")
	s.pair(2, "LAYER")
	for _, l := range d.layers {
		ent(l)
	}
	s.pair(0, "ENDTAB")
	s.pair(0, "ENDSEC")
	s.pair(0, "SECTION")
	s.pair(2, "BLOCKS")
	for _, b := range d.blocks {
		s.pair(0, "BLOCK")
		s.pair(8, "0")
		s.pair(2, b.name)
		s.pair(10, b.base[0])
		s.pair(20, b.base[1])
		for _, e := range b.ents {
			ent(e)
		}
		s.pair(0, "ENDBLK")
	}
	s.pair(0, "ENDSEC")
	s.pair(0, "SECTION")
	s.pair(2, "ENTITIES")
	for _, e := range d.entities {
		ent(e)
	}
	s.pair(0, "ENDSEC")
	s.pair(0, "EOF")
}

type asciiSink struct{ b *bytes.Buffer }

func (a asciiSink) pair(code int, v any) {
	fmt.Fprintf(a.b, "%3d\r\n%s\r\n", code, text(v))
}

func text(v any) string {
	switch v := v.(type) {
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return fmt.Sprint(v)
	}
}

// Bytes is the drawing as ASCII DXF.
func (d *Doc) Bytes() []byte {
	var b bytes.Buffer
	d.write(asciiSink{&b})
	return b.Bytes()
}

type binarySink struct{ b *bytes.Buffer }

func (s binarySink) pair(code int, v any) {
	if code == 999 {
		return // binary DXF has no comments
	}
	_ = binary.Write(s.b, binary.LittleEndian, uint16(code)) //nolint:gosec // group codes fit
	// These ranges must match kind() in internal/dxf/reader.go; dxftest
	// can't import dxf (dxf's own tests import dxftest), and a mismatch
	// shows up as a binary fixture that reads as truncated.
	switch {
	case code >= 10 && code <= 59, code >= 110 && code <= 149, code >= 210 && code <= 239,
		code >= 460 && code <= 469, code >= 1010 && code <= 1059:
		f, _ := strconv.ParseFloat(text(v), 64)
		_ = binary.Write(s.b, binary.LittleEndian, f)
	case code >= 60 && code <= 79, code >= 170 && code <= 179, code >= 270 && code <= 289,
		code >= 370 && code <= 389, code >= 400 && code <= 409, code >= 1060 && code <= 1070:
		n, _ := strconv.Atoi(text(v))
		_ = binary.Write(s.b, binary.LittleEndian, int16(n)) //nolint:gosec // fixture values fit
	case code >= 90 && code <= 99,
		code >= 420 && code <= 429,
		code >= 440 && code <= 459,
		code == 1071:
		n, _ := strconv.Atoi(text(v))
		_ = binary.Write(s.b, binary.LittleEndian, int32(n)) //nolint:gosec // fixture values fit
	case code >= 160 && code <= 169:
		n, _ := strconv.ParseInt(text(v), 10, 64)
		_ = binary.Write(s.b, binary.LittleEndian, n)
	case code >= 290 && code <= 299:
		n, _ := strconv.Atoi(text(v))
		s.b.WriteByte(byte(n)) //nolint:gosec // a fixture's bool is 0 or 1
	case code >= 310 && code <= 319, code == 1004:
		b := []byte(text(v))
		s.b.WriteByte(byte(len(b))) //nolint:gosec // fixture chunks are under 256 bytes
		s.b.Write(b)
	default:
		s.b.WriteString(text(v))
		s.b.WriteByte(0)
	}
}

// Binary is the drawing as binary DXF.
func (d *Doc) Binary() []byte {
	var b bytes.Buffer
	b.WriteString("AutoCAD Binary DXF\r\n\x1a\x00")
	d.write(binarySink{&b})
	return b.Bytes()
}

// Rect is a closed LWPOLYLINE rectangle.
func Rect(layer string, x0, y0, x1, y1 float64, extra ...any) Ent {
	p := make([]any, 0, 6+len(extra)+16)
	p = append(p, 8, layer, 90, 4, 70, 1)
	p = append(p, extra...)
	for _, c := range [][2]float64{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}} {
		p = append(p, 10, c[0], 20, c[1])
	}
	return E("LWPOLYLINE", p...)
}

// Line is a LINE.
func Line(layer string, x0, y0, x1, y1 float64) Ent {
	return E("LINE", 8, layer, 10, x0, 20, y0, 11, x1, 21, y1)
}

// PlanWidth and PlanDepth are the outer wall centrelines of Plan, in mm.
const (
	PlanWidth = 12000.0
	PlanDepth = 8000.0
)

// Plan is a small office floor plan in millimetres, using every entity type
// the reader draws: walls with width, doors as mirrored and rotated block
// inserts, a desk array, hatches, text, a dimension, a spline and an
// ellipse, plus things that must not show (an off layer, a frozen layer,
// paper space) and a stray line far away.
func Plan() *Doc {
	door := []Ent{
		Line("0", 0, 0, 0, 900),
		E("ARC", 8, "0", 10, 0.0, 20, 0.0, 40, 900.0, 50, 0.0, 51, 90.0),
	}
	desk := []Ent{
		Rect("0", 0, 0, 1600, 800),
		E("CIRCLE", 8, "0", 62, 0, 10, 800.0, 20, 1150.0, 40, 250.0),
	}
	dim := []Ent{
		Line("0", 0, -600, 12000, -600),
		E(
			"SOLID",
			8,
			"0",
			10,
			0.0,
			20,
			-600.0,
			11,
			150.0,
			21,
			-650.0,
			12,
			150.0,
			22,
			-550.0,
			13,
			150.0,
			23,
			-550.0,
		),
		E("MTEXT", 8, "0", 10, 6000.0, 20, -500.0, 40, 180.0, 71, 8, 1, "12 000"),
	}
	d := New(4).
		Layer("WALLS", 7, 0).Layer("DOORS", 2, 0).Layer("TEXT", 7, 0).Layer("FURN", 3, 0).
		Layer("HIDDEN", -1, 0).Layer("FROZEN", 7, 1).
		Block("DOOR", 0, 0, door...).Block("DESK", 0, 0, desk...).Block("*D1", 0, 0, dim...)
	d.Add(
		Rect("WALLS", 0, 0, PlanWidth, PlanDepth, 43, 200.0),
		Line("WALLS", 6000, 0, 6000, 3000),
		Line("WALLS", 6000, 4000, 6000, PlanDepth),
		E(
			"LWPOLYLINE",
			8,
			"WALLS",
			90,
			3,
			70,
			0,
			10,
			6000.0,
			20,
			3000.0,
			42,
			0.4142,
			10,
			7000.0,
			20,
			4000.0,
			10,
			8000.0,
			20,
			4000.0,
		),
		E(
			"HATCH",
			8,
			"WALLS",
			62,
			8,
			10,
			0.0,
			20,
			0.0,
			2,
			"SOLID",
			70,
			1,
			91,
			1,
			92,
			2,
			72,
			0,
			73,
			1,
			93,
			4,
			10,
			5800.0,
			20,
			5800.0,
			10,
			6200.0,
			20,
			5800.0,
			10,
			6200.0,
			20,
			6200.0,
			10,
			5800.0,
			20,
			6200.0,
			97,
			0,
		),
		E("HATCH", 8, "FURN", 10, 0.0, 20, 0.0, 2, "ANSI31", 70, 0, 91, 1,
			92, 1, 93, 4,
			72, 1, 10, 9000.0, 20, 6000.0, 11, 11000.0, 21, 6000.0,
			72, 2, 10, 11000.0, 20, 6500.0, 40, 500.0, 50, 270.0, 51, 90.0, 73, 1,
			72, 1, 10, 11000.0, 20, 7000.0, 11, 9000.0, 21, 7000.0,
			72, 3, 10, 9000.0, 20, 6500.0, 11, 0.0, 21, 500.0, 40, 1.0, 50, 0.0, 51, 180.0, 73, 1,
			97, 0),
		E("INSERT", 8, "DOORS", 2, "DOOR", 10, 3000.0, 20, 0.0),
		E("INSERT", 8, "DOORS", 2, "DOOR", 10, -8000.0, 20, 0.0, 230, -1.0),
		E("INSERT", 8, "DOORS", 2, "DOOR", 10, 6000.0, 20, 3000.0, 50, 90.0, 41, 1.0, 42, 1.0),
		E(
			"INSERT",
			8,
			"FURN",
			62,
			3,
			2,
			"DESK",
			10,
			800.0,
			20,
			4500.0,
			70,
			3,
			71,
			2,
			44,
			1800.0,
			45,
			1500.0,
		),
		E(
			"MTEXT",
			8,
			"TEXT",
			10,
			3000.0,
			20,
			2000.0,
			40,
			250.0,
			71,
			5,
			3,
			"{\\fArial|b1;Open office}",
			1,
			"\\PDesks 6",
		),
		E(
			"TEXT",
			8,
			"TEXT",
			10,
			9000.0,
			20,
			2000.0,
			11,
			9000.0,
			21,
			2000.0,
			40,
			250.0,
			72,
			1,
			73,
			2,
			1,
			"Meeting%%d",
		),
		E("DIMENSION", 8, "TEXT", 2, "*D1"),
		E(
			"ELLIPSE",
			8,
			"FURN",
			10,
			9000.0,
			20,
			3000.0,
			11,
			600.0,
			21,
			0.0,
			40,
			0.5,
			41,
			0.0,
			42,
			2*math.Pi,
		),
		E("SPLINE", 8, "WALLS", 70, 0, 71, 3, 72, 8, 73, 4,
			40, 0.0, 40, 0.0, 40, 0.0, 40, 0.0, 40, 1.0, 40, 1.0, 40, 1.0, 40, 1.0,
			10, 0.0, 20, 8000.0, 10, -800.0, 20, 6000.0, 10, -800.0, 20, 2000.0, 10, 0.0, 20, 0.0),
		E("POLYLINE", 8, "FURN", 66, 1, 70, 1),
		E("VERTEX", 8, "FURN", 10, 10500.0, 20, 1000.0),
		E("VERTEX", 8, "FURN", 10, 11500.0, 20, 1000.0),
		E("VERTEX", 8, "FURN", 10, 11500.0, 20, 2000.0),
		E("SEQEND", 8, "FURN"),
		Line("HIDDEN", 0, 0, PlanWidth, PlanDepth),
		Line("FROZEN", 0, PlanDepth, PlanWidth, 0),
		E("TEXT", 8, "TEXT", 67, 1, 10, 0.0, 20, 0.0, 40, 500.0, 1, "Title block"),
		E("VIEWPORT", 8, "0", 67, 1),
		E("LEADER", 8, "TEXT"),
		Line("WALLS", 5e6, 5e6, 5e6+100, 5e6),
	)
	return d
}
