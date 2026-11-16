package cad

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/weaveplatform/spaceweave/internal/vector"
)

// Limits on what Draw expands. Blocks nest (a desk block inside a cluster
// block inside a floor block), and a hostile or broken file can make that
// recursive or explosive.
const (
	MaxDepth = 16
	MaxItems = 2_000_000
)

// ErrTooBig is returned when a drawing expands to more than MaxItems.
var ErrTooBig = errors.New("the drawing is too big to render")

// patternShade is the fill for a pattern hatch: light enough not to read as
// a wall when it covers a whole floor, dark enough to show where it is.
var patternShade = vector.Color{R: 215, G: 215, B: 215}

type drawer struct {
	doc   *Document
	out   *vector.Drawing
	stack []string // the blocks being drawn, to catch a block inside itself
	loops int
	deep  int
}

// context is what an entity inherits from the inserts it sits in.
type context struct {
	m     vector.Affine
	layer *Layer // the insert's layer, for entities on layer 0 in a block
	color vector.Color
}

// Draw turns a document into a vector drawing, with its units in metres.
func Draw(doc *Document) (*vector.Drawing, error) {
	d := &drawer{doc: doc, out: &vector.Drawing{}}
	ents := doc.Model
	if !d.drawable(ents) {
		ents = doc.Paper
		if len(ents) > 0 {
			d.out.Notes = append(
				d.out.Notes,
				"Model space is empty; drew the paper layout instead.",
			)
		}
	}
	root := context{m: vector.Identity, layer: doc.Layers["0"], color: vector.Black}
	if err := d.entities(ents, root); err != nil {
		return nil, err
	}
	if d.loops > 0 {
		d.out.Notes = append(
			d.out.Notes,
			fmt.Sprintf("%d block references that contain themselves were left out.", d.loops),
		)
	}
	if d.deep > 0 {
		d.out.Notes = append(
			d.out.Notes,
			fmt.Sprintf("%d block references nested over %d deep were left out.", d.deep, MaxDepth),
		)
	}
	if doc.Skipped > 0 {
		d.out.Notes = append(
			d.out.Notes,
			fmt.Sprintf("%d objects the reader couldn't decode were left out.", doc.Skipped),
		)
	}
	d.out.Notes = append(d.out.Notes, doc.Notes...)
	d.units()
	return d.out, nil
}

// drawable reports whether any entity would show.
func (d *drawer) drawable(ents []Entity) bool {
	for _, e := range ents {
		if !e.Base().Invisible {
			return true
		}
	}
	return false
}

// buildingSizes are plausible lengths, in metres, for the long side of a
// floor plan, used to guess the units of a drawing that doesn't say.
const minBuilding, maxBuilding = 2.0, 400.0

// guesses are tried in order; millimetres first because metric
// architectural drawings, the commonest unitless ones, use them.
var guesses = []Units{Millimeters, Meters, Centimeters, Inches, Feet}

func (d *drawer) units() {
	if m := d.doc.Units.Metres(); m > 0 {
		d.out.Unit = m
		return
	}
	b, _ := vector.Extent(d.out.Items)
	if b.Empty() {
		return
	}
	size := b.Size()
	long := math.Max(size.X, size.Y)
	for _, u := range guesses {
		if l := long * u.Metres(); l >= minBuilding && l <= maxBuilding {
			d.out.Unit, d.out.Assumed = u.Metres(), true
			d.out.Notes = append(d.out.Notes, fmt.Sprintf(
				"The drawing doesn't state its units; assumed %s from its size (%.0f units across).",
				u,
				long,
			))
			return
		}
	}
	d.out.Notes = append(
		d.out.Notes,
		"The drawing doesn't state its units, and its size doesn't suggest any.",
	)
}

func (d *drawer) entities(ents []Entity, ctx context) error {
	for _, e := range ents {
		if err := d.entity(e, ctx); err != nil {
			return err
		}
	}
	return nil
}

