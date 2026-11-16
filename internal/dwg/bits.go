package dwg

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"unicode/utf16"

	"golang.org/x/text/encoding"
)

// errShort is what every read past the end of a stream reports. A
// stream's error is sticky: once set, reads return zero values and leave it
// alone, so a decoder can read a whole object and check once at the end
// rather than after every field.
var errShort = fmt.Errorf("%w: data ends early", ErrCorrupt)

// bitReader reads DWG's bit-coded values, most significant bit first,
// between two bit positions of a byte slice. The bounds are what keep a
// corrupt size or offset from reading into a neighbouring object, or off
// the end of the file: nothing here panics on any input.
//
// The bit codes follow chapter 2 of the ODA specification; ACadSharp's
// DwgStreamReaderBase (MIT) was a cross-check.
type bitReader struct {
	b   []byte
	pos int // bit position
	end int // bit limit, exclusive
	err error
	// enc decodes pre-R2007 8-bit strings; nil means Windows-1252.
	enc encoding.Encoding
}

func newBits(b []byte) *bitReader { return &bitReader{b: b, end: len(b) * 8} }

// sub returns a reader for bits [from, to) of the same data, clamped to
// this reader's data.
func (r *bitReader) sub(from, to int) *bitReader {
	n := &bitReader{b: r.b, pos: from, end: to, enc: r.enc}
	if from < 0 || to < from || to > len(r.b)*8 {
		n.err = errShort
		n.pos, n.end = 0, 0
	}
	return n
}

func (r *bitReader) fail(err error) {
	if r.err == nil {
		r.err = err
	}
}

// left is how many bits remain.
func (r *bitReader) left() int { return r.end - r.pos }

// bits reads n ≤ 64 bits as an unsigned number.
func (r *bitReader) bits(n int) uint64 {
	if r.err != nil {
		return 0
	}
	if n > r.left() {
		r.fail(errShort)
		r.pos = r.end
		return 0
	}
	var v uint64
	for n > 0 {
		off := r.pos & 7
		k := min(8-off, n)
		b := uint64(r.b[r.pos>>3]) >> (8 - off - k) & (1<<k - 1)
		v = v<<k | b
		n -= k
		r.pos += k
	}
	return v
}

func (r *bitReader) skip(n int) {
	if r.err != nil {
		return
	}
	if n < 0 || n > r.left() {
		r.fail(errShort)
		r.pos = r.end
		return
	}
	r.pos += n
}

// B is a bit.
func (r *bitReader) B() bool { return r.bits(1) == 1 }

// BB is a 2-bit code.
func (r *bitReader) BB() int { return int(r.bits(2) & 3) }

// RC is a raw byte.
func (r *bitReader) RC() byte { return byte(r.bits(8) & 0xff) }

// raw reads n raw bytes, which needn't be byte-aligned.
func (r *bitReader) raw(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || n*8 > r.left() {
		r.fail(errShort)
		r.pos = r.end
		return nil
	}
	out := make([]byte, n)
	if r.pos&7 == 0 {
		copy(out, r.b[r.pos>>3:])
		r.pos += n * 8
		return out
	}
	for i := range out {
		out[i] = r.RC()
	}
	return out
}

// RS is a raw little-endian short.
func (r *bitReader) RS() uint16 {
	lo := r.bits(8)
	return uint16((r.bits(8)<<8 | lo) & 0xffff)
}

// RL is a raw little-endian long.
func (r *bitReader) RL() uint32 {
	var v uint32
	for i := range 4 {
		v |= uint32(r.bits(8)&0xff) << (8 * i)
	}
	return v
}

// RD is a raw little-endian double.
func (r *bitReader) RD() float64 {
	var b [8]byte
	for i := range b {
		b[i] = r.RC()
	}
	return math.Float64frombits(binary.LittleEndian.Uint64(b[:]))
}

// BS is a bitshort.
func (r *bitReader) BS() int16 { return int16(r.bsBits()) } //nolint:gosec // reinterpreted

// bsBits is a bitshort's 16 bits, which ENC colours use as flags.
func (r *bitReader) bsBits() uint16 {
	switch r.BB() {
	case 0:
		return r.RS()
	case 1:
		return uint16(r.RC())
	case 2:
		return 0
	default:
		return 256
	}
}

// BL is a bitlong. The unused fourth code reads as 0 and marks the stream
// corrupt.
func (r *bitReader) BL() int32 { return int32(r.blBits()) } //nolint:gosec // reinterpreted

// blBits is a bitlong's 32 bits, which colours use as flags and bytes.
func (r *bitReader) blBits() uint32 {
	switch r.BB() {
	case 0:
		return r.RL()
	case 1:
		return uint32(r.RC())
	case 2:
		return 0
	default:
		r.fail(fmt.Errorf("%w: bad bitlong code", ErrCorrupt))
		return 0
	}
}

// BLL is a bitlonglong: a byte count, then that many bytes, low first.
// The specification calls the count a "3B" of one to three bits ending at
// the first 0; R2018 header variables only line up with a plain 3-bit
// count, as ACadSharp reads it.
func (r *bitReader) BLL() uint64 {
	n := int(r.bits(3) & 7)
	var v uint64
	for i := range n {
		v |= uint64(r.RC()) << (8 * i)
	}
	return v
}

// BD is a bitdouble.
func (r *bitReader) BD() float64 {
	switch r.BB() {
	case 0:
		return r.RD()
	case 1:
		return 1
	case 2:
		return 0
	default:
		r.fail(fmt.Errorf("%w: bad bitdouble code", ErrCorrupt))
		return 0
	}
}

