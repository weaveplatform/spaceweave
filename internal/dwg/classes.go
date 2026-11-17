package dwg

import (
	"bytes"
	"fmt"
)

// Section sentinels (ODA specification chapters 9 and 10).
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

// class is a custom object type: object types from 500 up are numbered
// per drawing, and the classes section says what each number is.
type class struct {
	name   string // DXF name, e.g. "LWPOLYLINE", "ACDBPLACEHOLDER"
	entity bool
}

// classItemEntity is a class's item class ID for entities (0x1F3 is for
// other objects).
const classItemEntity = 0x1F2

// sectionStart checks a section's sentinel and reads its size (and the
// R2010+ high word some writers add), returning a reader over the data.
func (d *decoder) sectionStart(sec, sentinel []byte) (*bitReader, error) {
	if len(sec) < 20 || !bytes.Equal(sec[:16], sentinel) {
		return nil, fmt.Errorf("%w: section sentinel missing", ErrCorrupt)
	}
	r := newBits(sec)
	r.enc = d.enc
	r.pos = 16 * 8
	size := int(r.RL())
	if d.hasHighSize() {
		r.RL()
	}
	end := r.pos + size*8
	if size < 0 || end > r.end {
		return nil, fmt.Errorf("%w: section size %d past its end", ErrCorrupt, size)
	}
	r.end = end
	return r, nil
}

// hasHighSize reports whether section sizes are followed by an unknown
// long: R2010 and R2013 files from maintenance release 4 on, and every
// R2018 file.
func (d *decoder) hasHighSize() bool {
	return d.ver >= v2018 || (d.ver >= v2010 && d.c.Maintenance > 3)
}

// readClasses reads the classes section into class numbers and names.
func (d *decoder) readClasses() error {
	r, err := d.sectionStart(d.c.Sections[SectionClasses], sentinelClasses)
	if err != nil {
		return err
	}
	strs := r
	if d.ver >= v2007 {
		// The strings live in a stream at the section's end, found the way
		// an object's are; the bit size counts from where it is read.
		start := r.pos
		bits := int(r.RL())
		strs = stringStream(r, start+bits, start)
		if strs == nil {
			return fmt.Errorf("%w: classes have no strings", ErrCorrupt)
		}
		r.end = strs.pos
	}
	if d.ver >= v2004 {
		r.BS() // highest class number
		r.RC()
		r.RC()
		r.B()
	}
	tv := func() string {
		if d.ver >= v2007 {
			return strs.TU()
		}
		return r.T()
	}
	// A class takes at least 8 bits even with every field at its shortest,
	// which bounds the loop on corrupt data.
	for r.left() >= 8 && r.err == nil && strs.err == nil {
		num := int(r.BS())
		r.BS() // proxy flags
		tv()   // application name
		tv()   // C++ class name
		name := tv()
		r.B() // was a zombie
		item := int(r.BS())
		if d.ver >= v2004 {
			r.BL() // instance count
			r.BL() // DWG version
			r.BL() // maintenance version
			r.BL()
			r.BL()
		}
		if r.err == nil && strs.err == nil {
			d.classes[num] = class{name: name, entity: item == classItemEntity}
		}
	}
	if r.err != nil {
		return r.err
	}
	return strs.err
}
