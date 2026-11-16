package dwg

import (
	"fmt"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// ver is a DWG version, ordered so comparisons read as "R2004 and later".
type ver int

const (
	v2000 ver = iota // AC1015
	v2004            // AC1018
	v2007            // AC1021; not read, but its changes carry forward
	v2010            // AC1024
	v2013            // AC1027
	v2018            // AC1032
	vMax  = v2018
)

var versions = map[string]ver{
	"AC1015": v2000, "AC1018": v2004, "AC1024": v2010, "AC1027": v2013, "AC1032": v2018,
}

// stringStream finds the R2007+ string stream of an object (or the header
// or classes section) whose data ends at bit endbit. The last data bit
// says whether there is one; if so a 16-bit size precedes it, with a high
// part before that when the size's top bit is set, and the strings precede
// the size. It returns nil when there are no strings, and a reader in error
// when the size points before lower, where the object starts.
//
// The specification says the size is 16 bytes before the flag; it is 16
// bits, as ACadSharp found.
func stringStream(r *bitReader, endbit, lower int) *bitReader {
	if !r.sub(endbit-1, endbit).B() {
		return nil
	}
	at := endbit - 1 - 16
	size := int(r.sub(at, at+16).RS())
	if size&0x8000 != 0 {
		at -= 16
		hi := int(r.sub(at, at+16).RS())
		size = size&0x7fff | hi<<15
	}
	if at-size < lower {
		return &bitReader{err: fmt.Errorf("%w: string stream before its object", ErrCorrupt)}
	}
	return r.sub(at-size, at)
}

// obj is one object being decoded: its data, string and handle streams.
type obj struct {
	d, s, h *bitReader // s is nil for an R2007+ object without strings
	ver     ver
	typ     int
	handle  uint64
}

// tv reads a string from wherever this version keeps them.
func (o *obj) tv() string {
	if o.ver < v2007 {
		return o.d.T()
	}
	if o.s == nil {
		return ""
	}
	return o.s.TU()
}

// ref reads a handle reference from the handle stream.
func (o *obj) ref() uint64 { return o.h.handle(o.handle) }

func (o *obj) err() error {
	for _, r := range []*bitReader{o.d, o.s, o.h} {
		if r != nil && r.err != nil {
			return r.err
		}
	}
	return nil
}

// refs reads n handles, n read from the data stream and checked against
// what the handle stream could hold.
func (o *obj) refs(n int32) []uint64 {
	k := o.h.count(n, 8)
	out := make([]uint64, 0, k)
	for range k {
		out = append(out, o.ref())
	}
	return out
}

// open frames the object at offset in the objects section: its size, type
// and the bounds of its streams. Each stream is limited to its own bits,
// so a field misread can't run on into the next stream or object.
func (d *decoder) open(offset int64) (*obj, error) {
	all := newBits(d.objs)
	all.enc = d.enc
	if offset < 0 || offset >= int64(len(d.objs)) {
		return nil, fmt.Errorf("%w: object offset %d outside the objects", ErrCorrupt, offset)
	}
	all.pos = int(offset) * 8
	size := int(all.MS())
	var mc uint64
	if d.ver >= v2010 {
		mc = all.MC()
	}
	if all.err != nil {
		return nil, all.err
	}
	handleBits := int(mc & 0x7fffffff)
	start, end := all.pos, all.pos+size*8
	if size <= 0 || end > all.end || mc > 0x7fffffff || handleBits > size*8 {
		return nil, fmt.Errorf("%w: object size %d", ErrCorrupt, size)
	}
	o := &obj{d: all.sub(start, end), ver: d.ver}
	var handles int
	if d.ver >= v2010 {
		o.typ = o.d.OT()
		handles = end - handleBits
	} else {
		o.typ = int(o.d.BS())
		handles = start + int(o.d.RL())
	}
	if handles < o.d.pos || handles > end {
		return nil, fmt.Errorf("%w: object handles outside the object", ErrCorrupt)
	}
	o.h = all.sub(handles, end)
	o.d.end = handles
	if d.ver >= v2007 {
		o.s = stringStream(all, handles, o.d.pos)
		o.d.end = handles - 1
		if o.s != nil {
			o.d.end = o.s.pos
		}
	}
	o.handle = o.d.handle(0)
	return o, o.err()
}

// eed skips extended entity data: runs of a size, an application handle
// and that many bytes, until a size of 0.
func (o *obj) eed() {
	for {
		n := int(o.d.BS())
		if n <= 0 || o.d.err != nil {
			return
		}
		o.d.handle(0)
		o.d.skip(n * 8)
	}
}

// common reads the start every non-entity object shares and returns its
// owner.
func (o *obj) common() uint64 {
	o.eed()
	owner := o.ref()
	o.reactors()
	return owner
}

// reactors skips the persistent reactors and extension dictionary.
func (o *obj) reactors() {
	o.refs(o.d.BL())
	if o.ver < v2004 || !o.d.B() {
		o.ref() // the extension dictionary
	}
	if o.ver >= v2013 {
		o.d.B() // has binary data in the data store
	}
}

// entCommon is what every entity has in common.
type entCommon struct {
	mode  int // 0: owned (see owner), 1: paper space, 2: model space
	owner uint64
	// next is the following entity in an R2000 owner's list.
	next      uint64
	layer     uint64
	color     int
	rgb       *vector.Color
	invisible bool
}

// Entity modes.
const (
	modeOwned = 0
	modePaper = 1
	modeModel = 2
)

// entity reads the common entity data (specification 20.4.1 and 20.4.2).
// Handles come out in the order they were stored: the data fields say
// which are present, so they are read together.
func (o *obj) entity() entCommon {
	o.eed()
	if o.d.B() {
		// A proxy graphic: a byte count and the bytes.
		var n uint64
		if o.ver >= v2010 {
			n = o.d.BLL()
		} else {
			n = uint64(o.d.RL())
		}
		if n > uint64(o.d.left()/8) { //nolint:gosec // left is never negative
			o.d.fail(errShort)
		} else {
			o.d.skip(int(n) * 8) //nolint:gosec // n fits: it is under left
		}
	}
	return o.entityMode()
}

// entityMode reads the common entity data from the entity mode on, which
// an R2018 multi-line attribute repeats for the MTEXT it embeds.
func (o *obj) entityMode() entCommon {
	var e entCommon
	e.mode = o.d.BB()
	if e.mode == modeOwned {
		e.owner = o.ref()
	}
	o.reactors()
	if o.ver == v2000 {
		// R2000 chains each owner's entities; "no links" means the
		// neighbours are the adjacent handles.
		if o.d.B() {
			e.next = o.handle + 1
		} else {
			o.ref()
			e.next = o.ref()
		}
	}
	if o.ver >= v2004 {
		e.color, e.rgb = o.enc()
	} else {
		e.color = int(o.d.BS())
	}
	o.d.BD() // linetype scale
	e.layer = o.ref()
	if o.d.BB() == 3 {
		o.ref() // linetype
	}
	if o.ver >= v2007 {
		if o.d.BB() == 3 {
			o.ref() // material
		}
		o.d.RC() // shadow flags
	}
	if o.d.BB() == 3 {
		o.ref() // plot style
	}
	if o.ver >= v2010 {
		for range 3 { // full, face and edge visual styles
			if o.d.B() {
				o.ref()
			}
		}
	}
	e.invisible = o.d.BS()&1 != 0
	o.d.RC() // lineweight
	return e
}

// ENC colour flags (specification 2.11).
const (
	encRGB          = 0x8000
	encBook         = 0x4000
	encTransparency = 0x2000
	encIndex        = 0x0fff
)

// enc reads an R2004+ entity colour: an index, or a true colour, perhaps
// with a colour-book reference in the handle stream and a transparency.
func (o *obj) enc() (int, *vector.Color) {
	v := o.d.bsBits()
	color, rgb := int(v&encIndex), (*vector.Color)(nil)
	if v&encRGB != 0 {
		color, rgb = trueColor(o.d.blBits())
	}
	if v&encBook != 0 {
		o.ref()
	}
	if v&encTransparency != 0 {
		o.d.BL()
	}
	return color, rgb
}

// trueColor decodes a colour long: its top byte says how to read the rest
// (0xC0 by layer, 0xC1 by block, 0xC2 RGB, 0xC3 an index in the low byte).
func trueColor(v uint32) (int, *vector.Color) {
	switch v >> 24 {
	case 0xC0:
		return cad.ByLayer, nil
	case 0xC1:
		return cad.ByBlock, nil
	case 0xC3:
		return int(v & 0xff), nil
	default:
		return cad.ByLayer, &vector.Color{
			R: uint8(v >> 16 & 0xff),
			G: uint8(v >> 8 & 0xff),
			B: uint8(v & 0xff),
		}
	}
}

// readCMC reads a CMC colour: a bitshort index before R2004; from then on
// an index, a colour long and a flag byte saying whether a colour name and
// book name follow, in the string stream from R2007.
func readCMC(d, s *bitReader, v ver) (int, *vector.Color) {
	idx := int(d.BS())
	if v < v2004 {
		return idx, nil
	}
	color, rgb := trueColor(d.blBits())
	if rgb != nil && idx > 0 && idx < cad.ByLayer {
		// A true colour keeps its nearest index beside it, which is what
		// the DXF reader takes for the index too.
		color = idx
	}
	flags := d.RC()
	for _, bit := range []byte{1, 2} {
		if flags&bit == 0 {
			continue
		}
		switch {
		case v < v2007:
			d.T()
		case s != nil:
			s.TU()
		}
	}
	return color, rgb
}
