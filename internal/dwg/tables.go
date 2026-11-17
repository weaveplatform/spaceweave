package dwg

import (
	"github.com/weaveplatform/spaceweave/internal/cad"
)

// blockRec is a BLOCK_HEADER: a block's record, which holds its base point
// and lists (or, before R2004, chains) the entities in it.
type blockRec struct {
	handle      uint64
	name        string
	base        cad.Point
	xref        bool
	blockEnt    uint64   // the BLOCK entity, whose name is the full one
	owned       []uint64 // R2004+
	first, last uint64   // R2000
}

// tableEntry reads what every table entry starts with after the common
// object data: its name and xref flags.
func (o *obj) tableEntry() string {
	name := o.tv()
	if o.ver >= v2007 {
		o.d.BS() // xref index, with the xref-dependent bit
	} else {
		o.d.B()  // the 70 group's 64 bit
		o.d.BS() // xref index + 1
		o.d.B()  // xref dependent
	}
	return name
}

func (d *decoder) blockHeader(o *obj) *blockRec {
	o.common()
	b := &blockRec{handle: o.handle, name: o.tableEntry()}
	o.d.B() // anonymous
	o.d.B() // has attribute definitions
	isXref, overlaid := o.d.B(), o.d.B()
	b.xref = isXref || overlaid
	o.d.B() // xref loaded
	var n int32
	if o.ver >= v2004 && !b.xref {
		n = o.d.BL()
	}
	b.base = point2(o.d.BD3())
	o.tv() // xref path
	// The inserts of this block are counted with a run of non-zero bytes.
	inserts := 0
	for o.d.RC() != 0 && o.d.err == nil {
		inserts++
	}
	o.tv() // description
	size := o.d.count(o.d.BL(), 8)
	o.d.skip(size * 8) // preview image
	o.ref()            // NULL
	b.blockEnt = o.ref()
	if o.ver >= v2004 {
		b.owned = o.refs(n)
	} else if !b.xref {
		b.first, b.last = o.ref(), o.ref()
	}
	return b
}

// blockControl reads the BLOCK_CONTROL object for its model and paper
// space block records, which follow its list of the others.
func (d *decoder) blockControl(o *obj) (model, paper uint64) {
	o.common()
	o.refs(o.d.BL())
	return o.ref(), o.ref()
}

// Layer flag bits (R2000+).
const (
	layerFrozen = 1
	layerOff    = 2
)

func (d *decoder) layer(o *obj) *cad.Layer {
	o.common()
	l := &cad.Layer{Name: o.tableEntry()}
	flags := o.d.BS()
	color, rgb := readCMC(o.d, o.s, o.ver)
	// A negative colour is the other way a layer is said to be off.
	l.Color, l.RGB, l.Off, l.Frozen = color, rgb, flags&layerOff != 0 ||
		color < 0, flags&layerFrozen != 0
	if color < 0 {
		l.Color = -color
	}
	return l
}
