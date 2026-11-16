package dwgtest

// The object-data writer mirrors internal/dwg's reader from the ODA "Open
// Design Specification for .dwg files" and, like the container writer,
// shares no code with it.

import (
	"encoding/binary"
	"math"
	"unicode/utf16"

	"golang.org/x/text/encoding/charmap"
)

// Bits writes DWG's bit-coded values, most significant bit first. Tests
// use it to hand-build streams the file writer has no reason to produce.
type Bits struct {
	b []byte
	n int // bits written
}

// Len is the number of bits written.
func (w *Bits) Len() int { return w.n }

// Bytes returns the bits written, the last byte padded with zeros.
func (w *Bits) Bytes() []byte { return w.b }

// Put writes the low n bits of v.
func (w *Bits) Put(v uint64, n int) {
	for i := n - 1; i >= 0; i-- {
		if w.n&7 == 0 {
			w.b = append(w.b, 0)
		}
		if v>>uint(i)&1 != 0 { //nolint:gosec // i >= 0
			w.b[w.n>>3] |= 0x80 >> uint(w.n&7) //nolint:gosec // w.n >= 0
		}
		w.n++
	}
}

// Append writes all of o's bits.
func (w *Bits) Append(o *Bits) {
	for i := range o.n {
		w.Put(uint64(o.b[i>>3]>>(7-uint(i&7))&1), 1) //nolint:gosec // i >= 0
	}
}

// Pad writes zeros to the next byte boundary.
func (w *Bits) Pad() { w.Put(0, (8-w.n&7)&7) }

// B writes a bit.
func (w *Bits) B(v bool) {
	if v {
		w.Put(1, 1)
	} else {
		w.Put(0, 1)
	}
}

// BB writes a 2-bit code.
func (w *Bits) BB(v int) { w.Put(uint64(v&3), 2) } //nolint:gosec // masked

// RC writes a raw byte.
func (w *Bits) RC(v byte) { w.Put(uint64(v), 8) }

// RS writes a raw little-endian short.
func (w *Bits) RS(v uint16) {
	w.RC(byte(v & 0xff))
	w.RC(byte(v >> 8))
}

// RL writes a raw little-endian long.
func (w *Bits) RL(v uint32) {
	for i := range 4 {
		w.RC(byte(v >> (8 * i) & 0xff))
	}
}

// RD writes a raw little-endian double.
func (w *Bits) RD(v float64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], math.Float64bits(v))
	for _, c := range b {
		w.RC(c)
	}
}

// RD2 writes two raw doubles.
func (w *Bits) RD2(x, y float64) {
	w.RD(x)
	w.RD(y)
}

// BS writes a bitshort in its shortest form.
func (w *Bits) BS(v int16) {
	switch {
	case v == 0:
		w.BB(2)
	case v == 256:
		w.BB(3)
	case v > 0 && v < 256:
		w.BB(1)
		w.RC(byte(v))
	default:
		w.BB(0)
		w.RS(uint16(v)) //nolint:gosec // the bits of v
	}
}

// BL writes a bitlong in its shortest form.
func (w *Bits) BL(v int32) {
	switch {
	case v == 0:
		w.BB(2)
	case v > 0 && v < 256:
		w.BB(1)
		w.RC(byte(v))
	default:
		w.BB(0)
		w.RL(uint32(v)) //nolint:gosec // the bits of v
	}
}

// BLL writes a bitlonglong: a 3-bit byte count, then the bytes low first.
func (w *Bits) BLL(v uint64) {
	n := 0
	for x := v; x != 0; x >>= 8 {
		n++
	}
	w.Put(u64(n), 3)
	for i := range n {
		w.RC(byte(v >> (8 * i) & 0xff))
	}
}

// BD writes a bitdouble in its shortest form.
func (w *Bits) BD(v float64) {
	switch {
	case v == 1:
		w.BB(1)
	case v == 0 && !math.Signbit(v):
		w.BB(2)
	default:
		w.BB(0)
		w.RD(v)
	}
}

