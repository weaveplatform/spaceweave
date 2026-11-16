// Package cadfront finds wall, opening, label and furniture evidence in a
// CAD drawing's entities, for floorgraph to build a trace from.
//
// The rasterised image keeps the geometry but loses what the drawing says
// about it: which layer a line is on, which block a symbol is, what the text
// reads. This front end reads those instead of pixels. Layer and block names
// say what is a wall, a door, a window or a desk; where they say nothing,
// pairs of parallel lines a wall's thickness apart are walls and quarter
// arcs a door's width across are doors.
//
// Everything is found in drawing coordinates, measured in metres through
// the drawing's units, and handed over in source-image pixels through the
// same transform the rasteriser used, so it lands on the picture.
package cadfront

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/dxf"
	"github.com/weaveplatform/spaceweave/internal/floorgraph"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// Options say where the drawing landed in the source image.
type Options struct {
	// View maps drawing coordinates (model space, as cad.Draw emits them)
	// to source-image pixels: vector.Raster.View of the image traced.
	View vector.Affine
	// Size is the source image's width and height in pixels.
	Size [2]int
	// PxPerMetre is the image's scale, 0 when the drawing's units are
	// unknown. Exact says it comes from units the drawing states rather
	// than ones guessed from its size.
	PxPerMetre float64
	Exact      bool
	Title      string
}

// ErrView is returned for a view that doesn't map the drawing onto an
// image (zero or squashed flat).
var ErrView = errors.New("cadfront: the view transform is degenerate")

// Read finds the evidence in a drawing. It is deterministic: the same
// document and options give the same Input.
func Read(doc *cad.Document, o Options) (floorgraph.Input, error) {
	px := o.View.Factor()
	if !(px > 0) || math.IsInf(px, 0) {
		return floorgraph.Input{}, ErrView
	}
	r := &reader{
		o: o, px: px, byPlaced: map[*cad.Placed]*node{}, owners: map[*cad.Insert][]*node{},
	}
	walked, err := cad.Walk(doc, r.visit)
	if err != nil {
		return floorgraph.Input{}, fmt.Errorf("walk the drawing: %w", err)
	}
	if walked.Paper {
		r.note("Model space is empty; read the paper layout instead.")
	}
	r.run()
	return r.in, nil
}

