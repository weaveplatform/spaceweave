package cad

import (
	"errors"
	"strings"

	"github.com/weaveplatform/spaceweave/internal/vector"
)

// SkipBlock, returned by a Walk visitor for an Insert or Dimension, leaves
// that block reference's contents out of the walk without stopping it. It
// is a signal, like fs.SkipDir, hence the name.
//
//nolint:errname,staticcheck // see above
var SkipBlock = errors.New("skip this block")

// maxVisits bounds a walk on its own, for callers that, unlike Draw, add
// nothing per entity to stop at MaxItems: nested arrays of empty blocks
// would otherwise run for as long as their product. A variable so tests can
// reach it without building millions of entities.
var maxVisits = 4 * MaxItems

// Placed is an entity where a walk found it.
type Placed struct {
	Entity Entity
	// M maps the entity's own coordinates into the drawing: every insert
	// it sits in and, for entities stored in object coordinates, its
	// mirroring. For an Insert or Dimension it maps the block's
	// coordinates (base point included) instead, one visit per MINSERT
	// array cell.
	M vector.Affine
	// Layer is the layer it shows on: an entity on layer 0 inside a block
	// takes the insert's. Nil when the drawing doesn't define the layer.
	Layer *Layer
	Color vector.Color
	// Parent is the Insert or Dimension visit it sits in, nil in model
	// space. Owner is the Insert an ATTRIB belongs to; attributes are
	// stored in drawing coordinates, so their Parent is the insert's.
	Parent *Placed
	Owner  *Insert
}

// Walked is what a walk left out or chose.
type Walked struct {
	// Paper says model space was empty and the paper layout was walked.
	Paper bool
	// Loops counts block references inside themselves; Deep, references
	// nested more than MaxDepth deep. Neither is expanded.
	Loops, Deep int
}

type walker struct {
	doc    *Document
	visit  func(*Placed) error
	stack  []string // the blocks open, to catch a block inside itself
	visits int
	stats  Walked
}

// context is what an entity inherits from the inserts it sits in.
type context struct {
	m      vector.Affine
	layer  *Layer // the insert's layer, for entities on layer 0 in a block
	color  vector.Color
	parent *Placed
}

// Walk visits every entity that shows, in file order, expanding block
// references depth first: model space, or the paper layout when model space
// has nothing visible. Entities that are invisible or on a layer that is off
// or frozen are not visited, and neither is anything in an insert on such a
// layer. Each Insert (per array cell) and Dimension is visited before its
// contents; the visitor returns SkipBlock to leave them out. Any other error
// stops the walk and is returned.
func Walk(doc *Document, visit func(*Placed) error) (Walked, error) {
	w := &walker{doc: doc, visit: visit}
	ents := doc.Model
	if !anyVisible(ents) {
		ents = doc.Paper
		w.stats.Paper = len(ents) > 0
	}
	root := context{m: vector.Identity, layer: doc.Layers["0"], color: vector.Black}
	err := w.entities(ents, root)
	return w.stats, err
}

func anyVisible(ents []Entity) bool {
	for _, e := range ents {
		if !e.Base().Invisible {
			return true
		}
	}
	return false
}

func (w *walker) entities(ents []Entity, ctx context) error {
	for _, e := range ents {
		if err := w.entity(e, ctx, nil); err != nil {
			return err
		}
	}
	return nil
}

// style resolves an entity's layer and colour. Entities on layer 0 inside a
// block take the insert's layer; BYBLOCK takes the insert's colour.
func (w *walker) style(c *Common, ctx context) (*Layer, vector.Color, bool) {
	layer := w.doc.Layers[c.Layer]
	if layer == nil || c.Layer == "0" {
		layer = ctx.layer
	}
	if layer != nil && (layer.Off || layer.Frozen) {
		return nil, vector.Color{}, false
	}
	switch {
	case c.RGB != nil:
		return layer, *c.RGB, true
	case c.Color == ByBlock:
		return layer, ctx.color, true
	case c.Color == ByLayer || c.Color < 0 || c.Color > 255:
		if layer == nil {
			return layer, vector.Black, true
		}
		if layer.RGB != nil {
			return layer, *layer.RGB, true
		}
		return layer, ACI(layer.Color), true
	default:
		return layer, ACI(c.Color), true
	}
}

