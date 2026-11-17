package dwg

import (
	"fmt"
	"sort"
	"strings"

	"github.com/weaveplatform/spaceweave/internal/cad"
)

// run reads the sections and objects and builds the document.
func (d *decoder) run() error {
	if h, err := d.readHeader(); err != nil {
		d.doc.Notes = append(
			d.doc.Notes,
			"The drawing's header couldn't be read, so its units aren't known.",
		)
	} else {
		d.hdr = h
		d.doc.Units = cad.Units(h.insUnits)
	}
	if err := d.readClasses(); err != nil {
		// Without classes, objects numbered by class can't be told apart;
		// the fixed types, which most of a plan is, still read.
		d.doc.Notes = append(
			d.doc.Notes,
			"The drawing's class list couldn't be read; some objects may be missing.",
		)
	}
	entries, err := readObjectMap(d.c.Sections[SectionHandles])
	if len(entries) == 0 {
		if err == nil {
			err = fmt.Errorf("%w: the object map is empty", ErrCorrupt)
		}
		return err
	}
	if err != nil {
		d.doc.Notes = append(
			d.doc.Notes,
			"The drawing's object map is damaged; some objects may be missing.",
		)
	}
	d.readObjects(entries)
	d.build()
	return nil
}

// kinds maps the fixed entity types the reader decodes to decodeEntity's
// names for them.
var kinds = map[int]string{
	typText: "TEXT", typAttrib: "ATTRIB", typBlock: "BLOCK", typInsert: "INSERT",
	typMinsert: "MINSERT", typVertex2D: "VERTEX2D", typVertex3D: "VERTEX3D",
	typPolyline2D: "POLYLINE2D", typPolyline3D: "POLYLINE3D", typArc: "ARC",
	typCircle: "CIRCLE", typLine: "LINE", typSolid: "SOLID", typTrace: "SOLID",
	typEllipse: "ELLIPSE", typSpline: "SPLINE", typMText: "MTEXT",
	typLWPolyline: "LWPOLYLINE", typHatch: "HATCH",
}

// classKinds maps class names to decodeEntity's names: the types a drawing
// may number as classes, and the dimensions that only exist as classes.
var classKinds = map[string]string{
	"LWPOLYLINE": "LWPOLYLINE", "HATCH": "HATCH",
	"ARC_DIMENSION": "DIMENSION", "LARGE_RADIAL_DIMENSION": "DIMENSION",
}

func (d *decoder) readObjects(entries []mapEntry) {
	d.layers = map[uint64]*cad.Layer{}
	d.blocks = map[uint64]*blockRec{}
	d.recs = map[uint64]*rec{}
	d.notDrawn = map[string]int{}
	for _, m := range entries {
		o, err := d.open(m.offset)
		if err != nil {
			d.skip("unreadable")
			continue
		}
		name, kind, entity := d.classify(o.typ)
		switch {
		case o.typ == typBlockControl:
			if model, paper := d.blockControl(o); o.err() == nil {
				d.model, d.paper = model, paper
			}
		case o.typ == typBlockHeader:
			if b := d.blockHeader(o); o.err() == nil {
				d.blocks[b.handle] = b
			} else {
				d.skip("BLOCK_HEADER")
			}
		case o.typ == typLayer:
			if l := d.layer(o); o.err() == nil {
				d.layers[o.handle] = l
			} else {
				d.skip("LAYER")
			}
		case kind != "":
			r := d.decodeEntity(o, kind)
			if o.err() != nil {
				d.skip(name)
				continue
			}
			d.st.types[name]++
			d.recs[r.handle] = r
			d.order = append(d.order, r.handle)
		case entity && !notDrawn[name] && name != "":
			d.notDrawn[name]++
		case name == "" && o.typ >= typClassBase:
			d.skip(fmt.Sprintf("class %d", o.typ))
		}
	}
}

// classify names an object type and says whether the reader decodes it
// (kind) and whether it is an entity.
func (d *decoder) classify(typ int) (name, kind string, entity bool) {
	if typ >= typClassBase && typ != typProxyEntity && typ != typProxyEntity+1 {
		c, ok := d.classes[typ]
		if !ok {
			return "", "", false
		}
		return c.name, classKinds[c.name], c.entity
	}
	name, entity = entityNames[typ]
	if typ >= 0x14 && typ <= typDimDiameter {
		return name, "DIMENSION", true
	}
	return name, kinds[typ], entity
}

func (d *decoder) skip(what string) {
	d.doc.Skipped++
	d.st.skipped[what]++
}

// build assembles the document from the decoded objects.
func (d *decoder) build() {
	for _, l := range d.layers {
		if l.Name == "" {
			continue
		}
		if l.Color == cad.ByBlock || l.Color == cad.ByLayer {
			l.Color = 7
		}
		d.doc.Layers[l.Name] = l
	}
	d.findSpaces()
	names := d.blockNames()
	members := d.members()
	for _, b := range d.sortedBlocks() {
		ents := d.place(members, b.handle, b.owned, b.first, b.last, names)
		switch b.handle {
		case d.model:
			d.doc.Model = ents
		case d.paper:
			d.doc.Paper = ents
		default:
			d.doc.Blocks[names[b.handle]] = &cad.Block{
				Name:     names[b.handle],
				Base:     b.base,
				Entities: ents,
			}
		}
	}
	d.doc.Notes = append(d.doc.Notes, d.notDrawnNote()...)
}

