package dwg

import (
	"fmt"
	"strings"
)

// header is what the reader takes from the header variables.
type header struct {
	insUnits       int
	extMin, extMax vec3
	// Control objects and the two layout block records. The reader finds
	// all of these from the objects themselves too; the header's are a
	// cross-check and the fallback.
	blockControl, layerControl uint64
	modelSpace, paperSpace     uint64
}

// headerStep is a run of header variables, present from version from up
// to version to.
type headerStep struct {
	from, to ver
	fields   string
}

// headerLayout lists the header variables of R2000 and later in file order
// (ODA specification chapter 9, cross-checked with ACadSharp's
// DwgHeaderReader). Each field is a bit-code type; a ":name" suffix keeps
// the value. Only the types matter for getting to $INSUNITS and the
// handles after it: everything else is read and dropped.
//
// "H" handles are in the handle stream from R2007 on, and "TV" strings in
// the string stream, so on those versions neither moves the data stream;
// HANDSEED alone ("HS") stays in the data stream. "CMC" is a colour, a
// bitshort before R2004.
var headerLayout = []headerStep{
	{v2013, vMax, "BLL"},
	{v2000, vMax, "BD BD BD BD TV TV TV TV BL BL"},
	{v2000, v2000, "H"},
	{v2000, vMax, "B B B B B B B B B"},
	{v2004, vMax, "B"},
	{v2000, vMax, "B B B B B B B B B B B BS BS BS BS BS BS BS BS"},
	{v2004, vMax, "BL BL BL"},
	{v2000, vMax, "BS BS BS BS BS BS BS BS BS BS BS BS BS BS BS BS BS BS BS"},
	{v2000, vMax, "BD BD BD BD BD BD BD BD BD BD BD BD BD BD BD BD BD BD BD BD BD"},
	{v2000, vMax, "TV BL BL BL BL"},
	{v2004, vMax, "BL BL BL"},
	{v2000, vMax, "BL BL BL BL CMC HS H H H"},
	{v2007, vMax, "H"},
	{v2000, vMax, "H H BD"},
	// Paper space: INSBASE EXTMIN EXTMAX LIMMIN LIMMAX ELEVATION UCS...
	{v2000, vMax, "3BD 3BD 3BD 2RD 2RD BD 3BD 3BD 3BD H H BS H 3BD 3BD 3BD 3BD 3BD 3BD"},
	// Model space, the same.
	{v2000, vMax, "3BD 3BD:extmin 3BD:extmax 2RD 2RD BD 3BD 3BD 3BD H H BS H"},
	{v2000, vMax, "3BD 3BD 3BD 3BD 3BD 3BD TV TV BD BD BD BD BD BD BD BD BD"},
	{v2007, vMax, "BD BD BS CMC"},
	{v2000, vMax, "B B B B B B BS BS BS"},
	{v2007, vMax, "BS"},
	{v2000, vMax, "BD BD BD BD BD BD BD BD BD B BS B B B B CMC CMC CMC"},
	{v2000, vMax, "BS BS BS BS BS BS BS BS BS BS BS B B BS BS BS BS B BS"},
	{v2007, vMax, "B"},
	{v2010, vMax, "B BD TV BD TV"},
	{v2000, vMax, "H H H H H"},
	{v2007, vMax, "H H H"},
	{v2000, vMax, "BS BS H:blockcontrol H:layercontrol H H H H H H H"},
	{v2000, v2000, "H"},
	{v2000, vMax, "H H H BS BS TV TV H H H"},
	{v2004, vMax, "H H"},
	{v2007, vMax, "H"},
	{v2013, vMax, "H"},
	// "H?" is CPSNID, there only when CEPSNTYPE is 3.
	{v2000, vMax, "BL BS:insunits BS:psntype H? TV TV"},
	{v2004, vMax, "RC RC RC RC RC RC BS BS RC RC TV"},
	{v2000, vMax, "H:paperspace H:modelspace H H H"},
}

// readHeader reads the header variables section.
func (d *decoder) readHeader() (header, error) {
	var h header
	r, err := d.sectionStart(d.c.Sections[SectionHeader], sentinelHeader)
	if err != nil {
		return h, err
	}
	hr := headerReader{d: r, s: r, h: r, ver: d.ver}
	if d.ver >= v2007 {
		// Like an object, the header keeps strings and handles in streams
		// of their own after the data; the bit size, counted from where it
		// is read, marks where the data ends and the handles begin.
		start := r.pos
		end := start + int(r.RL())
		hr.h = r.sub(end, r.end)
		hr.s = stringStream(r, end, start)
		r.end = end - 1
		if hr.s != nil {
			r.end = hr.s.pos
		}
	}
	for _, step := range headerLayout {
		if d.ver < step.from || d.ver > step.to {
			continue
		}
		for f := range strings.FieldsSeq(step.fields) {
			kind, name, _ := strings.Cut(f, ":")
			hr.field(&h, kind, name)
		}
	}
	for _, x := range []*bitReader{hr.d, hr.s, hr.h} {
		if x != nil && x.err != nil {
			return h, fmt.Errorf("header variables: %w", x.err)
		}
	}
	return h, nil
}

// headerReader reads header variables from their three streams, which are
// one and the same before R2007.
type headerReader struct {
	d, s, h *bitReader
	ver     ver
	psnType int16
}

func (r *headerReader) field(h *header, kind, name string) {
	switch kind {
	case "B":
		r.d.B()
	case "BS":
		v := r.d.BS()
		switch name {
		case "insunits":
			h.insUnits = int(v)
		case "psntype":
			r.psnType = v
		}
	case "BL":
		r.d.BL()
	case "BLL":
		r.d.BLL()
	case "BD":
		r.d.BD()
	case "RC":
		r.d.RC()
	case "2RD":
		r.d.RD2()
	case "3BD":
		v := r.d.BD3()
		switch name {
		case "extmin":
			h.extMin = v
		case "extmax":
			h.extMax = v
		}
	case "TV":
		if r.ver < v2007 {
			r.d.T()
		} else if r.s != nil {
			r.s.TU()
		}
	case "CMC":
		readCMC(r.d, r.s, r.ver)
	case "HS":
		r.d.handle(0)
	case "H":
		v := r.h.handle(0)
		switch name {
		case "blockcontrol":
			h.blockControl = v
		case "layercontrol":
			h.layerControl = v
		case "paperspace":
			h.paperSpace = v
		case "modelspace":
			h.modelSpace = v
		}
	case "H?":
		if r.psnType == 3 {
			r.h.handle(0)
		}
	}
}
