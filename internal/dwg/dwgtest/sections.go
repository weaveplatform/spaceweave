package dwgtest

import (
	"cmp"
	"encoding/binary"
	"slices"
	"strings"
)

// objects appends every object to out and returns where each starts. They
// are written in the order they were made, not by handle, so the object
// map's offsets go back as well as forward, as AutoCAD's do once a drawing
// has been edited.
func (e *encoder) objects(out []byte) ([]byte, []mapEntry) {
	offsets := make([]mapEntry, 0, len(e.objs))
	for _, o := range e.objs {
		offsets = append(offsets, mapEntry{o.handle, int64(len(out))})
		out = append(out, e.encodeObject(o)...)
	}
	slices.SortFunc(offsets, func(a, b mapEntry) int { return cmp.Compare(a.handle, b.handle) })
	return out, offsets
}

// encodeObject frames an object: its size; from R2010 its handle stream's
// size; its type, handle and data; from R2007 its strings; its handles;
// and a CRC.
//
// Before R2010 a long after the type gives the size in bits of the type
// and data, which is where the handles start.
func (e *encoder) encodeObject(o *object) []byte {
	body := &Bits{}
	rlAt := -1
	if e.v >= v2010 {
		body.OT(o.typ)
	} else {
		body.BS(i16(o.typ))
		rlAt = body.Len()
		body.RL(0)
	}
	body.H(0, o.handle)
	body.Append(o.d)
	if e.v >= v2007 {
		stringStream(body, o.s)
	}
	handleStart := body.Len()
	if rlAt >= 0 {
		body.patchRL(rlAt, u32(handleStart))
	}
	body.Append(o.h)
	body.Pad()
	size := len(body.Bytes())
	out := &Bits{}
	out.MS(u32(size))
	if e.v >= v2010 {
		out.MC(u64(size*8 - handleStart))
	}
	b := append(out.Bytes(), body.Bytes()...)
	return binary.LittleEndian.AppendUint16(b, crc16(0xC0C1, b))
}

// stringStream appends an R2007+ string stream: the strings, their size in
// bits (a high short first when it needs more than 15 bits) and a bit
// saying whether there are any.
func stringStream(w, s *Bits) {
	n := s.Len()
	if n == 0 {
		w.B(false)
		return
	}
	w.Append(s)
	if n >= 0x8000 {
		w.RS(u16(n >> 15))
		w.RS(u16(n&0x7fff) | 0x8000)
	} else {
		w.RS(u16(n))
	}
	w.B(true)
}

// patchRL overwrites the raw long written at bit at.
func (w *Bits) patchRL(at int, v uint32) {
	for i := range 32 {
		byteBit := 7 - i%8
		bit := v >> (8*(i/8) + byteBit) & 1
		p := at + i
		mask := byte(0x80) >> uint(p&7) //nolint:gosec // p >= 0
		if bit != 0 {
			w.b[p>>3] |= mask
		} else {
			w.b[p>>3] &^= mask
		}
	}
}

// Section sentinels; each section ends with its sentinel's complement.
var (
	sentinelHeader = []byte{
		0xCF, 0x7B, 0x1F, 0x23, 0xFD, 0xDE, 0x38, 0xA9,
		0x5F, 0x7C, 0x68, 0xB8, 0x4E, 0x6D, 0x33, 0x5F,
	}
	sentinelClasses = []byte{
		0x8D, 0xA1, 0xC4, 0xB8, 0xC4, 0xA9, 0xF8, 0xC5,
		0xC0, 0xDC, 0xF4, 0x5F, 0xE7, 0xCF, 0xB6, 0x8A,
	}
)

// section wraps a section's data: sentinel, size (and, where the version
// has one, a high word), the data, a CRC and the closing sentinel.
func (e *encoder) section(sentinel, data []byte) []byte {
	out := append([]byte{}, sentinel...)
	out = binary.LittleEndian.AppendUint32(out, u32(len(data)))
	if e.v >= v2018 || (e.v >= v2010 && e.dr.Maintenance > 3) {
		out = binary.LittleEndian.AppendUint32(out, 0)
	}
	out = append(out, data...)
	out = binary.LittleEndian.AppendUint16(out, crc16(0xC0C1, out[16:]))
	for _, c := range sentinel {
		out = append(out, ^c)
	}
	return out
}

// streams writes data, strings and handles the R2007+ way, behind a long
// giving the size in bits of everything before the handles; before R2007
// all three are the one stream.
func (e *encoder) streams(d, s, h *Bits) []byte {
	if e.v < v2007 {
		d.Pad()
		return d.Bytes()
	}
	w := &Bits{}
	w.RL(0)
	w.Append(d)
	stringStream(w, s)
	w.patchRL(0, u32(w.Len()))
	w.Append(h)
	w.Pad()
	return w.Bytes()
}

// classSection writes the classes section.
func (e *encoder) classSection() []byte {
	d, s := &Bits{}, &Bits{}
	if e.v < v2007 {
		s = d
	}
	tv := func(v string) {
		if e.v >= v2007 {
			s.TU(v)
		} else {
			s.T(v)
		}
	}
	if e.v >= v2004 {
		d.BS(i16(classes[len(classes)-1].num))
		d.RC(0)
		d.RC(0)
		d.B(true)
	}
	for _, c := range classes {
		d.BS(i16(c.num))
		d.BS(0) // proxy flags
		tv("ObjectDBX Classes")
		tv("AcDb" + strings.ToLower(c.name))
		tv(c.name)
		d.B(false) // a zombie
		if c.entity {
			d.BS(0x1F2)
		} else {
			d.BS(0x1F3)
		}
		if e.v >= v2004 {
			d.BL(1) // instances
			d.BL(0)
			d.BL(0)
			d.BL(0)
			d.BL(0)
		}
	}
	return e.section(sentinelClasses, e.streams(d, s, &Bits{}))
}