// ReadFile reads a DXF file. With a zero View it frames the drawing the
// way the converter does (cad.Draw, then vector.Rasterize with default
// options) and takes the view, size and scale from that.
func ReadFile(path string, o Options) (floorgraph.Input, error) {
	f, err := os.Open(path) //nolint:gosec // the caller names the file to read
	if err != nil {
		return floorgraph.Input{}, fmt.Errorf("open drawing: %w", err)
	}
	defer func() { _ = f.Close() }()
	doc, err := dxf.Read(f)
	if err != nil {
		return floorgraph.Input{}, fmt.Errorf("read DXF: %w", err)
	}
	if o.View == (vector.Affine{}) {
		framed, err := Frame(doc, vector.Options{})
		if err != nil {
			return floorgraph.Input{}, err
		}
		framed.Title = o.Title
		o = framed
	}
	if o.Title == "" {
		o.Title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	return Read(doc, o)
}

// Frame draws and rasterises a document as the converter does and returns
// the options that place it on that image.
func Frame(doc *cad.Document, ro vector.Options) (Options, error) {
	d, err := cad.Draw(doc)
	if err != nil {
		return Options{}, fmt.Errorf("draw: %w", err)
	}
	ras, err := vector.Rasterize(d, ro)
	if err != nil {
		return Options{}, fmt.Errorf("rasterise: %w", err)
	}
	o := Options{View: ras.View(), Size: [2]int{ras.Image.Bounds().Dx(), ras.Image.Bounds().Dy()}}
	if d.Unit > 0 && !d.Paper {
		o.PxPerMetre, o.Exact = ras.PxPerUnit/d.Unit, !d.Assumed
	}
	return o, nil
}

// leaf is an entity that isn't a block reference, where the walk found it.
type leaf struct {
	e     cad.Entity
	m     vector.Affine
	layer string
	role  role // the layer's
	node  *node
	owner *cad.Insert // for attributes
	box   vector.Rect // in drawing coordinates
	used  bool
	// doorLeaf marks a line found to be a loose door's leaf.
	doorLeaf bool
}

// node is one block reference (one cell of an array).
type node struct {
	ins    *cad.Insert
	m      vector.Affine // block coordinates to drawing
	layer  string
	parent *node
	role   role
	furn   string // furniture type, for roleFurniture
	leaves []int  // its own entities, not its children's
	kids   []*node
	ext    vector.Rect // everything in it, in drawing coordinates
	// holdsFurniture says a block reference inside it is furniture.
	holdsFurniture bool
	// top is the outermost reference, this one or one it sits in, that is
	// a door, window or piece of furniture: everything inside belongs to
	// that and nothing else. Nil when there is none.
	top *node
}

type reader struct {
	o        Options
	px       float64 // image pixels per drawing unit
	unit     float64 // metres per drawing unit, measured or estimated
	known    bool    // unit comes from the drawing's scale, not an estimate
	leaves   []leaf
	nodes    []*node
	byPlaced map[*cad.Placed]*node
	// owners are an insert's references, one per array cell, for telling
	// whether its attributes belong to a door or a desk.
	owners map[*cad.Insert][]*node
	walls  []wall
	in     floorgraph.Input
}

// layerName is the layer an entity's name is judged by: its own, unless
// that is layer 0 inside a block, which stands for the insert's. A layer
// missing from the layer table still has its name (Placed.Layer then falls
// back to the insert's, which is right for visibility, not for naming).
func layerName(p *cad.Placed) string {
	own := p.Entity.Base().Layer
	if (own == "0" || own == "") && p.Layer != nil {
		return p.Layer.Name
	}
	return own
}

func (r *reader) visit(p *cad.Placed) error {
	parent := r.byPlaced[p.Parent]
	switch e := p.Entity.(type) {
	case *cad.Dimension:
		// Dimension blocks are measurements: their lines aren't walls and
		// their numbers aren't room names.
		return cad.SkipBlock
	case *cad.Insert:
		n := &node{ins: e, m: p.M, layer: layerName(p), parent: parent}
		n.role = blockRole(e.Block, n.layer)
		if n.role == roleFurniture {
			n.furn = furnitureType(e.Block)
		}
		r.byPlaced[p] = n
		r.nodes = append(r.nodes, n)
		r.owners[e] = append(r.owners[e], n)
		if parent != nil {
			parent.kids = append(parent.kids, n)
		}
		return nil
	}
	if len(r.leaves) >= cad.MaxItems {
		return cad.ErrTooBig
	}
	name := layerName(p)
	r.leaves = append(r.leaves, leaf{
		e: p.Entity, m: p.M, layer: name, role: layerRole(name), node: parent, owner: p.Owner,
	})
	if parent != nil {
		parent.leaves = append(parent.leaves, len(r.leaves)-1)
	}
	return nil
}

// metres converts a length in metres to drawing units.
func (r *reader) metres(m float64) float64 { return m / r.unit }

func (r *reader) toPx(p point) point { return r.o.View.Apply(p) }

func (r *reader) note(format string, args ...any) {
	r.in.Notes = append(r.in.Notes, fmt.Sprintf(format, args...))
}

func (r *reader) run() {
	r.in.Title, r.in.Size = r.o.Title, r.o.Size
	r.extents()
	// Names first: the scale estimate looks for doors by name, and the
	// swing test that finds unnamed door blocks needs the scale.
	r.classify()
	r.scale()
	r.classify()
	r.findWalls()
	r.findOpenings()
	r.emitWalls()
	r.findLabels()
	r.findFurniture()
	r.coverage()
	r.sortOutput()
}

// extents finds every leaf's box and every node's, children before parents.
func (r *reader) extents() {
	for i := range r.leaves {
		l := &r.leaves[i]
		for _, p := range r.points(i) {
			l.box = l.box.Extend(p)
		}
	}
	for i := len(r.nodes) - 1; i >= 0; i-- {
		n := r.nodes[i]
		for _, li := range n.leaves {
			n.ext = n.ext.Union(r.leaves[li].box)
		}
		if n.parent != nil {
			n.parent.ext = n.parent.ext.Union(n.ext)
			n.parent.holdsFurniture = n.parent.holdsFurniture || n.holdsFurniture ||
				n.role == roleFurniture
		}
	}
}

// points samples a leaf in drawing coordinates.
func (r *reader) points(i int) []point {
	l := &r.leaves[i]
	var out []point
	if lines := cad.TextLines(l.e, l.m); lines != nil {
		for _, t := range lines {
			b := t.Bounds()
			out = append(out, b.Min, b.Max)
		}
		return out
	}
	for _, p := range samples(l.e) {
		out = append(out, l.m.Apply(p))
	}
	return out
}

// scale settles the working unit: the drawing's own when the caller knows
// the scale, otherwise an estimate from door swings or wall thicknesses.
func (r *reader) scale() {
	if r.o.PxPerMetre > 0 {
		r.unit, r.known = r.px/r.o.PxPerMetre, true
		p := r.o.PxPerMetre
		r.in.Scale = floorgraph.Scale{PxPerMetre: [2]float64{p, p}, Exact: r.o.Exact}
		if r.o.Exact {
			r.in.Scale.Evidence = "from the drawing's units"
		} else {
			r.in.Scale.Evidence = "assumed from the drawing's size"
		}
		return
	}
	if rad := r.doorRadius(); rad > 0 {
		r.unit = doorLeaf / rad
		r.note(
			"The drawing's units are unknown; sizes were judged by taking door swings as %.1f m, "+
				"and no scale is given.",
			doorLeaf,
		)
		return
	}
	if g := r.gapMode(); g > 0 {
		r.unit = typicalWall / g
		r.note(
			"The drawing's units are unknown; sizes were judged by taking the commonest gap between "+
				"parallel lines as a %.2f m wall, and no scale is given.",
			typicalWall,
		)
		return
	}
	var all vector.Rect
	for i := range r.leaves {
		all = all.Union(r.leaves[i].box)
	}
	long := 1.0
	if !all.Empty() {
		long = math.Max(math.Max(all.Size().X, all.Size().Y), 1e-9)
	}
	r.unit = assumedSpan / long
	r.note(
		"The drawing's units are unknown and nothing in it gives a size; thresholds assume it is "+
			"%.0f m across, and no scale is given.",
		assumedSpan,
	)
}

// Sizes the estimates above assume: a common door leaf, a common
// partition, and a mid-sized office floor.
const (
	doorLeaf    = 0.9
	typicalWall = 0.2
	assumedSpan = 30.0
)

// classify decides, outermost first, which block references are doors,
// windows and furniture. A reference whose name says nothing is a door when
// it is small and holds one or two door-sized swing arcs of its own.
func (r *reader) classify() {
	for _, n := range r.nodes { // parents come before their children
		n.top = nil
		if n.parent != nil && n.parent.top != nil {
			n.top = n.parent.top
			continue
		}
		if n.role == roleFurniture && n.holdsFurniture {
			// A workstation of a desk and a chair, or a cluster of
			// workstations: place the pieces, not the group.
			continue
		}
		if n.role == roleNone && r.swingBlock(n) {
			n.role = roleDoor
		}
		if n.role != roleNone {
			n.top = n
		}
	}
}

// consumed reports whether a leaf belongs to a door, window or piece of
// furniture, directly or as one of its attributes.
func (r *reader) consumed(l *leaf) bool {
	if l.node != nil && l.node.top != nil {
		return true
	}
	if l.owner != nil {
		for _, n := range r.owners[l.owner] {
			if n.top != nil {
				return true
			}
		}
	}
	return false
}

// use marks everything in a block reference as explained.
func (r *reader) use(n *node) {
	for _, li := range n.leaves {
		r.leaves[li].used = true
	}
	for _, k := range n.kids {
		r.use(k)
	}
}

func (r *reader) coverage() {
	used := 0
	for i := range r.leaves {
		if r.leaves[i].used {
			used++
		}
	}
	r.in.Coverage = -1
	if len(r.leaves) > 0 {
		r.in.Coverage = float64(used) / float64(len(r.leaves))
	}
}