// findSpaces settles which block records are model and paper space: the
// block control object's say, then the header's, then their names.
func (d *decoder) findSpaces() {
	if d.blocks[d.model] == nil {
		d.model = d.hdr.modelSpace
	}
	if d.blocks[d.paper] == nil {
		d.paper = d.hdr.paperSpace
	}
	for h, b := range d.blocks {
		switch {
		case d.blocks[d.model] == nil && strings.EqualFold(b.name, "*Model_Space"):
			d.model = h
		case d.blocks[d.paper] == nil && strings.EqualFold(b.name, "*Paper_Space"):
			d.paper = h
		}
	}
}

func (d *decoder) sortedBlocks() []*blockRec {
	out := make([]*blockRec, 0, len(d.blocks))
	for _, b := range d.blocks {
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].handle < out[j].handle })
	return out
}

// blockNames names every block record. The BLOCK entity has the full name
// (a record of an anonymous block may have only "*U"); names that clash
// get the record's handle appended, since inserts find blocks by handle
// and two blocks under one name would draw the wrong one.
func (d *decoder) blockNames() map[uint64]string {
	names := map[uint64]string{}
	used := map[string]bool{}
	for _, b := range d.sortedBlocks() {
		name := b.name
		if r := d.recs[b.blockEnt]; r != nil && r.name != "" {
			name = r.name
		}
		if used[strings.ToUpper(name)] {
			name = fmt.Sprintf("%s~%X", name, b.handle)
		}
		used[strings.ToUpper(name)] = true
		names[b.handle] = name
	}
	return names
}

// members indexes entities by owner: a block record or, for attributes
// and vertices, their INSERT or POLYLINE. Model and paper space entities
// name no owner; their mode says which space they are in.
func (d *decoder) members() map[uint64][]uint64 {
	m := map[uint64][]uint64{}
	for _, h := range d.order {
		r := d.recs[h]
		key := r.ent.owner
		switch r.ent.mode {
		case modeModel:
			key = d.model
		case modePaper:
			key = d.paper
		}
		m[key] = append(m[key], h)
	}
	return m
}

// ordered lists an owner's entities: in its owned list from R2004, along its
// chain before that, and then any that claim it as owner but neither
// lists, in handle order.
func (d *decoder) ordered(
	members map[uint64][]uint64,
	owner uint64,
	owned []uint64,
	first, last uint64,
) []*rec {
	seen := map[uint64]bool{}
	var out []*rec
	add := func(h uint64) bool {
		r := d.recs[h]
		if r == nil || seen[h] {
			return false
		}
		seen[h] = true
		out = append(out, r)
		return true
	}
	for _, h := range owned {
		add(h)
	}
	if first != 0 {
		// The chain can't be longer than the entities there are; a cycle or
		// a broken link ends it, and the stragglers follow.
		for h, n := first, 0; n < len(d.recs); n++ {
			if !add(h) || h == last {
				break
			}
			h = d.recs[h].ent.next
		}
	}
	for _, h := range members[owner] {
		add(h)
	}
	return out
}

// place turns an owner's entities into the document's, resolving layers,
// block names and children.
func (d *decoder) place(
	members map[uint64][]uint64, owner uint64, owned []uint64, first, last uint64,
	names map[uint64]string,
) []cad.Entity {
	var out []cad.Entity
	for _, r := range d.ordered(members, owner, owned, first, last) {
		if r.e == nil || r.typ == typAttrib {
			continue
		}
		d.resolve(r, names)
		switch e := r.e.(type) {
		case *cad.Insert:
			for _, k := range d.ordered(members, r.handle, r.owned, r.first, r.last) {
				if a, ok := k.e.(*cad.Text); ok && k.typ == typAttrib {
					d.resolve(k, names)
					e.Attribs = append(e.Attribs, a)
				}
			}
		case *cad.Polyline:
			if r.typ == typPolyline2D || r.typ == typPolyline3D {
				for _, k := range d.ordered(members, r.handle, r.owned, r.first, r.last) {
					if (k.typ == typVertex2D || k.typ == typVertex3D) && k.vflags&vxFrame == 0 {
						e.Vertices = append(e.Vertices, k.vertex)
					}
				}
			}
		}
		out = append(out, r.e)
	}
	return out
}

// resolve fills in an entity's layer name and the block it draws.
func (d *decoder) resolve(r *rec, names map[uint64]string) {
	c := r.e.Base()
	c.Layer = "0"
	if l := d.layers[r.ent.layer]; l != nil && l.Name != "" {
		c.Layer = l.Name
	}
	switch e := r.e.(type) {
	case *cad.Insert:
		e.Block = names[r.block]
	case *cad.Dimension:
		e.Block = names[r.block]
	}
}

func (d *decoder) notDrawnNote() []string {
	if len(d.notDrawn) == 0 {
		return nil
	}
	names := make([]string, 0, len(d.notDrawn))
	for n := range d.notDrawn {
		names = append(names, n)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, n := range names {
		parts = append(parts, fmt.Sprintf("%d %s", d.notDrawn[n], n))
	}
	return []string{"Not drawn: " + strings.Join(parts, ", ") + "."}
}
