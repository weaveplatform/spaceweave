package dxf

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"strings"
	"testing"
	"testing/iotest"
)

var errBoom = errors.New("boom")

func TestKind(t *testing.T) {
	// The ends of every range: an off-by-one here shifts every later pair of
	// a binary file and garbles the rest of it.
	cases := map[int]valueKind{
		0: kindString, 9: kindString,
		10: kindFloat, 59: kindFloat,
		60: kindInt16, 79: kindInt16,
		80: kindString, 89: kindString,
		90: kindInt32, 99: kindInt32,
		100: kindString, 109: kindString,
		110: kindFloat, 149: kindFloat,
		150: kindString,
		160: kindInt64, 169: kindInt64,
		170: kindInt16, 179: kindInt16,
		180: kindString,
		210: kindFloat, 239: kindFloat,
		240: kindString,
		270: kindInt16, 289: kindInt16,
		290: kindBool, 299: kindBool,
		300: kindString, 309: kindString,
		310: kindChunk, 319: kindChunk,
		320: kindString, 369: kindString,
		370: kindInt16, 389: kindInt16,
		390: kindString,
		400: kindInt16, 409: kindInt16,
		410: kindString, 419: kindString,
		420: kindInt32, 429: kindInt32,
		430: kindString, 439: kindString,
		440: kindInt32, 459: kindInt32,
		460: kindFloat, 469: kindFloat,
		470: kindString, 999: kindString,
		1000: kindString, 1003: kindString,
		1004: kindChunk,
		1005: kindString,
		1010: kindFloat, 1059: kindFloat,
		1060: kindInt16, 1070: kindInt16,
		1071: kindInt32,
		1072: kindString,
	}
	for code, want := range cases {
		if got := kind(code); got != want {
			t.Errorf("kind(%d) = %d, want %d", code, got, want)
		}
	}
}

