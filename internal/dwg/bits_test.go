package dwg

import (
	"errors"
	"math"
	"testing"

	"golang.org/x/text/encoding/charmap"

	"github.com/weaveplatform/spaceweave/internal/dwg/dwgtest"
)

// TestBitsRoundTrip writes every bit code in each of its forms and reads
// them back.
func TestBitsRoundTrip(t *testing.T) {
	w := &dwgtest.Bits{}
	shorts := []int16{0, 1, 255, 256, -1, 1000}
	longs := []int32{0, 1, 255, 256, -5}
	doubles := []float64{0, 1, 2.5, math.Copysign(0, -1)}
	mcs := []uint64{0, 0x7f, 0x80, 1 << 40}
	smcs := []int64{0, 63, 64, -64, -1 << 30}
	mss := []uint32{0, 0x7fff, 0x8000, 1 << 25}
	ots := []int{0x13, 0x1f2, 0x2f0, 0x1000}
	dds := [][2]float64{{1, 1}, {1.0000001, 1}, {3, 2}, {-7e300, 1}}
	for _, v := range shorts {
		w.BS(v)
	}
	for _, v := range longs {
		w.BL(v)
	}
	for _, v := range doubles {
		w.BD(v)
	}
	for _, v := range mcs {
		w.MC(v)
	}
	for _, v := range smcs {
		w.MCs(v)
	}
	for _, v := range mss {
		w.MS(v)
	}
	for _, v := range ots {
		w.OT(v)
	}
	for _, v := range dds {
		w.DD(v[0], v[1])
	}
	w.BLL(0x0102030405)
	w.BT(0)
	w.BT(2)
	w.BE(0, 0, 1)
	w.BE(0, 0, -1)
	w.T("Straße")
	w.T("")
	w.TU("Ω room")
	w.TU("")
	w.H(5, 0x1234)
	w.H(6, 0)
	w.H(8, 0)
	w.H(0xA, 3)
	w.H(0xC, 3)

	r := newBits(w.Bytes())
	for _, v := range shorts {
		if g := r.BS(); g != v {
			t.Errorf("BS %d, want %d", g, v)
		}
	}
	for _, v := range longs {
		if g := r.BL(); g != v {
			t.Errorf("BL %d, want %d", g, v)
		}
	}
	for _, v := range doubles {
		if g := r.BD(); g != v {
			t.Errorf("BD %v, want %v", g, v)
		}
	}
	for _, v := range mcs {
		if g := r.MC(); g != v {
			t.Errorf("MC %d, want %d", g, v)
		}
	}
	for _, v := range smcs {
		if g := r.MCs(); g != v {
			t.Errorf("MCs %d, want %d", g, v)
		}
	}
	for _, v := range mss {
		if g := r.MS(); g != v {
			t.Errorf("MS %d, want %d", g, v)
		}
	}
	for _, v := range ots {
		if g := r.OT(); g != v {
			t.Errorf("OT %#x, want %#x", g, v)
		}
	}
	for _, v := range dds {
		if g := r.DD(v[1]); g != v[0] {
			t.Errorf("DD %v, want %v", g, v[0])
		}
	}
	if g := r.BLL(); g != 0x0102030405 {
		t.Errorf("BLL %#x", g)
	}
	if a, b := r.BT(), r.BT(); a != 0 || b != 2 {
		t.Errorf("BT %v %v", a, b)
	}
	if a, b := r.BE(), r.BE(); a.Z != 1 || b.Z != -1 {
		t.Errorf("BE %v %v", a, b)
	}
	if a, b, c, d := r.T(), r.T(), r.TU(), r.TU(); a != "Straße" || b != "" || c != "Ω room" ||
		d != "" {
		t.Errorf("strings %q %q %q %q", a, b, c, d)
	}
	for _, want := range []uint64{0x1234, 101, 99, 103, 97} {
		if g := r.handle(100); g != want {
			t.Errorf("handle %#x, want %#x", g, want)
		}
	}
	if r.err != nil {
		t.Fatal(r.err)
	}
	if r.left() >= 8 {
		t.Errorf("%d bits left", r.left())
	}
}

// TestBitsErrors checks each way a read can fail: it reports ErrCorrupt,
// doesn't panic, and later reads return zero.
func TestBitsErrors(t *testing.T) {
	ff := func(n int) *bitReader {
		b := make([]byte, n)
		for i := range b {
			b[i] = 0xff
		}
		return newBits(b)
	}
	cases := map[string]func() *bitReader{
		"bits past the end": func() *bitReader { r := newBits([]byte{1}); r.bits(9); return r },
		"skip past the end": func() *bitReader { r := newBits([]byte{1}); r.skip(9); return r },
		"skip back":         func() *bitReader { r := newBits([]byte{1}); r.skip(-1); return r },
		"raw past the end":  func() *bitReader { r := newBits([]byte{1}); r.raw(2); return r },
		"bitlong code 3":    func() *bitReader { r := ff(1); r.BL(); return r },
		"bitdouble code 3":  func() *bitReader { r := ff(1); r.BD(); return r },
		"long MC":           func() *bitReader { r := ff(12); r.MC(); return r },
		"long MCs":          func() *bitReader { r := ff(12); r.MCs(); return r },
		"long MS":           func() *bitReader { r := ff(6); r.MS(); return r },
		"long handle":       func() *bitReader { r := newBits([]byte{0x59, 0}); r.handle(0); return r },
		"handle code":       func() *bitReader { r := newBits([]byte{0x70}); r.handle(0); return r },
		"short T":           func() *bitReader { w := &dwgtest.Bits{}; w.BS(5); r := newBits(w.Bytes()); r.T(); return r },
		"short TU":          func() *bitReader { w := &dwgtest.Bits{}; w.BS(5); r := newBits(w.Bytes()); r.TU(); return r },
		"negative count":    func() *bitReader { r := newBits([]byte{1}); r.count(-1, 1); return r },
		"large count":       func() *bitReader { r := newBits([]byte{1}); r.count(9, 1); return r },
		"sub out of range":  func() *bitReader { return newBits([]byte{1}).sub(0, 9) },
	}
	for name, f := range cases {
		r := f()
		if !errors.Is(r.err, ErrCorrupt) {
			t.Errorf("%s: err %v", name, r.err)
		}
		if r.RC() != 0 || r.count(0, 1) != 0 || r.raw(0) != nil {
			t.Errorf("%s: reads after an error aren't zero", name)
		}
		r.skip(1)
	}
}

func TestBitsUnaligned(t *testing.T) {
	w := &dwgtest.Bits{}
	w.B(true)
	w.RC('A')
	w.RC('B')
	r := newBits(w.Bytes())
	r.B()
	if got := string(r.raw(2)); got != "AB" {
		t.Errorf("raw %q", got)
	}
}

func TestDecode8(t *testing.T) {
	if got := decode8([]byte{'c', 'a', 'f', 0xe9, 0}, nil); got != "café" {
		t.Errorf("default %q", got)
	}
	if got := decode8([]byte{0xc4}, codepage(29)); got != "Д" {
		t.Errorf("1251 %q", got)
	}
	// Shift-JIS with a lead byte and nothing after it can't decode.
	if got := decode8([]byte{0x82}, codepage(22)); got == "" {
		t.Errorf("bad Shift-JIS gave nothing")
	}
	if codepage(999) != charmap.Windows1252 {
		t.Error("an unknown code page isn't Windows-1252")
	}
}
