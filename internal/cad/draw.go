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
	doc *Document
	out *vector.Drawing
}

// Draw turns a document into a vector drawing, with its units in metres.
func Draw(doc *Document) (*vector.Drawing, error) {
	d := &drawer{doc: doc, out: &vector.Drawing{}}
	walked, err := Walk(doc, d.entity)
	if err != nil {
		return nil, err
	}
	if walked.Paper {
		d.out.Notes = append(d.out.Notes, "Model space is empty; drew the paper layout instead.")
	}
	if walked.Loops > 0 {
		d.out.Notes = append(
			d.out.Notes,
			fmt.Sprintf("%d block references that contain themselves were left out.", walked.Loops),
		)
	}
	if walked.Deep > 0 {
		d.out.Notes = append(
			d.out.Notes,
			fmt.Sprintf(
				"%d block references nested over %d deep were left out.",
				walked.Deep,
				MaxDepth,
			),
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

// entity draws one entity the walk found. Block references draw nothing
// themselves; the walk goes on into their contents.
func (d *drawer) entity(p *Placed) error {
	col, m := p.Color, p.M
	switch e := p.Entity.(type) {
	case *Line:
		return d.stroke(col, m, func(p *vector.Path) { p.Polyline([]Point{e.A, e.B}, false) })
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
		return d.stroke(col, m, func(p *vector.Path) { p.Polyline(pts, closed) })
	case *Text, *MText:
		for _, t := range TextLines(e, m) {
			t.Color = col
			if err := d.add(t); err != nil {
				return err
			}
		}
		return nil
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

// TextLines lays out a Text or MText as Draw draws it, one line each, under
// m (a Placed's M). It returns nil for blank text and other entities. The
// lines' colour is left zero.
func TextLines(e Entity, m vector.Affine) []*vector.Text {
	switch e := e.(type) {
	case *Text:
		if t := textLine(e, m); t != nil {
			return []*vector.Text{t}
		}
	case *MText:
		return mtextLines(e, m)
	}
	return nil
}

func transformedText(t *vector.Text, m vector.Affine) *vector.Text {
	q, _ := t.Transformed(m).(*vector.Text)
	return q
}

func textLine(t *Text, m vector.Affine) *vector.Text {
	s := PlainText(t.Content)
	if strings.TrimSpace(s) == "" || t.Height <= 0 {
		return nil
	}
	align := map[int]vector.Align{0: vector.Left, 1: vector.Center, 2: vector.Right, 4: vector.Center}[t.Align]
	valign := map[int]vector.VAlign{0: vector.Baseline, 1: vector.Bottom, 2: vector.Middle, 3: vector.Top}[t.VAlign]
	if t.Align == 4 { // "middle": centred both ways
		valign = vector.Middle
	}
	return transformedText(&vector.Text{
		At: t.At, Height: t.Height, Angle: t.Rotation, Align: align, VAlign: valign,
		Content: s, WidthFactor: t.WidthFactor,
	}, m)
}

// lineSpacing is MTEXT's default distance between baselines, in text heights.
const lineSpacing = 5.0 / 3

func mtextLines(t *MText, m vector.Affine) []*vector.Text {
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
	out := make([]*vector.Text, 0, len(lines))
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		y := shift - float64(i)*lineSpacing*t.Height
		out = append(out, transformedText(&vector.Text{
			At: Point{Y: y}, Height: t.Height, Align: align, VAlign: vector.Top, Content: line,
		}, place))
	}
	return out
}