// DD is a bitdouble with a default: the default with 0, 4 or 6 of its bytes
// patched, or a whole double.
func (r *bitReader) DD(def float64) float64 {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], math.Float64bits(def))
	switch r.BB() {
	case 0:
		return def
	case 1:
		for i := range 4 {
			b[i] = r.RC()
		}
	case 2:
		b[4], b[5] = r.RC(), r.RC()
		for i := range 4 {
			b[i] = r.RC()
		}
	default:
		return r.RD()
	}
	return math.Float64frombits(binary.LittleEndian.Uint64(b[:]))
}

// vec3 is a 3D point or vector.
type vec3 struct{ X, Y, Z float64 }

func (r *bitReader) BD3() vec3 { return vec3{r.BD(), r.BD(), r.BD()} }
func (r *bitReader) RD2() vec3 { return vec3{X: r.RD(), Y: r.RD()} }

// BT is a thickness: R2000+ stores a set bit for 0.
func (r *bitReader) BT() float64 {
	if r.B() {
		return 0
	}
	return r.BD()
}

// BE is an extrusion: R2000+ stores a set bit for (0, 0, 1).
func (r *bitReader) BE() vec3 {
	if r.B() {
		return vec3{Z: 1}
	}
	return r.BD3()
}

// MC is an unsigned modular char: 7 bits a byte, low first, while the high
// bit is set. Ten bytes hold any uint64; more is corruption.
func (r *bitReader) MC() uint64 {
	var v uint64
	for i := 0; i < 10; i++ {
		b := r.RC()
		v |= uint64(b&0x7f) << (7 * i)
		if b&0x80 == 0 || r.err != nil {
			return v
		}
	}
	r.fail(fmt.Errorf("%w: modular char too long", ErrCorrupt))
	return 0
}

// MCs is a signed modular char: the last byte's 0x40 bit is the sign.
func (r *bitReader) MCs() int64 {
	var v int64
	for i := 0; i < 10; i++ {
		b := r.RC()
		if b&0x80 == 0 || r.err != nil {
			v |= int64(b&0x3f) << (7 * i)
			if b&0x40 != 0 {
				v = -v
			}
			return v
		}
		v |= int64(b&0x7f) << (7 * i)
	}
	r.fail(fmt.Errorf("%w: modular char too long", ErrCorrupt))
	return 0
}

// MS is a modular short: 15 bits a little-endian short, low first, while
// the short's high bit is set. Object sizes are the only use, and two
// shorts already hold a gigabyte.
func (r *bitReader) MS() uint32 {
	var v uint32
	for i := range 2 {
		w := r.RS()
		v |= uint32(w&0x7fff) << (15 * i)
		if w&0x8000 == 0 {
			return v
		}
	}
	r.fail(fmt.Errorf("%w: modular short too long", ErrCorrupt))
	return 0
}

// handle reads a handle reference, resolving the relative codes against
// base (the referring object's own handle).
func (r *bitReader) handle(base uint64) uint64 {
	form := r.RC()
	code, n := form>>4, int(form&0x0f)
	if n > 8 {
		r.fail(fmt.Errorf("%w: handle longer than 8 bytes", ErrCorrupt))
		return 0
	}
	var v uint64
	for range n {
		v = v<<8 | uint64(r.RC())
	}
	switch code {
	case 0, 2, 3, 4, 5:
		return v
	case 6:
		return base + 1
	case 8:
		return base - 1
	case 0xA:
		return base + v
	case 0xC:
		return base - v
	default:
		r.fail(fmt.Errorf("%w: handle code %#x", ErrCorrupt, code))
		return 0
	}
}

// OT is an R2010+ object type: a 2-bit code and one or two bytes.
func (r *bitReader) OT() int {
	switch r.BB() {
	case 0:
		return int(r.RC())
	case 1:
		return 0x1f0 + int(r.RC())
	default:
		return int(r.RS())
	}
}

// T is a pre-R2007 string: a bitshort length and that many bytes in the
// drawing's code page.
func (r *bitReader) T() string {
	n := int(r.BS())
	if n <= 0 {
		return ""
	}
	if n*8 > r.left() {
		r.fail(errShort)
		return ""
	}
	return decode8(r.raw(n), r.enc)
}

// TU is an R2007+ string: a bitshort count of UTF-16 code units, little
// endian.
func (r *bitReader) TU() string {
	n := int(r.BS())
	if n <= 0 {
		return ""
	}
	if n*16 > r.left() {
		r.fail(errShort)
		return ""
	}
	u := make([]uint16, n)
	for i := range u {
		u[i] = r.RS()
	}
	for len(u) > 0 && u[len(u)-1] == 0 {
		u = u[:len(u)-1]
	}
	return string(utf16.Decode(u))
}

// decode8 turns 8-bit text into UTF-8, dropping the NUL terminator some
// writers include in the length.
func decode8(b []byte, enc encoding.Encoding) string {
	for len(b) > 0 && b[len(b)-1] == 0 {
		b = b[:len(b)-1]
	}
	if isASCII(b) {
		return string(b)
	}
	if enc == nil {
		enc = defaultCodepage
	}
	s, err := enc.NewDecoder().Bytes(b)
	if err != nil {
		return strings.ToValidUTF8(string(b), "\uFFFD")
	}
	return strings.ToValidUTF8(string(s), "\uFFFD")
}

func isASCII(b []byte) bool {
	for _, c := range b {
		if c >= 0x80 {
			return false
		}
	}
	return true
}

// count reads a non-negative count of items that each take at least
// minBits, and refuses one the remaining data couldn't hold, so a corrupt
// count can't make a decoder allocate or loop without end.
func (r *bitReader) count(n int32, minBits int) int {
	if r.err != nil {
		return 0
	}
	if n < 0 || int(n)*max(minBits, 1) > r.left() {
		r.fail(fmt.Errorf("%w: count %d too large", ErrCorrupt, n))
		return 0
	}
	return int(n)
}
