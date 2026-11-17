package dwgtest

import (
	"maps"
	"math"
	"slices"
	"strings"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// ver orders the versions so comparisons read as "R2004 and later".
type ver int

const (
	v2000 ver = iota
	v2004
	v2007 // not written, but its changes carry forward
	v2010
	v2013
	v2018
)

var versionOf = map[string]ver{
	"AC1015": v2000, "AC1018": v2004, "AC1024": v2010, "AC1027": v2013, "AC1032": v2018,
}

// Fixed object types (ODA specification 20.3).
const (
	typText         = 0x01
	typAttrib       = 0x02
	typBlock        = 0x04
	typEndblk       = 0x05
	typSeqend       = 0x06
	typInsert       = 0x07
	typMinsert      = 0x08
	typVertex2D     = 0x0A
	typVertex3D     = 0x0B
	typPolyline2D   = 0x0F
	typPolyline3D   = 0x10
	typArc          = 0x11
	typCircle       = 0x12
	typLine         = 0x13
	typDimLinear    = 0x15
	typPoint        = 0x1B
	typFace3D       = 0x1C
	typPolyPface    = 0x1D
	typSolid        = 0x1F
	typEllipse      = 0x23
	typSpline       = 0x24
	typMText        = 0x2C
	typBlockControl = 0x30
	typBlockHeader  = 0x31
	typLayerControl = 0x32
	typLayer        = 0x33
	typLWPolyline   = 0x4D
	typHatch        = 0x4E
)

// The classes every file declares, and the numbers objects use for them.
// LWPOLYLINE and HATCH are fixed types too; files use either, so the
// writer uses both.
const (
	clsPlaceholder = 500 + iota
	clsLWPolyline
	clsHatch
	clsWall
	clsArcDim
	// clsUndeclared is a number no class has.
	clsUndeclared = 599
)

type classDef struct {
	num    int
	name   string
	entity bool
}

var classes = []classDef{
	{clsPlaceholder, "ACDBPLACEHOLDER", false},
	{clsLWPolyline, "LWPOLYLINE", true},
	{clsHatch, "HATCH", true},
	{clsWall, "AEC_WALL", true},
	{clsArcDim, "ARC_DIMENSION", true},
}

// Fixed handles.
const (
	hBlockControl = 1
	hLayerControl = 2
	hStyle        = 3 // a text style, line type and so on: never looked up
	hFirst        = 0x10
)

// object is one object being written: its data, string and handle
// streams. Before R2007 strings go in the data stream.
type object struct {
	handle  uint64
	typ     int
	d, s, h *Bits
}

// ref writes a reference to target, in the relative forms when they fit,
// as AutoCAD does for neighbours.
func (o *object) ref(target uint64) {
	switch {
	case target == 0:
		o.h.H(5, 0)
	case target == o.handle+1:
		o.h.H(6, 0)
	case target+1 == o.handle:
		o.h.H(8, 0)
	case target > o.handle && target-o.handle < 0x10:
		o.h.H(0xA, target-o.handle)
	case target < o.handle && o.handle-target < 0x10:
		o.h.H(0xC, o.handle-target)
	default:
		o.h.H(5, target)
	}
}

type encoder struct {
	v     ver
	dr    Drawing
	objs  []*object
	next  uint64
	k     int // entities written, which picks among equivalent encodings
	model uint64
	paper uint64

	layers map[string]uint64
	blocks map[string]uint64 // by upper-case name
}

func newEncoder(dr Drawing) *encoder {
	v, ok := versionOf[dr.Version]
	if !ok {
		panic("dwgtest: unknown version " + dr.Version)
	}
	e := &encoder{
		v:      v,
		dr:     dr,
		next:   hFirst,
		layers: map[string]uint64{},
		blocks: map[string]uint64{},
	}
	e.build()
	return e
}

func (e *encoder) alloc() uint64 {
	h := e.next
	e.next++
	return h
}

func (e *encoder) object(h uint64, typ int) *object {
	o := &object{handle: h, typ: typ, d: &Bits{}, h: &Bits{}}
	o.s = o.d
	if e.v >= v2007 {
		o.s = &Bits{}
	}
	e.objs = append(e.objs, o)
	return o
}

func (e *encoder) tv(o *object, s string) {
	if e.v >= v2007 {
		o.s.TU(s)
	} else {
		o.d.T(s)
	}
}

// build writes every object.
func (e *encoder) build() {
	doc := e.dr.Doc
	layerNames := slices.Sorted(maps.Keys(doc.Layers))
	for _, n := range layerNames {
		e.layers[n] = e.alloc()
	}
	blockNames := slices.Sorted(maps.Keys(doc.Blocks))
	e.model, e.paper = e.alloc(), e.alloc()
	others := make([]uint64, 0, len(blockNames))
	for _, n := range blockNames {
		h := e.alloc()
		e.blocks[strings.ToUpper(n)] = h
		others = append(others, h)
	}

	o := e.object(hBlockControl, typBlockControl)
	e.nonEntity(o, 0, 0)
	o.d.BL(n32(len(others)))
	for _, h := range others {
		o.ref(h)
	}
	o.ref(e.model)
	o.ref(e.paper)

	o = e.object(hLayerControl, typLayerControl)
	e.nonEntity(o, 0, 0)
	o.d.BL(n32(len(layerNames)))
	for _, n := range layerNames {
		o.ref(e.layers[n])
	}

	for i, n := range layerNames {
		e.layer(e.layers[n], doc.Layers[n], i)
	}
	model := doc.Model
	if e.dr.Extras {
		model = append(slices.Clip(model), &mesh{})
	}
	e.block(e.model, "*Model_Space", cad.Point{}, model, modeModel)
	e.block(e.paper, "*Paper_Space", cad.Point{}, doc.Paper, modePaper)
	for _, n := range blockNames {
		b := doc.Blocks[n]
		e.block(e.blocks[strings.ToUpper(n)], n, b.Base, b.Entities, modeOwned)
	}
	if e.dr.Extras {
		e.extras()
	}
}

// Entity modes: owned by the entity's owner, or in a layout's space.
const (
	modeOwned = 0
	modePaper = 1
	modeModel = 2
)

// nonEntity writes what every non-entity object starts with: EED, its
// owner, reactors and extension dictionary. variant picks reactors and a
// dictionary for some.
func (e *encoder) nonEntity(o *object, owner uint64, variant int) {
	e.eed(o, variant%3 == 1)
	o.ref(owner)
	e.reactors(o, variant)
}

func (e *encoder) eed(o *object, some bool) {
	if some {
		o.d.BS(3)
		o.d.H(5, hStyle)
		o.d.RC(1)
		o.d.RC(2)
		o.d.RC(3)
	}
	o.d.BS(0)
}

func (e *encoder) reactors(o *object, variant int) {
	n := variant % 2
	o.d.BL(n32(n))
	for range n {
		o.h.H(4, hStyle)
	}
	switch {
	case e.v < v2004:
		o.h.H(3, 0)
	case variant%4 == 1:
		o.d.B(false)
		o.h.H(3, hStyle)
	default:
		o.d.B(true) // no extension dictionary
	}
	if e.v >= v2013 {
		o.d.B(false)
	}
}

func (e *encoder) tableEntry(o *object, name string) {
	e.tv(o, name)
	if e.v >= v2007 {
		o.d.BS(0)
	} else {
		o.d.B(false)
		o.d.BS(0)
		o.d.B(false)
	}
}

// Layer flags.
const (
	layerFrozen = 1
	layerOff    = 2
)

func (e *encoder) layer(h uint64, l *cad.Layer, i int) {
	o := e.object(h, typLayer)
	e.nonEntity(o, hLayerControl, i)
	e.tableEntry(o, l.Name)
	var flags int16
	if l.Frozen {
		flags |= layerFrozen
	}
	color := l.Color
	if l.Off {
		// R2000 can also say "off" with a negative colour.
		if e.v == v2000 && i%2 == 1 {
			color = -color
		} else {
			flags |= layerOff
		}
	}
	o.d.BS(flags)
	e.cmc(o, color, l.RGB, i)
	o.ref(hStyle) // plot style
	o.ref(hStyle) // line type
}

// cmc writes a CMC colour: an index, and from R2004 a colour long and,
// for some, a colour name.
func (e *encoder) cmc(o *object, color int, rgb *vector.Color, variant int) {
	o.d.BS(int16(color)) //nolint:gosec // colour indexes are small
	if e.v < v2004 {
		return
	}
	switch {
	case rgb != nil:
		bl(o.d, 0xC2<<24|uint32(rgb.R)<<16|uint32(rgb.G)<<8|uint32(rgb.B))
	case color == cad.ByLayer:
		bl(o.d, 0xC0<<24)
	case color == cad.ByBlock:
		bl(o.d, 0xC1<<24)
	default:
		bl(o.d, 0xC3<<24|u32(color&0xff))
	}
	if variant%2 == 1 {
		o.d.RC(3) // a colour name and a book name
		e.tv(o, "NAME")
		e.tv(o, "BOOK")
	} else {
		o.d.RC(0)
	}
}

// bl writes all 32 bits of v as a bitlong.
func bl(w *Bits, v uint32) {
	w.BB(0)
	w.RL(v)
}

// block writes a block record, its BLOCK entity and its entities.
func (e *encoder) block(h uint64, name string, base cad.Point, ents []cad.Entity, mode int) {
	blockEnt := e.alloc()
	handles := e.entities(ents, mode, h)
	end := e.alloc()

	o := e.object(h, typBlockHeader)
	e.nonEntity(o, hBlockControl, int(h%4))
	e.tableEntry(o, name)
	o.d.B(strings.HasPrefix(name, "*")) // anonymous
	o.d.B(false)                        // has attribute definitions
	o.d.B(false)                        // an xref
	o.d.B(false)                        // an overlaid xref
	o.d.B(false)                        // a loaded xref
	if e.v >= v2004 {
		o.d.BL(n32(len(handles)))
	}
	o.d.BD3(base.X, base.Y, 0)
	e.tv(o, "") // xref path
	if h%2 == 1 {
		o.d.RC(1)
		o.d.RC(1)
	}
	o.d.RC(0)
	e.tv(o, "") // description
	if h%2 == 0 {
		o.d.BL(3)
		o.d.RC(1)
		o.d.RC(2)
		o.d.RC(3)
	} else {
		o.d.BL(0)
	}
	o.ref(0)
	o.ref(blockEnt)
	if e.v >= v2004 {
		for _, c := range handles {
			o.ref(c)
		}
	} else {
		first, last := uint64(0), uint64(0)
		if len(handles) > 0 {
			first, last = handles[0], handles[len(handles)-1]
		}
		o.ref(first)
		o.ref(last)
	}
	o.ref(end)

	b := e.object(blockEnt, typBlock)
	e.entity(b, common{mode: modeOwned, owner: h, layer: e.layers["0"]})
	e.tv(b, name)
	en := e.object(end, typEndblk)
	e.entity(en, common{mode: modeOwned, owner: h, layer: e.layers["0"]})
}

// common is the entity data the writer varies by entity.
type common struct {
	mode       int
	owner      uint64
	prev, next uint64 // R2000's chain
	layer      uint64
	color      int
	rgb        *vector.Color
	invisible  bool
}

// entities writes a list of entities, chained for R2000, and returns their
// handles. Handles are given out before any entity is written so each can
// name its neighbours.
func (e *encoder) entities(ents []cad.Entity, mode int, owner uint64) []uint64 {
	hs := make([]uint64, len(ents))
	for i := range hs {
		hs[i] = e.alloc()
	}
	for i, ent := range ents {
		c := common{mode: mode, owner: owner}
		if i > 0 {
			c.prev = hs[i-1]
		}
		if i+1 < len(hs) {
			c.next = hs[i+1]
		}
		b := ent.Base()
		c.layer, c.color, c.rgb, c.invisible = e.layers[b.Layer], b.Color, b.RGB, b.Invisible
		e.write(hs[i], ent, c)
	}
	return hs
}

// entity writes the common entity data (specification 20.4.1).
func (e *encoder) entity(o *object, c common) {
	k := e.k
	e.k++
	e.eed(o, k%5 == 1)
	graphic := k%7 == 3
	o.d.B(graphic)
	if graphic {
		if e.v >= v2010 {
			o.d.BLL(4)
		} else {
			o.d.RL(4)
		}
		o.d.RL(0xDEADBEEF)
	}
	e.entityMode(o, c, k)
}

func (e *encoder) entityMode(o *object, c common, k int) {
	o.d.BB(c.mode)
	if c.mode == modeOwned {
		o.ref(c.owner)
	}
	e.reactors(o, k)
	if e.v == v2000 {
		nolinks := c.prev+1 == o.handle && c.next == o.handle+1
		o.d.B(nolinks)
		if !nolinks {
			o.ref(c.prev)
			o.ref(c.next)
		}
	}
	e.color(o, c, k)
	o.d.BD(1) // line type scale
	o.ref(c.layer)
	if k%3 == 1 {
		o.d.BB(3)
		o.ref(hStyle)
	} else {
		o.d.BB(0)
	}
	if e.v >= v2007 {
		if k%3 == 2 {
			o.d.BB(3)
			o.ref(hStyle)
		} else {
			o.d.BB(0)
		}
		o.d.RC(0) // shadows
	}
	if k%5 == 4 {
		o.d.BB(3)
		o.ref(hStyle)
	} else {
		o.d.BB(0)
	}
	if e.v >= v2010 {
		o.d.B(k%2 == 1)
		if k%2 == 1 {
			o.ref(hStyle)
		}
		o.d.B(false)
		o.d.B(false)
	}
	if c.invisible {
		o.d.BS(1)
	} else {
		o.d.BS(0)
	}
	o.d.RC(0x1D) // line weight
}

// ENC flags.
const (
	encRGB          = 0x8000
	encBook         = 0x4000
	encTransparency = 0x2000
)

func (e *encoder) color(o *object, c common, k int) {
	if e.v < v2004 {
		o.d.BS(int16(c.color)) //nolint:gosec // colour indexes are small
		return
	}
	flags := uint16(0)
	var long uint32
	switch {
	case c.rgb != nil:
		flags = encRGB
		long = 0xC2<<24 | uint32(c.rgb.R)<<16 | uint32(c.rgb.G)<<8 | uint32(c.rgb.B)
	case k%4 != 1:
		flags = u16(c.color & 0x0fff)
	case c.color == cad.ByLayer:
		flags, long = encRGB, 0xC0<<24
	case c.color == cad.ByBlock:
		flags, long = encRGB, 0xC1<<24
	default:
		flags, long = encRGB|encBook, 0xC3<<24|u32(c.color&0xff)
	}
	if k%3 == 0 {
		flags |= encTransparency
	}
	o.d.BB(0)
	o.d.RS(flags)
	if flags&encRGB != 0 {
		bl(o.d, long)
	}
	if flags&encBook != 0 {
		o.ref(hStyle)
	}
	if flags&encTransparency != 0 {
		bl(o.d, 0x02000080)
	}
}

// extrusion is the Z of an entity's extrusion: down for a mirrored one.
func extrusion(c *cad.Common) float64 {
	if c.Mirrored {
		return -1
	}
	return 1
}

// write writes one entity of the model at handle h.
//
//nolint:gocyclo,cyclop // one case per entity type
func (e *encoder) write(h uint64, ent cad.Entity, c common) {
	k := e.k
	switch t := ent.(type) {
	case *cad.Line:
		o := e.object(h, typLine)
		e.entity(o, c)
		e.line(o, t)
	case *cad.Circle:
		o := e.object(h, typCircle)
		e.entity(o, c)
		o.d.BD3(t.Center.X, t.Center.Y, 0)
		o.d.BD(t.Radius)
		o.d.BT(0)
		o.d.BE(0, 0, extrusion(&t.Common))
	case *cad.Arc:
		o := e.object(h, typArc)
		e.entity(o, c)
		o.d.BD3(t.Center.X, t.Center.Y, 0)
		o.d.BD(t.Radius)
		o.d.BT(float64(k % 2))
		o.d.BE(0, 0, extrusion(&t.Common))
		o.d.BD(t.Start)
		o.d.BD(t.End)
	case *cad.Ellipse:
		o := e.object(h, typEllipse)
		e.entity(o, c)
		o.d.BD3(t.Center.X, t.Center.Y, 0)
		o.d.BD3(t.Major.X, t.Major.Y, 0)
		o.d.BD3(0, 0, 1)
		o.d.BD(t.Ratio)
		o.d.BD(t.Start)
		o.d.BD(t.End)
	case *cad.Polyline:
		e.polyline(h, t, c, k)
	case *mesh:
		o := e.object(h, typPolyline2D)
		e.entity(o, c)
		e.polyline2D(o, &t.Polyline, polylineMesh, k)
	case *cad.Spline:
		o := e.object(h, typSpline)
		e.entity(o, c)
		e.spline(o, t, k)
	case *cad.Text:
		o := e.object(h, typText)
		e.entity(o, c)
		e.text(o, t, k)
	case *cad.MText:
		o := e.object(h, typMText)
		e.entity(o, c)
		e.mtext(o, t)
		o.ref(hStyle)
	case *cad.Insert:
		e.insert(h, t, c, k)
	case *cad.Solid:
		o := e.object(h, typSolid)
		e.entity(o, c)
		o.d.BT(0)
		o.d.BD(0) // elevation
		for _, p := range t.Corners {
			o.d.RD2(p.X, p.Y)
		}
		o.d.BE(0, 0, extrusion(&t.Common))
	case *cad.Hatch:
		typ := typHatch
		if k%2 == 1 {
			typ = clsHatch
		}
		o := e.object(h, typ)
		e.entity(o, c)
		e.hatch(o, t, k)
	case *cad.Dimension:
		typ := typDimLinear
		if k%2 == 1 {
			typ = clsArcDim
		}
		o := e.object(h, typ)
		e.entity(o, c)
		// The dimension's own fields, which the reader steps over.
		o.d.RC(0)
		o.d.BD3(0, 0, 1)
		o.ref(hStyle) // dimension style
		o.ref(e.blocks[strings.ToUpper(t.Block)])
	}
}

func (e *encoder) line(o *object, t *cad.Line) {
	o.d.B(true) // Z is 0 at both ends
	o.d.RD(t.A.X)
	o.d.DD(t.B.X, t.A.X)
	o.d.RD(t.A.Y)
	o.d.DD(t.B.Y, t.A.Y)
	o.d.BT(0)
	o.d.BE(0, 0, extrusion(&t.Common))
}

// LWPOLYLINE flags.
const (
	lwExtrusion = 0x01
	lwThickness = 0x02
	lwWidth     = 0x04
	lwElevation = 0x08
	lwBulges    = 0x10
	lwWidths    = 0x20
	lwClosed    = 0x200
	lwIDs       = 0x400
)

// polyline writes a polyline as an LWPOLYLINE (fixed or class numbered)
// or as an old-style POLYLINE with VERTEX entities, 2D or, when it has
// neither bulges nor width, 3D.
func (e *encoder) polyline(h uint64, p *cad.Polyline, c common, k int) {
	plain := p.Width == 0 && !p.Mirrored
	for _, v := range p.Vertices {
		plain = plain && v.Bulge == 0
	}
	switch {
	case k%3 == 1:
		o := e.object(h, typPolyline2D)
		e.entity(o, c)
		var flags int16
		if p.Closed {
			flags = 1
		}
		e.polyline2D(o, p, flags, k)
	case k%3 == 2 && plain:
		o := e.object(h, typPolyline3D)
		e.entity(o, c)
		o.d.RC(0)
		if p.Closed {
			o.d.RC(1)
		} else {
			o.d.RC(0)
		}
		e.vertices(o, p, typVertex3D, k)
	default:
		typ := typLWPolyline
		if k%2 == 1 {
			typ = clsLWPolyline
		}
		o := e.object(h, typ)
		e.entity(o, c)
		e.lwpolyline(o, p, k)
	}
}

func (e *encoder) lwpolyline(o *object, p *cad.Polyline, k int) {
	flags := 0
	if p.Closed {
		flags |= lwClosed
	}
	if p.Width > 0 {
		if k%2 == 0 {
			flags |= lwWidth
		} else {
			flags |= lwWidths
		}
	}
	if k%4 == 3 {
		flags |= lwElevation | lwThickness
	}
	if p.Mirrored {
		flags |= lwExtrusion
	}
	for _, v := range p.Vertices {
		if v.Bulge != 0 {
			flags |= lwBulges
		}
	}
	if k%5 == 2 {
		flags |= lwIDs
	}
	o.d.BS(i16(flags))
	if flags&lwWidth != 0 {
		o.d.BD(p.Width)
	}
	if flags&lwElevation != 0 {
		o.d.BD(0)
		o.d.BD(0) // thickness
	}
	if flags&lwExtrusion != 0 {
		o.d.BD3(0, 0, -1)
	}
	n := n32(len(p.Vertices))
	o.d.BL(n)
	for _, f := range []int{lwBulges, lwIDs, lwWidths} {
		if flags&f != 0 {
			o.d.BL(n)
		}
	}
	for i, v := range p.Vertices {
		if i == 0 {
			o.d.RD2(v.P.X, v.P.Y)
			continue
		}
		prev := p.Vertices[i-1].P
		o.d.DD(v.P.X, prev.X)
		o.d.DD(v.P.Y, prev.Y)
	}
	if flags&lwBulges != 0 {
		for _, v := range p.Vertices {
			o.d.BD(v.Bulge)
		}
	}
	if flags&lwIDs != 0 {
		for i := range p.Vertices {
			o.d.BL(n32(i + 1))
		}
	}
	if flags&lwWidths != 0 {
		for range p.Vertices {
			o.d.BD(p.Width)
			o.d.BD(p.Width / 2)
		}
	}
}

func (e *encoder) polyline2D(o *object, p *cad.Polyline, flags int16, k int) {
	o.d.BS(flags)
	o.d.BS(0) // curve type
	o.d.BD(p.Width)
	o.d.BD(p.Width / 2)
	o.d.BT(0)
	o.d.BD(0) // elevation
	o.d.BE(0, 0, extrusion(&p.Common))
	e.vertices(o, p, typVertex2D, k)
}

// vertexFrame flags a spline frame's control point, which isn't on the
// curve.
const vertexFrame = 16

// vertices writes a POLYLINE's children: its VERTEX entities (a frame
// point first, for some, which the reader must leave out) and SEQEND.
func (e *encoder) vertices(o *object, p *cad.Polyline, typ, k int) {
	vs := p.Vertices
	frame := typ == typVertex2D && k%2 == 1
	n := len(vs)
	if frame {
		n++
	}
	hs := make([]uint64, n)
	for i := range hs {
		hs[i] = e.alloc()
	}
	seq := e.alloc()
	e.children(o, hs, seq)
	for i, h := range hs {
		c := common{mode: modeOwned, owner: o.handle, layer: e.layers[p.Layer]}
		if i > 0 {
			c.prev = hs[i-1]
		}
		if i+1 < len(hs) {
			c.next = hs[i+1]
		}
		v := e.object(h, typ)
		e.entity(v, c)
		var at cad.Vertex
		flags := byte(32)
		switch {
		case frame && i == 0:
			at, flags = cad.Vertex{P: cad.Point{X: -1e6, Y: -1e6}}, vertexFrame
		case frame:
			at = vs[i-1]
		default:
			at = vs[i]
		}
		v.d.RC(flags)
		v.d.BD3(at.P.X, at.P.Y, 0)
		if typ == typVertex3D {
			continue
		}
		if i%2 == 1 && p.Width > 0 {
			v.d.BD(-p.Width) // negative: the end width is the same
		} else {
			v.d.BD(p.Width)
			v.d.BD(p.Width)
		}
		v.d.BD(at.Bulge)
		if e.v >= v2010 {
			v.d.BL(0) // vertex id
		}
		v.d.BD(0) // tangent direction
	}
	e.seqend(seq, o.handle, e.layers[p.Layer])
}

// children writes an INSERT's or POLYLINE's handles of its children: a
// counted list from R2004, the first and last before; then its SEQEND.
func (e *encoder) children(o *object, hs []uint64, seq uint64) {
	if e.v >= v2004 {
		o.d.BL(n32(len(hs)))
		for _, h := range hs {
			o.ref(h)
		}
	} else {
		first, last := uint64(0), uint64(0)
		if len(hs) > 0 {
			first, last = hs[0], hs[len(hs)-1]
		}
		o.ref(first)
		o.ref(last)
	}
	o.ref(seq)
}

func (e *encoder) seqend(h, owner, layer uint64) {
	o := e.object(h, typSeqend)
	e.entity(o, common{mode: modeOwned, owner: owner, layer: layer})
}

func (e *encoder) spline(o *object, s *cad.Spline, k int) {
	scenario := int32(1)
	if len(s.Control) == 0 {
		scenario = 2
	}
	o.d.BL(scenario)
	if e.v >= v2013 {
		var flags1, knotParam int32
		if s.Closed {
			flags1 |= 4
		}
		switch {
		case scenario == 2:
			flags1 |= 8
		case k%2 == 1:
			flags1 |= 8
			knotParam = 15
		}
		o.d.BL(flags1)
		o.d.BL(knotParam)
	}
	o.d.BL(n32(s.Degree))
	if scenario == 2 {
		o.d.BD(1e-10) // fit tolerance
		o.d.BD3(0, 0, 0)
		o.d.BD3(0, 0, 0) // tangents
		o.d.BL(n32(len(s.Fit)))
		for _, p := range s.Fit {
			o.d.BD3(p.X, p.Y, 0)
		}
		return
	}
	weighted := len(s.Weights) == len(s.Control)
	o.d.B(weighted) // rational
	o.d.B(s.Closed)
	o.d.B(false)  // periodic
	o.d.BD(1e-10) // knot tolerance
	o.d.BD(1e-10) // control tolerance
	o.d.BL(n32(len(s.Knots)))
	o.d.BL(n32(len(s.Control)))
	o.d.B(weighted)
	for _, v := range s.Knots {
		o.d.BD(v)
	}
	for i, p := range s.Control {
		o.d.BD3(p.X, p.Y, 0)
		if weighted {
			o.d.BD(s.Weights[i])
		}
	}
}

// TEXT data flags: a set bit leaves the field out.
const (
	txNoElevation = 0x01
	txNoAlign     = 0x02
	txNoOblique   = 0x04
	txNoRotation  = 0x08
	txNoWidth     = 0x10
	txNoMirror    = 0x20
	txNoHAlign    = 0x40
	txNoVAlign    = 0x80
)

// text writes TEXT's fields, which ATTRIB starts with. Fields at their
// defaults are left out, except that some texts spell them out.
func (e *encoder) text(o *object, t *cad.Text, k int) {
	aligned := t.Align != 0 || t.VAlign != 0
	var df byte
	if k%2 == 0 {
		df |= txNoElevation | txNoOblique | txNoMirror
	}
	if !aligned {
		df |= txNoAlign
	}
	if t.Rotation == 0 {
		df |= txNoRotation
	}
	if t.WidthFactor == 1 {
		df |= txNoWidth
	}
	if t.Align == 0 {
		df |= txNoHAlign
	}
	if t.VAlign == 0 {
		df |= txNoVAlign
	}
	// At is the alignment point, except for aligned (3) and fit (5) text
	// and unaligned text, where it is the insertion point.
	ins, align := t.At, t.At
	switch {
	case !aligned:
	case t.Align == 3 || t.Align == 5:
		align = t.At.Add(cad.Point{X: 1000})
	default:
		ins = t.At.Add(cad.Point{X: -50, Y: -50})
	}
	o.d.RC(df)
	if df&txNoElevation == 0 {
		o.d.RD(0)
	}
	o.d.RD2(ins.X, ins.Y)
	if aligned {
		o.d.DD(align.X, ins.X)
		o.d.DD(align.Y, ins.Y)
	}
	o.d.BE(0, 0, extrusion(&t.Common))
	o.d.BT(0)
	if df&txNoOblique == 0 {
		o.d.RD(0)
	}
	if df&txNoRotation == 0 {
		o.d.RD(t.Rotation)
	}
	o.d.RD(t.Height)
	if df&txNoWidth == 0 {
		o.d.RD(t.WidthFactor)
	}
	e.tv(o, t.Content)
	if df&txNoMirror == 0 {
		o.d.BS(0)
	}
	if df&txNoHAlign == 0 {
		o.d.BS(i16(t.Align))
	}
	if df&txNoVAlign == 0 {
		o.d.BS(i16(t.VAlign))
	}
	o.ref(hStyle)
}

func (e *encoder) mtext(o *object, t *cad.MText) {
	z := extrusion(&t.Common)
	o.d.BD3(t.At.X, t.At.Y, 0)
	o.d.BD3(0, 0, z)
	o.d.BD3(math.Cos(t.Rotation), math.Sin(t.Rotation), 0)
	o.d.BD(0) // reference rectangle width
	if e.v >= v2007 {
		o.d.BD(0) // and height
	}
	o.d.BD(t.Height)
	o.d.BS(i16(t.Attach))
	o.d.BS(1) // drawing direction
	o.d.BD(0)
	o.d.BD(0) // extents
	e.tv(o, t.Content)
}

// Insert scale codes.
const (
	scaleXDD = iota
	scaleOneDD
	scaleUniform
	scaleOne
)

func (e *encoder) insert(h uint64, in *cad.Insert, c common, k int) {
	minsert := in.Columns != 0 || in.Rows != 0 || in.ColumnGap != 0 || in.RowGap != 0
	typ := typInsert
	if minsert {
		typ = typMinsert
	}
	o := e.object(h, typ)
	e.entity(o, c)
	o.d.BD3(in.At.X, in.At.Y, 0)
	switch s := in.Scale; {
	case s.X == 1 && s.Y == 1:
		o.d.BB(scaleOne)
	case s.X == s.Y:
		o.d.BB(scaleUniform)
		o.d.RD(s.X)
	case s.X == 1:
		o.d.BB(scaleOneDD)
		o.d.DD(s.Y, 1)
		o.d.DD(1, 1)
	default:
		o.d.BB(scaleXDD)
		o.d.RD(s.X)
		o.d.DD(s.Y, s.X)
		o.d.DD(s.X, s.X)
	}
	o.d.BD(in.Rotation)
	o.d.BD3(0, 0, extrusion(&in.Common))
	has := len(in.Attribs) > 0
	o.d.B(has)
	if has && e.v >= v2004 {
		o.d.BL(n32(len(in.Attribs)))
	}
	if minsert {
		o.d.BS(i16(in.Columns))
		o.d.BS(i16(in.Rows))
		o.d.BD(in.ColumnGap)
		o.d.BD(in.RowGap)
	}
	o.ref(e.blocks[strings.ToUpper(in.Block)])
	if !has {
		return
	}
	hs := make([]uint64, len(in.Attribs))
	for i := range hs {
		hs[i] = e.alloc()
	}
	seq := e.alloc()
	if e.v >= v2004 {
		for _, a := range hs {
			o.ref(a)
		}
	} else {
		o.ref(hs[0])
		o.ref(hs[len(hs)-1])
	}
	o.ref(seq)
	for i, a := range in.Attribs {
		ac := common{
			mode: modeOwned, owner: h, layer: e.layers[a.Layer],
			color: a.Color, rgb: a.RGB, invisible: a.Invisible,
		}
		if i > 0 {
			ac.prev = hs[i-1]
		}
		if i+1 < len(hs) {
			ac.next = hs[i+1]
		}
		e.attrib(hs[i], a, ac, k+i)
	}
	e.seqend(seq, h, c.layer)
}

// Attribute types (R2018).
const (
	attSingle    = 1
	attMultiline = 2
)

func (e *encoder) attrib(h uint64, t *cad.Text, c common, k int) {
	o := e.object(h, typAttrib)
	e.entity(o, c)
	multi := e.v >= v2018 && k%2 == 1
	own := *t
	if multi {
		own.Content = ""
	}
	e.text(o, &own, k)
	if e.v >= v2010 {
		o.d.RC(0) // version
	}
	if e.v >= v2018 {
		if !multi {
			o.d.RC(attSingle)
		} else {
			// The text lives in an embedded MTEXT, with entity data of its
			// own from the entity mode on.
			o.d.RC(attMultiline)
			e.entityMode(o, common{mode: modeModel, layer: c.layer}, k)
			e.mtext(o, &cad.MText{At: t.At, Height: t.Height, Content: t.Content})
			if k%4 == 1 {
				o.d.BS(2) // annotative data
				o.d.RC(0)
				o.d.RC(0)
				o.ref(hStyle)
				o.d.BS(0)
			} else {
				o.d.BS(0)
			}
		}
	}
	e.tv(o, "TAG")
	o.d.BS(0) // field length
	if t.Invisible {
		o.d.RC(1)
	} else {
		o.d.RC(0)
	}
}

// Hatch loop flags and edge types.
const (
	loopPolyline = 2
	edgeLine     = 1
)

func (e *encoder) hatch(o *object, t *cad.Hatch, k int) {
	if e.v >= v2004 {
		o.d.BL(0) // not a gradient
		o.d.BL(0)
		o.d.BD(0)
		o.d.BD(0)
		o.d.BL(0)
		o.d.BD(0)
		if k%2 == 1 {
			o.d.BL(1)
			o.d.BD(0)
			e.cmc(o, 5, nil, k)
		} else {
			o.d.BL(0)
		}
		e.tv(o, "")
	}
	o.d.BD(0) // elevation
	o.d.BD3(0, 0, extrusion(&t.Common))
	if t.Solid {
		e.tv(o, "SOLID")
	} else {
		e.tv(o, "ANSI31")
	}
	o.d.B(t.Solid)
	o.d.B(false) // associative
	loops := subpaths(t.Boundary)
	o.d.BL(n32(len(loops)))
	for i, l := range loops {
		if l.curved {
			// Curves are flattened to the line through their ends: a hatch
			// read from DXF keeps its arcs as Bézier curves, which have no
			// DWG form, and only the polyline loops need to round-trip.
			o.d.BL(0)
			o.d.BL(n32(len(l.pts)))
			for j, p := range l.pts {
				q := l.pts[(j+1)%len(l.pts)]
				o.d.RC(edgeLine)
				o.d.RD2(p.X, p.Y)
				o.d.RD2(q.X, q.Y)
			}
		} else {
			o.d.BL(loopPolyline)
			o.d.B(false) // no bulges
			o.d.B(true)  // closed
			o.d.BL(n32(len(l.pts)))
			for _, p := range l.pts {
				o.d.RD2(p.X, p.Y)
			}
		}
		// The objects the loop was picked from: one for some loops.
		n := (i + k) % 2
		o.d.BL(n32(n))
		for range n {
			o.ref(hStyle)
		}
	}
	o.d.BS(0) // style
	o.d.BS(1) // pattern type
}

type loop struct {
	pts    []cad.Point
	curved bool
}

// subpaths splits a hatch boundary into its loops' corners, undoing what
// cad.AddPolyline does to a closed polyline: the line back to the start
// is dropped.
func subpaths(p vector.Path) []loop {
	var out []loop
	var cur *loop
	for _, s := range p.Segs {
		switch s.Op {
		case vector.MoveTo:
			out = append(out, loop{pts: []cad.Point{s.P[0]}})
			cur = &out[len(out)-1]
		case vector.LineTo:
			if cur != nil {
				cur.pts = append(cur.pts, s.P[0])
			}
		case vector.CubicTo:
			if cur != nil {
				cur.pts = append(cur.pts, s.P[2])
				cur.curved = true
			}
		case vector.Close:
			if cur != nil && len(cur.pts) > 1 && cur.pts[len(cur.pts)-1] == cur.pts[0] {
				cur.pts = cur.pts[:len(cur.pts)-1]
			}
			cur = nil
		}
	}
	return out
}

// mesh is a POLYLINE flagged as a polygon mesh, which the reader decodes
// and then leaves out.
type mesh struct{ cad.Polyline }

const polylineMesh = 16

// extras writes the objects Drawing.Extras asks for, in model space.
func (e *encoder) extras() {
	c := common{mode: modeModel, layer: e.layers["0"], color: cad.ByLayer}
	for _, typ := range []int{typFace3D, typPolyPface, typPoint, clsWall} {
		o := e.object(e.alloc(), typ)
		e.entity(o, c)
		o.d.BD3(1, 2, 3) // whatever the entity's fields are
	}
	o := e.object(e.alloc(), clsPlaceholder)
	e.nonEntity(o, 0, 0)

	// A LINE whose data stops before its fields.
	o = e.object(e.alloc(), typLine)
	e.entity(o, c)

	o = e.object(e.alloc(), clsUndeclared)
	e.nonEntity(o, 0, 0)
}