// BD3 writes three bitdoubles.
func (w *Bits) BD3(x, y, z float64) {
	w.BD(x)
	w.BD(y)
	w.BD(z)
}

// DD writes v as a bitdouble with default def, patching as few of the
// default's bytes as will do.
func (w *Bits) DD(v, def float64) {
	var b, d [8]byte
	binary.LittleEndian.PutUint64(b[:], math.Float64bits(v))
	binary.LittleEndian.PutUint64(d[:], math.Float64bits(def))
	switch {
	case b == d:
		w.BB(0)
	case b[4] == d[4] && b[5] == d[5] && b[6] == d[6] && b[7] == d[7]:
		w.BB(1)
		for _, c := range b[:4] {
			w.RC(c)
		}
	case b[6] == d[6] && b[7] == d[7]:
		w.BB(2)
		w.RC(b[4])
		w.RC(b[5])
		for _, c := range b[:4] {
			w.RC(c)
		}
	default:
		w.BB(3)
		w.RD(v)
	}
}

// BT writes a thickness: a set bit for 0.
func (w *Bits) BT(v float64) {
	w.B(v == 0)
	if v != 0 {
		w.BD(v)
	}
}

// BE writes an extrusion: a set bit for (0, 0, 1).
func (w *Bits) BE(x, y, z float64) {
	up := x == 0 && y == 0 && z == 1
	w.B(up)
	if !up {
		w.BD3(x, y, z)
	}
}

// MC writes an unsigned modular char.
func (w *Bits) MC(v uint64) {
	for v >= 0x80 {
		w.RC(byte(v&0x7f) | 0x80)
		v >>= 7
	}
	w.RC(byte(v))
}

// MCs writes a signed modular char: 7 bits a byte, and 6 with the sign in
// the last.
func (w *Bits) MCs(v int64) {
	neg := v < 0
	if neg {
		v = -v
	}
	for v >= 0x40 {
		w.RC(byte(v&0x7f) | 0x80)
		v >>= 7
	}
	last := byte(v & 0x3f)
	if neg {
		last |= 0x40
	}
	w.RC(last)
}

// MS writes a modular short.
func (w *Bits) MS(v uint32) {
	for v >= 0x8000 {
		w.RS(uint16(v&0x7fff) | 0x8000)
		v >>= 15
	}
	w.RS(uint16(v & 0x7fff))
}

// H writes a handle reference: the code in the high nibble of the first
// byte, the value's length in the low, then the value high byte first.
func (w *Bits) H(code byte, v uint64) {
	var b []byte
	for x := v; x != 0; x >>= 8 {
		b = append([]byte{byte(x)}, b...)
	}
	w.RC(code<<4 | byte(len(b)&0xf))
	for _, c := range b {
		w.RC(c)
	}
}

// OT writes an R2010+ object type.
func (w *Bits) OT(v int) {
	switch {
	case v < 0x100:
		w.BB(0)
		w.RC(byte(v & 0xff))
	case v >= 0x1f0 && v < 0x2f0:
		w.BB(1)
		w.RC(byte(v - 0x1f0))
	default:
		w.BB(2)
		w.RS(u16(v))
	}
}

// T writes a pre-R2007 string in Windows-1252, the code page the writer
// declares.
func (w *Bits) T(s string) {
	b, err := charmap.Windows1252.NewEncoder().Bytes([]byte(s))
	if err != nil {
		b = []byte(s)
	}
	w.BS(i16(len(b)))
	for _, c := range b {
		w.RC(c)
	}
}

// TU writes an R2007+ string: a count of UTF-16 code units, then the units.
func (w *Bits) TU(s string) {
	u := utf16.Encode([]rune(s))
	w.BS(i16(len(u)))
	for _, c := range u {
		w.RS(c)
	}
}

// Conversions of values the writer knows are small. Masking, rather than
// a bare conversion, says so to the linter.
func i16(v int) int16  { return int16(v & 0x7fff) }
func u16(v int) uint16 { return uint16(v & 0xffff) }
func n32(v int) int32  { return int32(v & 0x7fffffff) }
