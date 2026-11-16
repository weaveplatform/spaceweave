package dwg

import (
	"bytes"
	"errors"
	"math/rand/v2"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/dwg/dwgtest"
)

// lzSamples are inputs chosen to reach every opcode the encoder writes:
// short and long literal runs, overlapping matches (runs of one byte),
// matches near (0x40–0xFF), mid (0x20–0x3F) and far (0x10–0x1F, with and
// without offset bit 14), and literal counts of 0–3 and more after them.
func lzSamples() map[string][]byte {
	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // deterministic test data
	random := func(n int) []byte {
		b := make([]byte, n)
		for i := range b {
			b[i] = byte(rng.IntN(256))
		}
		return b
	}
	block := random(300)
	far := random(0xC100)
	copy(far[0xA000:], far[0x5000:0x5000+300]) // offset 0x5000: bit 14 of offset-0x4000 clear
	copy(far[0xB000:], far[0x100:0x100+300])   // offset 0xAF00: bit 14 set
	var mixed []byte
	for i := range 200 {
		mixed = append(mixed, block[:3+i%20]...)
		mixed = append(mixed, random(i%7)...)
	}
	return map[string][]byte{
		"empty":    {},
		"one":      {7},
		"three":    {1, 2, 3},
		"four":     {1, 2, 3, 4},
		"eighteen": random(18),
		"nineteen": random(19),
		"long":     random(1000),
		"zeros":    make([]byte, 5000),
		"text":     bytes.Repeat([]byte("the quick brown fox "), 400),
		"mixed":    mixed,
		"far":      far,
		"abab":     []byte("abababababababababababab"),
		"short":    append([]byte("ab"), bytes.Repeat([]byte("xyz"), 3)...),
	}
}

func TestDecompressRoundTrip(t *testing.T) {
	for name, data := range lzSamples() {
		for enc, compress := range map[string]func([]byte) []byte{
			"lz": dwgtest.Compress, "literal": dwgtest.CompressLiteral,
		} {
			src := compress(data)
			dst := make([]byte, len(data))
			n, err := decompressR2004(dst, src)
			if err != nil || n != len(data) || !bytes.Equal(dst, data) {
				t.Errorf("%s/%s: n=%d err=%v, want %d bytes back", name, enc, n, err, len(data))
			}
			if len(data) > 0 {
				short := make([]byte, len(data)-1)
				if _, err := decompressR2004(short, src); !errors.Is(err, errLZOverflow) {
					t.Errorf("%s/%s into a short buffer: err=%v, want overflow", name, enc, err)
				}
			}
		}
	}
}

