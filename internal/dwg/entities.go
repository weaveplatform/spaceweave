package dwg

import (
	"math"

	"github.com/weaveplatform/spaceweave/internal/cad"
)

// rec is a decoded entity and the links that place it: its owner, the
// block it references, the children it owns. Field layouts follow the ODA
// specification section 20.4, cross-checked against ACadSharp's
// DwgObjectReader (MIT).
type rec struct {
	handle uint64
	typ    int
	ent    entCommon
	e      cad.Entity // nil for parts of other entities
	// block is the block record an INSERT or DIMENSION draws.
	block uint64
	// Children (an INSERT's attributes, a POLYLINE's vertices): the owned
	// list from R2004, the first and last of a chain before.
	owned       []uint64
	first, last uint64
	// name is a BLOCK entity's block name.
	name string
	// vertex is a VERTEX's point; vflags its flags.
	vertex cad.Vertex
	vflags int
}

// point2 drops Z: plans are drawn flat.
func point2(v vec3) cad.Point { return cad.Point{X: v.X, Y: v.Y} }

// common makes the entity's cad.Common. The layer is filled in once all
// layers are known.
func (r *rec) common(extrusion vec3) cad.Common {
	return cad.Common{
		Color: r.ent.color, RGB: r.ent.rgb, Invisible: r.ent.invisible,
		Mirrored: extrusion.Z < 0,
	}
}

// decodeEntity decodes an entity of a type the reader draws or needs, by
// its fixed type or, for a class number, its DXF name.
//
//nolint:gocyclo,cyclop // one case per entity type
func (d *decoder) decodeEntity(o *obj, kind string) *rec {
	r := &rec{handle: o.handle, typ: o.typ, ent: o.entity()}
	switch kind {
	case "LINE":
		r.e = d.line(o, r)
	case "CIRCLE", "ARC":
		c := o.d.BD3()
		rad := o.d.BD()
		o.d.BT()
		x := o.d.BE()
		if kind == "CIRCLE" {
			r.e = &cad.Circle{Common: r.common(x), Center: point2(c), Radius: rad}
			break
		}
		a0, a1 := o.d.BD(), o.d.BD()
		r.e = &cad.Arc{Common: r.common(x), Center: point2(c), Radius: rad, Start: a0, End: a1}
	case "ELLIPSE":
		c, major := o.d.BD3(), o.d.BD3()
		o.d.BD3() // extrusion: an ellipse is in world coordinates whatever it is
		ratio, t0, t1 := o.d.BD(), o.d.BD(), o.d.BD()
		r.e = &cad.Ellipse{
			Common: r.common(vec3{Z: 1}), Center: point2(c), Major: point2(major),
			Ratio: ratio, Start: t0, End: t1,
		}
	case "LWPOLYLINE":
		r.e = d.lwpolyline(o, r)
	case "POLYLINE2D":
		// A mesh comes back nil, which mustn't become a non-nil interface.
		if pl := d.polyline2D(o, r); pl != nil {
			r.e = pl
		}
	case "POLYLINE3D":
		r.e = d.polyline3D(o, r)
	case "VERTEX2D":
		r.vflags = int(o.d.RC())
		p := o.d.BD3()
		if w := o.d.BD(); w >= 0 {
			o.d.BD() // end width
		}
		r.vertex = cad.Vertex{P: point2(p), Bulge: o.d.BD()}
	case "VERTEX3D":
		r.vflags = int(o.d.RC())
		r.vertex = cad.Vertex{P: point2(o.d.BD3())}
	case "SPLINE":
		r.e = d.spline(o, r)
	case "TEXT":
		r.e, _ = d.text(o, r)
	case "ATTRIB":
		r.e = d.attrib(o, r)
	case "MTEXT":
		r.e = d.mtext(o, r)
	case "INSERT", "MINSERT":
		r.e = d.insert(o, r, kind == "MINSERT")
	case "SOLID":
		o.d.BT()
		o.d.BD() // elevation
		var s cad.Solid
		for i := range s.Corners {
			s.Corners[i] = point2(o.d.RD2())
		}
		s.Common = r.common(o.d.BE())
		r.e = &s
	case "HATCH":
		r.e = d.hatch(o, r)
	case "DIMENSION":
		// The dimension's own fields don't matter: it is drawn through its
		// block, whose handle follows the dimension style's in the handle
		// stream, which the common data has already reached.
		o.ref()
		r.block = o.ref()
		r.e = &cad.Dimension{Common: r.common(vec3{Z: 1})}
	case "BLOCK":
		r.name = o.tv()
	}
	return r
}