func TestPairNumbers(t *testing.T) {
	floats := map[string]float64{
		"1.5":   1.5,
		"  -2 ": -2,
		"1e3":   1000,
		"abc":   0,
		"":      0,
		// NaN and infinity would poison every bound and transform downstream.
		"NaN":   0,
		"+Inf":  0,
		"-inf":  0,
		"1e999": 0,
	}
	for s, want := range floats {
		if got := (pair{val: s}).float(); got != want {
			t.Errorf("float(%q) = %v, want %v", s, got, want)
		}
	}
	ints := map[string]int{
		"7":     7,
		" -3  ": -3,
		"2.9":   2, // some writers put a float where an integer belongs
		"x":     0,
	}
	for s, want := range ints {
		if got := (pair{val: s}).int(); got != want {
			t.Errorf("int(%q) = %d, want %d", s, got, want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short"); got != "short" {
		t.Errorf("truncate(short) = %q", got)
	}
	if got := truncate(strings.Repeat("x", 30)); got != strings.Repeat("x", 20)+"…" {
		t.Errorf("truncate(30 x) = %q", got)
	}
}

// pairs drains a source.
func pairs(t *testing.T, data []byte) ([]pair, error) {
	t.Helper()
	src, err := newSource(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("newSource: %v", err)
	}
	var out []pair
	for {
		p, err := src.next()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		out = append(out, p)
	}
}

func TestASCIISource(t *testing.T) {
	// A BOM, padded codes, CRLF and LF mixed, and a value with spaces kept.
	in := "\xef\xbb\xbf  0\r\nSECTION\r\n  2\nHEADER\n999\n a comment \n"
	got, err := pairs(t, []byte(in))
	if err != nil {
		t.Fatal(err)
	}
	want := []pair{{0, "SECTION"}, {2, "HEADER"}, {999, " a comment "}}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("pair %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestASCIISourceErrors(t *testing.T) {
	_, err := pairs(t, []byte("0\nSECTION\n12x\nfoo\n"))
	if !errors.Is(err, ErrNotDXF) || !strings.Contains(err.Error(), "line 3") {
		t.Errorf("bad code: %v, want ErrNotDXF naming line 3", err)
	}
	_, err = pairs(t, []byte("0\nSECTION\n2"))
	if !errors.Is(err, ErrTruncated) {
		t.Errorf("code without value: %v, want ErrTruncated", err)
	}
}

func TestASCIILineTooLong(t *testing.T) {
	long := strings.Repeat("7", maxLine+1)
	// On the code line and on the value line the scanner's error must come
	// through, not be mistaken for the end of the file.
	for name, in := range map[string]string{
		"code":  long + "\nx\n",
		"value": "0\n" + long + "\n",
	} {
		_, err := pairs(t, []byte(in))
		if !errors.Is(err, bufio.ErrTooLong) {
			t.Errorf("%s: %v, want bufio.ErrTooLong", name, err)
		}
	}
}

func TestASCIIReadError(t *testing.T) {
	_, err := Read(iotest.ErrReader(errBoom))
	if !errors.Is(err, errBoom) {
		t.Errorf("Read = %v, want the reader's error", err)
	}
}

// bin encodes binary DXF pairs; values must already have the Go type of
// the code's kind.
func bin(t *testing.T, kv ...any) []byte {
	t.Helper()
	b := bytes.NewBufferString(BinarySentinel)
	for i := 0; i+1 < len(kv); i += 2 {
		code, ok := kv[i].(int)
		if !ok {
			t.Fatalf("code %v is not an int", kv[i])
		}
		le(t, b, uint16(code)) //nolint:gosec // test codes fit
		switch v := kv[i+1].(type) {
		case string:
			b.WriteString(v)
			b.WriteByte(0)
		case []byte: // a chunk: length then bytes
			b.WriteByte(byte(len(v)))
			b.Write(v)
		default:
			le(t, b, v)
		}
	}
	return b.Bytes()
}

func le(t *testing.T, b *bytes.Buffer, v any) {
	t.Helper()
	if err := binary.Write(b, binary.LittleEndian, v); err != nil {
		t.Fatal(err)
	}
}

func TestBinarySourceKinds(t *testing.T) {
	data := bin(t,
		0, "SECTION",
		10, 1.25,
		62, int16(-3),
		90, int32(-70000),
		160, int64(1)<<40,
		290, uint8(1),
		310, []byte{1, 2, 3},
		1, "after the chunk",
	)
	got, err := pairs(t, data)
	if err != nil {
		t.Fatal(err)
	}
	want := []pair{
		{0, "SECTION"},
		{10, "1.25"},
		{62, "-3"},
		{90, "-70000"},
		{160, "1099511627776"},
		{290, "1"},
		{310, ""},
		{1, "after the chunk"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("pair %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestBinarySourceTruncated(t *testing.T) {
	cases := map[string][]byte{
		"half a code":  {0x00},
		"float":        {10, 0, 1, 2, 3},
		"int16":        {62, 0, 1},
		"int32":        {90, 0, 1, 2},
		"int64":        {160, 0, 1, 2, 3, 4},
		"bool":         {34, 1},          // 290
		"chunk length": {54, 1},          // 310
		"chunk body":   {54, 1, 9, 1, 2}, // 310, nine bytes promised
		"string":       {1, 0, 'a', 'b'},
	}
	for name, body := range cases {
		_, err := pairs(t, append([]byte(BinarySentinel), body...))
		if !errors.Is(err, ErrTruncated) {
			t.Errorf("%s: %v, want ErrTruncated", name, err)
		}
	}
}

func TestBinaryReadError(t *testing.T) {
	// A failing reader is reported as a truncated file: the binary reader
	// cannot tell the two apart and either way the rest is gone.
	r := io.MultiReader(
		bytes.NewReader(bin(t, 0, "SECTION")),
		bytes.NewReader([]byte{10}),
		iotest.ErrReader(errBoom),
	)
	_, err := Read(r)
	if !errors.Is(err, ErrTruncated) {
		t.Errorf("Read = %v, want ErrTruncated", err)
	}
}

func TestBinaryFloatExact(t *testing.T) {
	// Formatting with the shortest round-tripping form keeps every bit.
	for _, f := range []float64{0.1, 1.0 / 3, math.MaxFloat64, -math.SmallestNonzeroFloat64} {
		got, err := pairs(t, bin(t, 10, f))
		if err != nil {
			t.Fatal(err)
		}
		if got[0].float() != f {
			t.Errorf("%v came back as %v", f, got[0].float())
		}
	}
}