// style resolves an entity's layer and colour. Entities on layer 0 inside a
// block take the insert's layer; BYBLOCK takes the insert's colour.
func (d *drawer) style(c *Common, ctx context) (*Layer, vector.Color, bool) {
	layer := d.doc.Layers[c.Layer]
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

func (d *drawer) add(it vector.Item) error {
	if len(d.out.Items) >= MaxItems {
		return ErrTooBig
	}
	d.out.Add(it)
	return nil
}

func (d *drawer) stroke(c vector.Color, m vector.Affine, build func(p *vector.Path)) error {
	p := &vector.Path{Stroke: true, Color: c}
	build(p)
	if len(p.Segs) == 0 {
		return nil
	}
	return d.add(p.Transformed(m))
}

//nolint:gocyclo // one case per entity type
func (d *drawer) entity(e Entity, ctx context) error {
	c := e.Base()
	if c.Invisible {
		return nil
	}
	layer, col, ok := d.style(c, ctx)
	if !ok {
		return nil
	}
	m := ocs(c, ctx.m)
	switch e := e.(type) {
	case *Line:
		return d.stroke(col, ctx.m, func(p *vector.Path) { p.Polyline([]Point{e.A, e.B}, false) })
	case *Circle:
		return d.stroke(col, m, func(p *vector.Path) { p.Circle(e.Center, e.Radius) })
	case *Arc:
		end := e.End
		for end <= e.Start {
			end += 2 * math.Pi
		}
		return d.stroke(col, m, func(p *vector.Path) {
			p.EllipseArc(e.Center, Point{X: e.Radius}, 1, e.Start, end, false)
		})
	case *Ellipse:
		end := e.End
		for end <= e.Start {
			end += 2 * math.Pi
		}
		return d.stroke(col, m, func(p *vector.Path) {
			p.EllipseArc(e.Center, e.Major, e.Ratio, e.Start, end, false)
		})
	case *Polyline:
		p := &vector.Path{Stroke: true, Color: col, Width: e.Width}
		AddPolyline(p, e.Vertices, e.Closed)
		if len(p.Segs) == 0 {
			return nil
		}
		return d.add(p.Transformed(m))
	case *Spline:
		pts := e.Points()
		closed := e.Closed && len(e.Fit) == 0
		return d.stroke(col, ctx.m, func(p *vector.Path) { p.Polyline(pts, closed) })
	case *Text:
		return d.text(e, col, m)
	case *MText:
		return d.mtext(e, col, m)
	case *Solid:
		k := e.Corners
		p := &vector.Path{Fill: true, Stroke: true, Color: col}
		p.Polyline([]Point{k[0], k[1], k[3], k[2]}, true)
		return d.add(p.Transformed(m))
	case *Hatch:
		p := e.Boundary
		p.Fill, p.Stroke, p.Color = true, false, col
		if !e.Solid {
			p.Color = patternShade
		}
		if len(p.Segs) == 0 {
			return nil
		}
		return d.add(p.Transformed(m))
	case *Insert:
		return d.insert(e, layer, col, ctx)
	case *Dimension:
		return d.block(e.Block, vector.Identity, layer, col, ctx)
	}
	return nil
}

// AddPolyline adds a subpath through vertices joined by straight segments
// and bulge arcs.
func AddPolyline(p *vector.Path, vs []Vertex, closed bool) {
	if len(vs) == 0 {
		return
	}
	p.MoveTo(vs[0].P)
	n := len(vs) - 1
	if closed {
		n = len(vs)
	}
	for i := range n {
		a, b := vs[i], vs[(i+1)%len(vs)]
		bulgeTo(p, a.P, b.P, a.Bulge)
	}
	if closed {
		p.Close()
	}
}

// bulgeTo adds the segment from a to b with the given bulge.
func bulgeTo(p *vector.Path, a, b Point, bulge float64) {
	chord := b.Sub(a)
	l := chord.Len()
	if math.Abs(bulge) < 1e-9 || l < 1e-12 {
		p.LineTo(b)
		return
	}
	// The centre sits off the chord's midpoint, to the left for a positive
	// bulge, at a distance that is (1-b²)/(4b) chords.
	left := Point{X: -chord.Y, Y: chord.X}.Mul(1 / l)
	c := a.Add(chord.Mul(0.5)).Add(left.Mul(l * (1 - bulge*bulge) / (4 * bulge)))
	r := a.Sub(c).Len()
	start := math.Atan2(a.Y-c.Y, a.X-c.X)
	p.EllipseArc(c, Point{X: r}, 1, start, start+4*math.Atan(bulge), true)
	// The arc ends on b up to rounding; land exactly on it so the next
	// segment starts where the file says.
	p.Segs[len(p.Segs)-1].P[2] = b
}

func (d *drawer) text(t *Text, col vector.Color, m vector.Affine) error {
	s := PlainText(t.Content)
	if strings.TrimSpace(s) == "" || t.Height <= 0 {
		return nil
	}
	align := map[int]vector.Align{0: vector.Left, 1: vector.Center, 2: vector.Right, 4: vector.Center}[t.Align]
	valign := map[int]vector.VAlign{0: vector.Baseline, 1: vector.Bottom, 2: vector.Middle, 3: vector.Top}[t.VAlign]
	if t.Align == 4 { // "middle": centred both ways
		valign = vector.Middle
	}
	return d.add((&vector.Text{
		At: t.At, Height: t.Height, Angle: t.Rotation, Align: align, VAlign: valign,
		Content: s, Color: col, WidthFactor: t.WidthFactor,
	}).Transformed(m))
}

// lineSpacing is MTEXT's default distance between baselines, in text heights.
const lineSpacing = 5.0 / 3

func (d *drawer) mtext(t *MText, col vector.Color, m vector.Affine) error {
	lines := MTextLines(t.Content)
	if t.Height <= 0 || strings.TrimSpace(strings.Join(lines, "")) == "" {
		return nil
	}
	attach := t.Attach
	if attach < 1 || attach > 9 {
		attach = 1
	}
	align := []vector.Align{vector.Left, vector.Center, vector.Right}[(attach-1)%3]
	// Lay the lines out from the top, then shift the block so the attachment
	// point lands on its middle or bottom when asked.
	total := t.Height + float64(len(lines)-1)*lineSpacing*t.Height
	shift := []float64{0, total / 2, total}[(attach-1)/3]
	place := vector.Rotate(t.Rotation).Then(vector.Translate(t.At.X, t.At.Y)).Then(m)
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		y := shift - float64(i)*lineSpacing*t.Height
		err := d.add((&vector.Text{
			At: Point{Y: y}, Height: t.Height, Align: align, VAlign: vector.Top, Content: line, Color: col,
		}).Transformed(place))
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *drawer) insert(in *Insert, layer *Layer, col vector.Color, ctx context) error {
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
			if err := d.block(in.Block, m, layer, col, ctx); err != nil {
				return err
			}
		}
	}
	// Attributes are stored where they sit in the drawing, not in the block.
	for _, a := range in.Attribs {
		if err := d.entity(a, ctx); err != nil {
			return err
		}
	}
	return nil
}

// block draws a block's entities under m (placing its base point at the
// origin first) and then ctx.m.
func (d *drawer) block(
	name string,
	m vector.Affine,
	layer *Layer,
	col vector.Color,
	ctx context,
) error {
	b := d.doc.Blocks[name]
	if b == nil {
		return nil
	}
	for _, open := range d.stack {
		if strings.EqualFold(open, name) {
			d.loops++
			return nil
		}
	}
	if len(d.stack) >= MaxDepth {
		d.deep++
		return nil
	}
	d.stack = append(d.stack, name)
	defer func() { d.stack = d.stack[:len(d.stack)-1] }()
	inner := context{
		m:     vector.Translate(-b.Base.X, -b.Base.Y).Then(m).Then(ctx.m),
		layer: layer,
		color: col,
	}
	return d.entities(b.Entities, inner)
}