// ocs maps an entity's object coordinates into the drawing. With the
// extrusion pointing down Z, CAD's arbitrary-axis rule makes the object's
// X axis the drawing's -X.
func ocs(c *Common, m vector.Affine) vector.Affine {
	if c.Mirrored {
		return vector.Scale(-1, 1).Then(m)
	}
	return m
}

func (w *walker) entity(e Entity, ctx context, owner *Insert) error {
	c := e.Base()
	if c.Invisible {
		return nil
	}
	layer, col, ok := w.style(c, ctx)
	if !ok {
		return nil
	}
	if w.visits++; w.visits > maxVisits {
		return ErrTooBig
	}
	m := ctx.m
	switch e := e.(type) {
	case *Insert:
		return w.insert(e, layer, col, ctx)
	case *Dimension:
		return w.block(e, e.Block, vector.Identity, layer, col, ctx)
	case *Line, *Spline:
		// Stored in world coordinates whatever their extrusion.
	default:
		m = ocs(c, m)
	}
	p := Placed{Entity: e, M: m, Layer: layer, Color: col, Parent: ctx.parent, Owner: owner}
	if err := w.visit(&p); err != nil && !errors.Is(err, SkipBlock) {
		return err
	}
	return nil
}

func (w *walker) insert(in *Insert, layer *Layer, col vector.Color, ctx context) error {
	sx, sy := in.Scale.X, in.Scale.Y
	if sx == 0 {
		sx = 1
	}
	if sy == 0 {
		sy = 1
	}
	cols, rows := max(in.Columns, 1), max(in.Rows, 1)
	if cols*rows > MaxItems {
		return ErrTooBig
	}
	for r := range rows {
		for c := range cols {
			// Array spacing is measured along the insert's rotated axes, but
			// isn't scaled by it.
			m := vector.Scale(sx, sy).
				Then(vector.Translate(float64(c)*in.ColumnGap, float64(r)*in.RowGap)).
				Then(vector.Rotate(in.Rotation)).
				Then(vector.Translate(in.At.X, in.At.Y))
			if in.Mirrored {
				// The insertion point is in object coordinates too, so the
				// mirror comes after the placement, not before it.
				m = m.Then(vector.Scale(-1, 1))
			}
			if err := w.block(in, in.Block, m, layer, col, ctx); err != nil {
				return err
			}
		}
	}
	// Attributes are stored where they sit in the drawing, not in the block.
	for _, a := range in.Attribs {
		if err := w.entity(a, ctx, in); err != nil {
			return err
		}
	}
	return nil
}

// block visits a block reference and then its entities under m (placing
// the block's base point at the origin first) and then ctx.m.
func (w *walker) block(
	ref Entity,
	name string,
	m vector.Affine,
	layer *Layer,
	col vector.Color,
	ctx context,
) error {
	b := w.doc.Blocks[name]
	if b == nil {
		return nil
	}
	for _, open := range w.stack {
		if strings.EqualFold(open, name) {
			w.stats.Loops++
			return nil
		}
	}
	if len(w.stack) >= MaxDepth {
		w.stats.Deep++
		return nil
	}
	if w.visits++; w.visits > maxVisits {
		return ErrTooBig
	}
	inner := vector.Translate(-b.Base.X, -b.Base.Y).Then(m).Then(ctx.m)
	p := &Placed{Entity: ref, M: inner, Layer: layer, Color: col, Parent: ctx.parent}
	if err := w.visit(p); err != nil {
		if errors.Is(err, SkipBlock) {
			return nil
		}
		return err
	}
	w.stack = append(w.stack, name)
	defer func() { w.stack = w.stack[:len(w.stack)-1] }()
	return w.entities(b.Entities, context{m: inner, layer: layer, color: col, parent: p})
}