func (d *decoder) line(o *obj, r *rec) *cad.Line {
	flat := o.d.B()
	x0 := o.d.RD()
	x1 := o.d.DD(x0)
	y0 := o.d.RD()
	y1 := o.d.DD(y0)
	if !flat {
		z := o.d.RD()
		o.d.DD(z)
	}
	o.d.BT()
	return &cad.Line{
		Common: r.common(o.d.BE()),
		A:      cad.Point{X: x0, Y: y0},
		B:      cad.Point{X: x1, Y: y1},
	}
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

func (d *decoder) lwpolyline(o *obj, r *rec) *cad.Polyline {
	flags := int(o.d.BS())
	pl := &cad.Polyline{Closed: flags&lwClosed != 0}
	if flags&lwWidth != 0 {
		pl.Width = o.d.BD()
	}
	if flags&lwElevation != 0 {
		o.d.BD()
	}
	if flags&lwThickness != 0 {
		o.d.BD()
	}
	x := vec3{Z: 1}
	if flags&lwExtrusion != 0 {
		x = o.d.BD3()
	}
	pl.Common = r.common(x)
	// The smallest vertex is two 2-bit "same as before" codes.
	n := o.d.count(o.d.BL(), 4)
	var bulges, ids, widths int
	if flags&lwBulges != 0 {
		bulges = o.d.count(o.d.BL(), 2)
	}
	if flags&lwIDs != 0 {
		ids = o.d.count(o.d.BL(), 2)
	}
	if flags&lwWidths != 0 {
		widths = o.d.count(o.d.BL(), 4)
	}
	pl.Vertices = make([]cad.Vertex, n)
	for i := range n {
		if i == 0 {
			pl.Vertices[i].P = point2(o.d.RD2())
			continue
		}
		prev := pl.Vertices[i-1].P
		pl.Vertices[i].P = cad.Point{X: o.d.DD(prev.X), Y: o.d.DD(prev.Y)}
	}
	for i := range bulges {
		b := o.d.BD()
		if i < n {
			pl.Vertices[i].Bulge = b
		}
	}
	for range ids {
		o.d.BL()
	}
	for range widths {
		pl.Width = math.Max(pl.Width, math.Max(o.d.BD(), o.d.BD()))
	}
	return pl
}

// POLYLINE flags.
const (
	plClosed   = 1
	plMesh     = 16
	plPolyface = 64
	vxFrame    = 16 // a spline frame's control point, not on the curve
)

func (d *decoder) polyline2D(o *obj, r *rec) *cad.Polyline {
	flags := int(o.d.BS())
	o.d.BS() // curve type
	w0, w1 := o.d.BD(), o.d.BD()
	o.d.BT()
	o.d.BD() // elevation
	pl := &cad.Polyline{
		Common: r.common(o.d.BE()),
		Closed: flags&plClosed != 0,
		Width:  math.Max(w0, w1),
	}
	d.children(o, r)
	if flags&(plMesh|plPolyface) != 0 {
		return nil
	}
	return pl
}

func (d *decoder) polyline3D(o *obj, r *rec) *cad.Polyline {
	o.d.RC() // spline flags
	closed := o.d.RC()&1 != 0
	pl := &cad.Polyline{Common: r.common(vec3{Z: 1}), Closed: closed}
	d.children(o, r)
	return pl
}

// children reads the handles of the attributes or vertices an INSERT or
// POLYLINE owns, and its SEQEND's.
func (d *decoder) children(o *obj, r *rec) {
	n := int32(0)
	if o.ver >= v2004 {
		n = o.d.BL()
	}
	if o.ver >= v2004 {
		r.owned = o.refs(n)
	} else {
		r.first, r.last = o.ref(), o.ref()
	}
	o.ref() // SEQEND
}

func (d *decoder) spline(o *obj, r *rec) *cad.Spline {
	s := &cad.Spline{Common: r.common(vec3{Z: 1})}
	scenario := o.d.BL()
	if o.ver >= v2013 {
		flags1 := o.d.BL()
		knotParam := o.d.BL()
		s.Closed = flags1&4 != 0
		// From R2013 the flags decide: fit data unless the knots are
		// custom or the "use knot parameter" flag is clear.
		scenario = 2
		if knotParam == 15 || flags1&8 == 0 {
			scenario = 1
		}
	}
	s.Degree = int(o.d.BL())
	var knots, ctrl, fit int
	weighted := false
	switch scenario {
	case 1:
		o.d.B() // rational
		if o.d.B() {
			s.Closed = true
		}
		o.d.B() // periodic
		o.d.BD()
		o.d.BD() // knot and control tolerances
		knots = o.d.count(o.d.BL(), 2)
		ctrl = o.d.count(o.d.BL(), 6)
		weighted = o.d.B()
	case 2:
		o.d.BD() // fit tolerance
		o.d.BD3()
		o.d.BD3() // start and end tangents
		fit = o.d.count(o.d.BL(), 6)
	default:
		o.d.fail(errShort)
	}
	s.Knots = make([]float64, 0, knots)
	for range knots {
		s.Knots = append(s.Knots, o.d.BD())
	}
	s.Control = make([]cad.Point, 0, ctrl)
	for range ctrl {
		s.Control = append(s.Control, point2(o.d.BD3()))
		if weighted {
			s.Weights = append(s.Weights, o.d.BD())
		}
	}
	for range fit {
		s.Fit = append(s.Fit, point2(o.d.BD3()))
	}
	if len(s.Knots) == 0 {
		s.Knots = nil
	}
	if len(s.Control) == 0 {
		s.Control = nil
	}
	return s
}

// TEXT data flags: a set bit means the field is left out and takes its
// default.
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

// text reads TEXT, and the same fields at the start of ATTRIB and ATTDEF.
// It returns the alignment point too, which ATTRIB needs nothing more of.
func (d *decoder) text(o *obj, r *rec) (*cad.Text, bool) {
	df := o.d.RC()
	if df&txNoElevation == 0 {
		o.d.RD()
	}
	at := point2(o.d.RD2())
	align, hasAlign := at, false
	if df&txNoAlign == 0 {
		align = cad.Point{X: o.d.DD(at.X), Y: o.d.DD(at.Y)}
		hasAlign = true
	}
	t := &cad.Text{Common: r.common(o.d.BE()), WidthFactor: 1}
	o.d.BT()
	if df&txNoOblique == 0 {
		o.d.RD()
	}
	if df&txNoRotation == 0 {
		t.Rotation = o.d.RD()
	}
	t.Height = o.d.RD()
	if df&txNoWidth == 0 {
		t.WidthFactor = o.d.RD()
	}
	t.Content = o.tv()
	if df&txNoMirror == 0 {
		o.d.BS()
	}
	if df&txNoHAlign == 0 {
		t.Align = int(o.d.BS())
	}
	if df&txNoVAlign == 0 {
		t.VAlign = int(o.d.BS())
	}
	o.ref() // text style
	t.At = at
	// As the DXF reader does: aligned (3) and fit (5) text stretch between
	// the two points, and the insertion point anchors both.
	if hasAlign && (t.Align != 0 || t.VAlign != 0) && t.Align != 3 && t.Align != 5 {
		t.At = align
	}
	return t, hasAlign
}

// Attribute types (R2018).
const (
	attMultiline         = 2
	attConstantMultiline = 4
	attInvisible         = 1 // attribute flag
)

func (d *decoder) attrib(o *obj, r *rec) *cad.Text {
	t, _ := d.text(o, r)
	if o.ver >= v2010 {
		o.d.RC() // version
	}
	if o.ver >= v2018 {
		if typ := o.d.RC(); typ == attMultiline || typ == attConstantMultiline {
			// The text is an embedded MTEXT; the ATTRIB's own is empty.
			inner := &rec{handle: o.handle, ent: o.entityMode()}
			t.Content = d.mtextBody(o, inner).Content
			if n := int(o.d.BS()); n > 0 { // annotative data
				o.d.skip(n * 8)
				o.ref()
				o.d.BS()
			}
		}
	}
	o.tv() // tag
	o.d.BS()
	if o.d.RC()&attInvisible != 0 {
		t.Invisible = true
	}
	return t
}

func (d *decoder) mtext(o *obj, r *rec) *cad.MText { return d.mtextBody(o, r) }

func (d *decoder) mtextBody(o *obj, r *rec) *cad.MText {
	at := point2(o.d.BD3())
	x := o.d.BD3()
	dir := o.d.BD3()
	o.d.BD() // reference rectangle width
	if o.ver >= v2007 {
		o.d.BD() // and height
	}
	t := &cad.MText{Common: r.common(x), At: at, Height: o.d.BD(), Attach: int(o.d.BS())}
	o.d.BS() // drawing direction
	o.d.BD()
	o.d.BD() // extents
	t.Content = o.tv()
	if dir.X != 0 || dir.Y != 0 {
		t.Rotation = math.Atan2(dir.Y, dir.X)
	}
	return t
}

// Insert scale codes.
const (
	scaleXDD     = iota // X raw, Y and Z defaulting to it
	scaleOneDD          // X 1, Y and Z defaulting to 1
	scaleUniform        // X raw, Y and Z the same
	scaleOne            // all 1
)

func (d *decoder) insert(o *obj, r *rec, minsert bool) *cad.Insert {
	at := point2(o.d.BD3())
	in := &cad.Insert{At: at, Scale: cad.Point{X: 1, Y: 1}}
	switch o.d.BB() {
	case scaleXDD:
		in.Scale.X = o.d.RD()
		in.Scale.Y = o.d.DD(in.Scale.X)
		o.d.DD(in.Scale.X)
	case scaleOneDD:
		in.Scale.Y = o.d.DD(1)
		o.d.DD(1)
	case scaleUniform:
		in.Scale.X = o.d.RD()
		in.Scale.Y = in.Scale.X
	}
	in.Rotation = o.d.BD()
	in.Common = r.common(o.d.BD3())
	hasAttribs := o.d.B()
	var n int32
	if hasAttribs && o.ver >= v2004 {
		n = o.d.BL()
	}
	if minsert {
		in.Columns, in.Rows = int(o.d.BS()), int(o.d.BS())
		in.ColumnGap, in.RowGap = o.d.BD(), o.d.BD()
	}
	r.block = o.ref()
	if hasAttribs {
		if o.ver >= v2004 {
			r.owned = o.refs(n)
		} else {
			r.first, r.last = o.ref(), o.ref()
		}
		o.ref() // SEQEND
	}
	return in
}
