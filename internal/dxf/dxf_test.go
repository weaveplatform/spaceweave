package dxf

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// readBoth reads a drawing as ASCII and as binary DXF and insists both give
// the same document, so every builder-made case also checks the binary path.
func readBoth(t *testing.T, d *dxftest.Doc) *cad.Document {
	t.Helper()
	a, err := Read(bytes.NewReader(d.Bytes()))
	if err != nil {
		t.Fatalf("ASCII: %v", err)
	}
	b, err := Read(bytes.NewReader(d.Binary()))
	if err != nil {
		t.Fatalf("binary: %v", err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("ASCII and binary differ:\n%+v\n%+v", a, b)
	}
	return a
}

// model reads entities placed in model space of an otherwise empty drawing.
func model(t *testing.T, ents ...dxftest.Ent) []cad.Entity {
	t.Helper()
	return readBoth(t, dxftest.New(4).Add(ents...)).Model
}

// one reads a single entity and asserts its type.
func one[T cad.Entity](t *testing.T, ents ...dxftest.Ent) T {
	t.Helper()
	m := model(t, ents...)
	if len(m) != 1 {
		t.Fatalf("got %d entities, want 1: %+v", len(m), m)
	}
	e, ok := m[0].(T)
	if !ok {
		t.Fatalf("got %T, want %T", m[0], e)
	}
	return e
}

// raw writes pairs as LF-separated ASCII DXF, for files the builder can't
// express (stray pairs, other table types, broken structure).
func raw(pairs ...any) []byte {
	var b bytes.Buffer
	for i := 0; i+1 < len(pairs); i += 2 {
		fmt.Fprintf(&b, "%v\n%v\n", pairs[i], pairs[i+1])
	}
	return b.Bytes()
}

func readRaw(t *testing.T, pairs ...any) *cad.Document {
	t.Helper()
	d, err := Read(bytes.NewReader(raw(pairs...)))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	return d
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func nearPt(a, b cad.Point) bool { return near(a.X, b.X) && near(a.Y, b.Y) }

func TestPlanASCIIMatchesBinary(t *testing.T) {
	d := readBoth(t, dxftest.Plan())
	if d.Units != cad.Millimeters || d.Version != "AC1027" {
		t.Errorf("units %v, version %q", d.Units, d.Version)
	}
	// 18 drawn model entities plus the stray line; the paper-space TEXT is
	// drawn, the paper-space VIEWPORT is not.
	if len(d.Model) != 19 || len(d.Paper) != 1 {
		t.Errorf("model %d, paper %d; want 19, 1", len(d.Model), len(d.Paper))
	}
	if len(d.Blocks) != 3 || len(d.Layers) != 7 {
		t.Errorf("blocks %d, layers %d; want 3, 7", len(d.Blocks), len(d.Layers))
	}
	if !reflect.DeepEqual(d.Notes, []string{"Not drawn: 1 LEADER."}) {
		t.Errorf("notes %q", d.Notes)
	}
	kinds := map[string]int{}
	for _, e := range d.Model {
		kinds[fmt.Sprintf("%T", e)]++
	}
	want := map[string]int{
		"*cad.Polyline": 3, "*cad.Line": 5, "*cad.Hatch": 2, "*cad.Insert": 4,
		"*cad.MText": 1, "*cad.Text": 1, "*cad.Dimension": 1, "*cad.Ellipse": 1, "*cad.Spline": 1,
	}
	if !reflect.DeepEqual(kinds, want) {
		t.Errorf("entity kinds %v, want %v", kinds, want)
	}
	if b := d.Blocks["DOOR"]; b == nil || len(b.Entities) != 2 {
		t.Errorf("DOOR block %+v", b)
	}
	if b := d.Blocks["*D1"]; b == nil || len(b.Entities) != 3 {
		t.Errorf("*D1 block %+v", b)
	}
	if txt, ok := d.Paper[0].(*cad.Text); !ok || txt.Content != "Title block" {
		t.Errorf("paper %+v", d.Paper[0])
	}
}

func TestPlanLayers(t *testing.T) {
	d := readBoth(t, dxftest.Plan())
	want := map[string]cad.Layer{
		"0":      {Name: "0", Color: 7},
		"WALLS":  {Name: "WALLS", Color: 7},
		"DOORS":  {Name: "DOORS", Color: 2},
		"HIDDEN": {Name: "HIDDEN", Color: 1, Off: true},
		"FROZEN": {Name: "FROZEN", Color: 7, Frozen: true},
	}
	for name, w := range want {
		l := d.Layers[name]
		if l == nil || *l != w {
			t.Errorf("layer %s = %+v, want %+v", name, l, w)
		}
	}
}

func TestLayerTable(t *testing.T) {
	d := readRaw(t,
		0, "SECTION", 2, "TABLES",
		// Other tables' entries are not layers and must not become one.
		0, "TABLE", 2, "LTYPE", 0, "LTYPE", 2, "DASHED", 70, 0, 0, "ENDTAB",
		0, "TABLE", 2, "LAYER",
		0, "LAYER", 2, "RGB", 62, 5, 420, 0x12ab34, 70, 4,
		0, "LAYER", 2, "PLAIN",
		0, "LAYER", 70, 0, // no name
		0, "ENDTAB",
		0, "ENDSEC", 0, "EOF")
	if len(d.Layers) != 3 {
		t.Errorf("layers %v, want 0, RGB, PLAIN", d.Layers)
	}
	rgb := d.Layers["RGB"]
	if rgb == nil || rgb.Color != 5 || rgb.Frozen || rgb.Off ||
		rgb.RGB == nil || *rgb.RGB != (vector.Color{R: 0x12, G: 0xab, B: 0x34}) {
		t.Errorf("RGB layer %+v", rgb)
	}
	// A layer with no colour group is white, AutoCAD's default.
	if p := d.Layers["PLAIN"]; p == nil || p.Color != 7 || p.RGB != nil {
		t.Errorf("PLAIN layer %+v", p)
	}
}

func TestHeaderUnits(t *testing.T) {
	for _, u := range []cad.Units{
		cad.Unitless, cad.Inches, cad.Feet, cad.Millimeters,
		cad.Centimeters, cad.Meters, cad.Decimeters, 21,
	} {
		d := readBoth(t, dxftest.New(int(u)))
		if d.Units != u {
			t.Errorf("$INSUNITS %d read as %d", u, d.Units)
		}
	}
	if d := readBoth(t, dxftest.New(-1)); d.Units != cad.Unitless {
		t.Errorf("no $INSUNITS read as %v", d.Units)
	}
}

func TestHeaderIgnoresOtherVariables(t *testing.T) {
	d := readRaw(t,
		0, "SECTION", 2, "HEADER",
		9, "$EXTMIN", 10, 1.0, 20, 2.0,
		// A stray non-variable pair is skipped, not taken as a variable name.
		5, "$INSUNITS", 70, 6,
		9, "$INSUNITS", 70, 5,
		9, "$ACADVER", 1, " AC1032 ",
		0, "ENDSEC", 0, "EOF")
	if d.Units != cad.Centimeters || d.Version != "AC1032" {
		t.Errorf("units %v, version %q", d.Units, d.Version)
	}
}

// legacyText is a drawing whose layer name and text hold a raw 0xB0 byte,
// the degree sign in Windows-1252.
func legacyText(version, s string) []byte {
	return raw(
		0, "SECTION", 2, "HEADER", 9, "$ACADVER", 1, version, 0, "ENDSEC",
		0, "SECTION", 2, "TABLES", 0, "TABLE", 2, "LAYER",
		0, "LAYER", 2, "L"+s, 62, 1, 0, "ENDTAB", 0, "ENDSEC",
		0, "SECTION", 2, "ENTITIES",
		0, "TEXT", 8, "L"+s, 1, "20"+s+"C",
		0, "MTEXT", 8, "L"+s, 1, "m"+s,
		0, "INSERT", 8, "L"+s, 2, "B"+s,
		0, "DIMENSION", 2, "D"+s,
		0, "ENDSEC", 0, "EOF")
}

func TestLegacyCodePage(t *testing.T) {
	cases := []struct {
		version, in, want string
	}{
		{"AC1015", "\xb0", "°"},
		{"AC1018", "\xb0", "°"},
		// From AutoCAD 2007 on text is UTF-8; a stray byte is replaced,
		// not guessed at.
		{"AC1021", "\xb0", "�"},
		{"AC1027", "\xb0", "�"},
		// Text that is already valid UTF-8 is left alone even in an old file.
		{"AC1015", "°", "°"},
	}
	for _, c := range cases {
		d, err := Read(bytes.NewReader(legacyText(c.version, c.in)))
		if err != nil {
			t.Fatalf("%s: %v", c.version, err)
		}
		layer := "L" + c.want
		if d.Layers[layer] == nil {
			t.Errorf("%s %q: layers %v, want %q", c.version, c.in, d.Layers, layer)
		}
		if len(d.Model) != 4 {
			t.Fatalf("%s: model %+v", c.version, d.Model)
		}
		txt, _ := d.Model[0].(*cad.Text)
		mt, _ := d.Model[1].(*cad.MText)
		in, _ := d.Model[2].(*cad.Insert)
		dim, _ := d.Model[3].(*cad.Dimension)
		if txt == nil || txt.Content != "20"+c.want+"C" || txt.Layer != layer ||
			mt == nil || mt.Content != "m"+c.want ||
			in == nil || in.Block != "B"+c.want ||
			dim == nil || dim.Block != "D"+c.want {
			t.Errorf("%s %q: got %+v %+v %+v %+v", c.version, c.in, txt, mt, in, dim)
		}
	}
}

func TestNoVersionIsUTF8(t *testing.T) {
	d := readRaw(t, 0, "SECTION", 2, "ENTITIES", 0, "TEXT", 1, "a\xb0", 0, "ENDSEC")
	if txt, _ := d.Model[0].(*cad.Text); txt == nil || txt.Content != "a�" {
		t.Errorf("text %+v", d.Model[0])
	}
}

func TestBlocks(t *testing.T) {
	d := readRaw(t,
		0, "SECTION", 2, "BLOCKS",
		0, "BLOCK", 8, "0", 2, "CHAIR", 10, 5.0, 20, -2.0,
		0, "LINE", 10, 0, 20, 0, 11, 1, 21, 1,
		0, "CIRCLE", 40, 2,
		0, "ENDBLK", 8, "0", 5, "1F",
		// A block without a name has nothing to be inserted by.
		0, "BLOCK", 8, "0", 0, "LINE", 0, "ENDBLK",
		0, "ENDSEC", 0, "EOF")
	if len(d.Blocks) != 1 {
		t.Fatalf("blocks %v", d.Blocks)
	}
	b := d.Blocks["CHAIR"]
	if b == nil || b.Name != "CHAIR" || b.Base != (cad.Point{X: 5, Y: -2}) || len(b.Entities) != 2 {
		t.Errorf("CHAIR %+v", b)
	}
}

func TestEntitiesSectionsAppend(t *testing.T) {
	// Two ENTITIES sections, the second with a stray pair before its first
	// entity: both contribute, in order.
	d := readRaw(t,
		0, "SECTION", 2, "ENTITIES", 0, "LINE", 8, "A", 0, "ENDSEC",
		0, "SECTION", 2, "ENTITIES", 5, "FF", 0, "LINE", 8, "B", 0, "TEXT", 67, 1, 0, "ENDSEC")
	if len(d.Model) != 2 || d.Model[0].Base().Layer != "A" || d.Model[1].Base().Layer != "B" ||
		len(d.Paper) != 1 {
		t.Errorf("model %+v paper %+v", d.Model, d.Paper)
	}
}

func TestUnknownSectionsAndComments(t *testing.T) {
	d := readRaw(t,
		999, "written by hand",
		0, "SECTION", 2, "CLASSES", 0, "CLASS", 1, "X", 0, "ENDSEC",
		0, "SECTION", 2, "OBJECTS", 0, "DICTIONARY", 0, "LINE", 0, "ENDSEC",
		999, "between sections",
		0, "SECTION", 2, "ENTITIES", 0, "LINE", 999, "inside an entity", 10, 3, 0, "ENDSEC",
		0, "EOF",
		// Anything after EOF is never read.
		"garbage", "garbage")
	if len(d.Model) != 1 {
		t.Fatalf("model %+v", d.Model)
	}
	if l, _ := d.Model[0].(*cad.Line); l == nil || l.A.X != 3 {
		t.Errorf("line %+v", d.Model[0])
	}
}

func TestUnknownEntitiesNoted(t *testing.T) {
	d := readBoth(t, dxftest.New(4).Add(
		dxftest.E("WIPEOUT"), dxftest.E("LEADER"), dxftest.E("WIPEOUT"),
		dxftest.E("3DFACE", 67, 1),
		// Types that carry nothing a plan shows are dropped without a note.
		dxftest.E("POINT"),
		dxftest.E("VIEWPORT"), dxftest.E("ATTDEF"), dxftest.E("XLINE"), dxftest.E("RAY"),
		dxftest.Line("0", 0, 0, 1, 1),
	),
	)
	if len(d.Model) != 1 || len(d.Paper) != 0 {
		t.Errorf("model %+v paper %+v", d.Model, d.Paper)
	}
	want := []string{"Not drawn: 1 3DFACE, 1 LEADER, 2 WIPEOUT."}
	if !reflect.DeepEqual(d.Notes, want) {
		t.Errorf("notes %q, want %q", d.Notes, want)
	}
	if d := readBoth(t, dxftest.New(4).Add(dxftest.E("POINT"))); d.Notes != nil {
		t.Errorf("notes %q, want none", d.Notes)
	}
}

func TestBOMAndLineEndings(t *testing.T) {
	plan := dxftest.Plan().Bytes()
	want, err := Read(bytes.NewReader(plan))
	if err != nil {
		t.Fatal(err)
	}
	variants := map[string][]byte{
		"BOM": append([]byte("\xef\xbb\xbf"), plan...),
		"LF":  bytes.ReplaceAll(plan, []byte("\r\n"), []byte("\n")),
	}
	for name, in := range variants {
		got, err := Read(bytes.NewReader(in))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: document differs", name)
		}
	}
}

func TestMissingEOFIsFine(t *testing.T) {
	d := readRaw(t, 0, "SECTION", 2, "ENTITIES", 0, "LINE", 0, "ENDSEC")
	if len(d.Model) != 1 {
		t.Errorf("model %+v", d.Model)
	}
}

func TestBadNumbersAreZero(t *testing.T) {
	// A garbled number loses one coordinate, not the drawing.
	d := readRaw(
		t,
		0,
		"SECTION",
		2,
		"ENTITIES",
		0,
		"LINE",
		10,
		"abc",
		20,
		"2",
		11,
		"1,5",
		21,
		"",
		0,
		"ENDSEC",
	)
	if l, _ := d.Model[0].(*cad.Line); l == nil || l.A != (cad.Point{Y: 2}) ||
		l.B != (cad.Point{}) {
		t.Errorf("line %+v", d.Model[0])
	}
}

func TestNotDXF(t *testing.T) {
	cases := map[string]string{
		"empty":            "",
		"text":             "hello world\n",
		"long text":        strings.Repeat("not a dxf ", 10) + "\n",
		"comment only":     "999\nnothing here\n",
		"pair before":      "1\nfoo\n0\nSECTION\n2\nENTITIES\n0\nENDSEC\n",
		"short sentinel":   BinarySentinel[:10],
		"bad code mid-way": "0\nSECTION\n2\nENTITIES\nLINE\n0\n",
	}
	for name, in := range cases {
		d, err := Read(strings.NewReader(in))
		if !errors.Is(err, ErrNotDXF) || d != nil {
			t.Errorf("%s: %v, %v; want ErrNotDXF", name, d, err)
		}
	}
	_, err := Read(strings.NewReader(strings.Repeat("x", 40)))
	if err == nil || !strings.Contains(err.Error(), strings.Repeat("x", 20)+"…") {
		t.Errorf("long first line: %v, want it shortened in the message", err)
	}
}

// TestEOFBeforeAnySection shows a two-line "0 / EOF" file read as an empty
// drawing, though file() promises that anything before the first section
// other than a comment means the input isn't DXF.
func TestEOFBeforeAnySection(t *testing.T) {
	d, err := Read(strings.NewReader("0\nEOF\n"))
	if !errors.Is(err, ErrNotDXF) {
		t.Errorf("Read = %+v, %v; want ErrNotDXF", d, err)
	}
}

func TestTruncatedStructures(t *testing.T) {
	sec := func(name string, rest ...any) []any { return append([]any{0, "SECTION", 2, name}, rest...) }
	cases := map[string][]any{
		"section name":     {0, "SECTION"},
		"header":           sec("HEADER", 9, "$ACADVER"),
		"header variable":  sec("HEADER", 9, "$ACADVER", 1, "AC1015"),
		"tables":           sec("TABLES", 0, "TABLE"),
		"layer":            sec("TABLES", 0, "LAYER", 2, "A"),
		"blocks":           sec("BLOCKS", 999, "x"),
		"block header":     sec("BLOCKS", 0, "BLOCK", 2, "A"),
		"block entities":   sec("BLOCKS", 0, "BLOCK", 2, "A", 0, "LINE", 0, "CIRCLE", 40, 1),
		"block no ENDBLK":  sec("BLOCKS", 0, "BLOCK", 2, "A", 0, "LINE", 8, "0", 0, "LINE"),
		"entities":         sec("ENTITIES", 5, "A"),
		"entity":           sec("ENTITIES", 0, "LINE", 10, 1),
		"polyline vertex":  sec("ENTITIES", 0, "POLYLINE", 0, "VERTEX", 10, 1),
		"insert attribute": sec("ENTITIES", 0, "INSERT", 66, 1, 0, "ATTRIB", 1, "x"),
		"unknown section":  sec("OBJECTS", 0, "DICTIONARY"),
		"in an entity":     sec("ENTITIES", 0, "LINE", 10),
	}
	for name, pairs := range cases {
		d, err := Read(bytes.NewReader(raw(pairs...)))
		if !errors.Is(err, ErrTruncated) || d != nil {
			t.Errorf("%s: %v, %v; want ErrTruncated", name, d, err)
		}
	}
}

// TestEveryTruncation cuts the sample plan at every line (ASCII) and every
// byte (binary). A cut file must fail cleanly or, when the cut falls between
// sections, read as what came before it.
func TestEveryTruncation(t *testing.T) {
	plan := dxftest.Plan()
	check := func(name string, in []byte) {
		d, err := Read(bytes.NewReader(in))
		switch {
		case err == nil && d == nil:
			t.Errorf("%s: no document and no error", name)
		case err != nil && d != nil:
			t.Errorf("%s: both a document and %v", name, err)
		case err != nil && !errors.Is(err, ErrTruncated) && !errors.Is(err, ErrNotDXF):
			t.Errorf("%s: unexpected error %v", name, err)
		}
	}
	ascii := plan.Bytes()
	for i := range len(ascii) {
		if i == 0 || ascii[i-1] == '\n' {
			check(fmt.Sprintf("ASCII cut at %d", i), ascii[:i])
		}
	}
	bin := plan.Binary()
	for i := range len(bin) {
		check(fmt.Sprintf("binary cut at %d", i), bin[:i])
	}
}

func TestBinaryCleanEndBetweenSections(t *testing.T) {
	// A binary file that stops after a complete section reads like an ASCII
	// file that lacks its closing EOF.
	data := bin(t, 0, "SECTION", 2, "HEADER", 9, "$INSUNITS", 70, int16(6), 0, "ENDSEC")
	d, err := Read(bytes.NewReader(data))
	if err != nil || d.Units != cad.Meters {
		t.Errorf("Read = %+v, %v", d, err)
	}
}
