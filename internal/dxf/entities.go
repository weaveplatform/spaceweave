package dxf

import (
	"math"
	"strings"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// fields are one object's pairs, in file order.
type fields []pair

func (f fields) find(code int) (pair, bool) {
	for _, p := range f {
		if p.code == code {
			return p, true
		}
	}
	return pair{}, false
}

func (f fields) str(code int) string {
	p, _ := f.find(code)
	return p.val
}

func (f fields) float(code int) float64 {
	p, _ := f.find(code)
	return p.float()
}

func (f fields) int(code int) (int, bool) {
	p, ok := f.find(code)
	return p.int(), ok
}

// point reads the X and Y of a point whose X is group code x (Y is x+10).
func (f fields) point(x int) cad.Point {
	return cad.Point{X: f.float(x), Y: f.float(x + 10)}
}

func deg(d float64) float64 { return d * math.Pi / 180 }

func trueColor(c int) *vector.Color {
	return &vector.Color{
		R: uint8(c >> 16 & 0xff),
		G: uint8(c >> 8 & 0xff),
		B: uint8(c & 0xff),
	} //nolint:gosec // masked to a byte
}

func (p *parser) common(g fields) cad.Common {
	c := cad.Common{Layer: p.text(g.str(8)), Color: cad.ByLayer}
	if c.Layer == "" {
		c.Layer = "0"
	}
	if v, ok := g.int(62); ok {
		c.Color = v
	}
	if v, ok := g.int(420); ok {
		c.RGB = trueColor(v)
	}
	if v, _ := g.int(60); v == 1 {
		c.Invisible = true
	}
	if z, ok := g.find(230); ok && z.float() < 0 {
		c.Mirrored = true
	}
	return c
}

// entity builds one entity, reading any VERTEX or ATTRIB children it has.
// It returns nil for a type that isn't drawn.
//
//nolint:gocyclo // one case per entity type
func (p *parser) entity(kind string, g fields) (cad.Entity, error) {
	c := p.common(g)
	switch kind {
	case "LINE":
		return &cad.Line{Common: c, A: g.point(10), B: g.point(11)}, nil
	case "CIRCLE":
		return &cad.Circle{Common: c, Center: g.point(10), Radius: g.float(40)}, nil
	case "ARC":
		return &cad.Arc{
			Common: c, Center: g.point(10), Radius: g.float(40),
			Start: deg(g.float(50)), End: deg(g.float(51)),
		}, nil
	case "ELLIPSE":
		// Ellipses are stored in world coordinates whatever their extrusion.
		c.Mirrored = false
		end := 2 * math.Pi
		if v, ok := g.find(42); ok {
			end = v.float()
		}
		return &cad.Ellipse{
			Common: c, Center: g.point(10), Major: g.point(11), Ratio: g.float(40),
			Start: g.float(41), End: end,
		}, nil
	case "LWPOLYLINE":
		return lwpolyline(c, g), nil
	case "POLYLINE":
		kids, err := p.children()
		if err != nil {
			return nil, err
		}
		return polyline(c, g, kids), nil
	case "SPLINE":
		return spline(c, g), nil
	case "TEXT":
		return p.textEntity(c, g, 73), nil
	case "MTEXT":
		return p.mtext(c, g), nil
	case "INSERT":
		in := insert(c, g)
		in.Block = p.text(in.Block)
		if v, _ := g.int(66); v == 1 {
			kids, err := p.children()
			if err != nil {
				return nil, err
			}
			for _, k := range kids {
				a := p.textEntity(p.common(k), k, 74)
				if f, _ := k.int(70); f&1 != 0 {
					a.Invisible = true
				}
				in.Attribs = append(in.Attribs, a)
			}
		}
		return in, nil
	case "SOLID", "TRACE":
		return &cad.Solid{
			Common:  c,
			Corners: [4]cad.Point{g.point(10), g.point(11), g.point(12), g.point(13)},
		}, nil
	case "HATCH":
		h := &cad.Hatch{Common: c}
		if v, _ := g.int(70); v == 1 {
			h.Solid = true
		}
		hatchBoundary(&h.Boundary, g)
		return h, nil
	case "DIMENSION":
		return &cad.Dimension{Common: c, Block: p.text(g.str(2))}, nil
	default:
		p.skip(kind)
		return nil, nil //nolint:nilnil // a type that isn't drawn is not an error
	}
}

func lwpolyline(c cad.Common, g fields) *cad.Polyline {
	pl := &cad.Polyline{Common: c, Width: g.float(43)}
	if f, _ := g.int(70); f&1 != 0 {
		pl.Closed = true
	}
	for _, pr := range g {
		switch pr.code {
		case 10:
			pl.Vertices = append(pl.Vertices, cad.Vertex{P: cad.Point{X: pr.float()}})
		case 20:
			if n := len(pl.Vertices); n > 0 {
				pl.Vertices[n-1].P.Y = pr.float()
			}
		case 42:
			if n := len(pl.Vertices); n > 0 {
				pl.Vertices[n-1].Bulge = pr.float()
			}
		case 40, 41:
			pl.Width = math.Max(pl.Width, pr.float())
		}
	}
	return pl
}

// Polyline flags.
const (
	plClosed   = 1
	plMesh     = 16
	plPolyface = 64
	vxFrame    = 16 // a spline's frame control point, not on the curve
)

func polyline(c cad.Common, g fields, kids []fields) cad.Entity {
	flags, _ := g.int(70)
	if flags&(plMesh|plPolyface) != 0 {
		return nil
	}
	pl := &cad.Polyline{
		Common: c, Closed: flags&plClosed != 0,
		Width: math.Max(g.float(40), g.float(41)),
	}
	for _, k := range kids {
		if f, _ := k.int(70); f&vxFrame != 0 {
			continue
		}
		pl.Vertices = append(pl.Vertices, cad.Vertex{P: k.point(10), Bulge: k.float(42)})
	}
	return pl
}

func spline(c cad.Common, g fields) *cad.Spline {
	s := &cad.Spline{Common: c}
	s.Degree, _ = g.int(71)
	if f, _ := g.int(70); f&1 != 0 {
		s.Closed = true
	}
	for _, pr := range g {
		switch pr.code {
		case 40:
			s.Knots = append(s.Knots, pr.float())
		case 41:
			s.Weights = append(s.Weights, pr.float())
		case 10:
			s.Control = append(s.Control, cad.Point{X: pr.float()})
		case 20:
			if n := len(s.Control); n > 0 {
				s.Control[n-1].Y = pr.float()
			}
		case 11:
			s.Fit = append(s.Fit, cad.Point{X: pr.float()})
		case 21:
			if n := len(s.Fit); n > 0 {
				s.Fit[n-1].Y = pr.float()
			}
		}
	}
	return s
}

// textEntity reads TEXT, or ATTRIB when vcode is 74 (an attribute keeps its
// vertical alignment there, because 73 is its field length).
func (p *parser) textEntity(c cad.Common, g fields, vcode int) *cad.Text {
	t := &cad.Text{
		Common: c, Height: g.float(40), Rotation: deg(g.float(50)),
		WidthFactor: g.float(41), Content: p.text(g.str(1)),
	}
	t.Align, _ = g.int(72)
	t.VAlign, _ = g.int(vcode)
	t.At = g.point(10)
	// Aligned (3) and fit (5) text stretch between the two points; the
	// insertion point is a fine anchor for both.
	if (t.Align != 0 || t.VAlign != 0) && t.Align != 3 && t.Align != 5 {
		if _, ok := g.find(11); ok {
			t.At = g.point(11)
		}
	}
	return t
}

func (p *parser) mtext(c cad.Common, g fields) *cad.MText {
	t := &cad.MText{Common: c, At: g.point(10), Height: g.float(40), Rotation: deg(g.float(50))}
	t.Attach, _ = g.int(71)
	// Long text is split into 250-character chunks: code 3 for every chunk
	// but the last, which is code 1.
	var b strings.Builder
	for _, pr := range g {
		if pr.code == 3 {
			b.WriteString(pr.val)
		}
	}
	b.WriteString(g.str(1))
	t.Content = p.text(b.String())
	if _, ok := g.find(11); ok {
		// The X-axis direction overrides the rotation angle.
		d := g.point(11)
		if d.X != 0 || d.Y != 0 {
			t.Rotation = math.Atan2(d.Y, d.X)
		}
	}
	return t
}

func insert(c cad.Common, g fields) *cad.Insert {
	in := &cad.Insert{
		Common: c, Block: g.str(2), At: g.point(10), Rotation: deg(g.float(50)),
		ColumnGap: g.float(44), RowGap: g.float(45),
	}
	in.Scale = cad.Point{X: 1, Y: 1}
	if _, ok := g.find(41); ok {
		in.Scale.X = g.float(41)
	}
	if _, ok := g.find(42); ok {
		in.Scale.Y = g.float(42)
	}
	in.Columns, _ = g.int(70)
	in.Rows, _ = g.int(71)
	return in
}