func TestDecompressMalformed(t *testing.T) {
	abcd := []byte{0x01, 'a', 'b', 'c', 'd'} // a four-byte literal run
	with := func(tail ...byte) []byte { return append(bytes.Clone(abcd), tail...) }
	long := make([]byte, 0x0F+0xFF+1+3) // the count 0x00 0x00 0x01 encodes
	tests := []struct {
		name string
		src  []byte
		want error // nil: decodes, to wantOut
		out  string
	}{
		{"empty", nil, errLZTruncated, ""},
		{"terminator only", []byte{0x11}, nil, ""},
		{"long literal count cut", []byte{0x00, 0x00}, errLZTruncated, ""},
		{"long literal count", append([]byte{0x00, 0x00, 0x01}, long...), nil, string(long)},
		{"literal run cut", []byte{0x01, 'a'}, errLZTruncated, ""},
		{"leading literal cut", []byte{0x13, 'a'}, errLZTruncated, ""},
		{"leading literal at end", []byte{0x13, 'a', 'b'}, nil, "ab"},
		{"leading then run", []byte{0x12, 'z', 0x01, 'a', 'b', 'c', 'd', 0x11}, nil, "zabcd"},
		{"leading then bad run", []byte{0x12, 'z', 0x00}, errLZTruncated, ""},
		{"run without terminator", with(), nil, "abcd"},
		{"reserved opcode", with(0x05), errLZOpcode, ""},
		{"match first", []byte{0x10, 0x01, 0x00, 0x00}, errLZOffset, ""},
		{"near offset too far", with(0x40, 0x01), errLZOffset, ""},
		{"near cut", with(0x40), errLZTruncated, ""},
		{"near match then end", with(0x4C, 0x00), nil, "abcdabc"},
		{"near match, two literals", with(0x4E, 0x00, 'x', 'y', 0x11), nil, "abcdabcxy"},
		{"near match, literals cut", with(0x4E, 0x00, 'x'), errLZTruncated, ""},
		{"near match, literals then end", with(0x4D, 0x00, 'x'), nil, "abcdabcx"},
		{"near match, run count cut", with(0x4C, 0x00, 0x00), errLZTruncated, ""},
		{"near match, run", with(0x4C, 0x00, 0x01, 'w', 'x', 'y', 'z', 0x11), nil, "abcdabcwxyz"},
		{"mid cut", with(0x21, 0x0C), errLZTruncated, ""},
		{"mid long count cut", with(0x20), errLZTruncated, ""},
		{"mid overlapping", with(0x21, 0x00, 0x00, 0x11), nil, "abcdddd"},
		{"far long count cut", with(0x10), errLZTruncated, ""},
		{"far cut", with(0x12, 0x00), errLZTruncated, ""},
		{"far offset", with(0x12, 0x00, 0x00), errLZOffset, ""},
	}
	for _, tt := range tests {
		dst := make([]byte, 1024)
		n, err := decompressR2004(dst, tt.src)
		switch {
		case tt.want != nil && !errors.Is(err, tt.want):
			t.Errorf("%s: err=%v, want %v", tt.name, err, tt.want)
		case tt.want != nil && !errors.Is(err, ErrCorrupt):
			t.Errorf("%s: %v doesn't wrap ErrCorrupt", tt.name, err)
		case tt.want == nil && (err != nil || string(dst[:n]) != tt.out):
			t.Errorf("%s: got %q, %v, want %q", tt.name, dst[:n], err, tt.out)
		}
	}
	// Output is bounded by the buffer for literals and matches alike.
	if _, err := decompressR2004(make([]byte, 3), abcd); !errors.Is(err, errLZOverflow) {
		t.Errorf("literal overflow: %v", err)
	}
	if _, err := decompressR2004(make([]byte, 1), []byte{0x12, 'a'}); err != nil {
		t.Errorf("leading literal into its own size: %v", err)
	}
	if _, err := decompressR2004(nil, []byte{0x12, 'a'}); !errors.Is(err, errLZOverflow) {
		t.Errorf("leading literal overflow: %v", err)
	}
	if _, err := decompressR2004(nil, []byte{0x00, 0x01}); !errors.Is(err, errLZTruncated) {
		t.Errorf("long literal count past the input: %v", err)
	}
}

func FuzzDecompress(f *testing.F) {
	for _, data := range lzSamples() {
		if len(data) <= 4096 {
			f.Add(dwgtest.Compress(data), uint16(len(data)))
			f.Add(dwgtest.CompressLiteral(data), uint16(len(data)))
		}
	}
	f.Fuzz(func(t *testing.T, src []byte, size uint16) {
		dst := make([]byte, size)
		n, err := decompressR2004(dst, src)
		if n < 0 || n > len(dst) {
			t.Fatalf("wrote %d bytes into %d", n, len(dst))
		}
		if err != nil && !errors.Is(err, ErrCorrupt) {
			t.Fatalf("error %v doesn't wrap ErrCorrupt", err)
		}
		// Whatever the input, the test encoders must round-trip it.
		for _, compress := range []func([]byte) []byte{dwgtest.Compress, dwgtest.CompressLiteral} {
			back := make([]byte, len(src))
			n, err := decompressR2004(back, compress(src))
			if err != nil || n != len(src) || !bytes.Equal(back, src) {
				t.Fatalf("round trip of %d bytes: n=%d err=%v", len(src), n, err)
			}
		}
	})
}
