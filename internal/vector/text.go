package vector

import (
	"math"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// textFont is the face every text is drawn in. Drawings name fonts we don't
// have (SHX, Arial, ...); the tracer needs the words and their size, not the
// typeface. Go Regular is bundled with x/image under a BSD licence.
var textFont = sync.OnceValues(func() (*sfnt.Font, error) {
	return sfnt.Parse(goregular.TTF)
})

// metrics are the font's in font units.
type metrics struct {
	upem          fixed.Int26_6
	cap, descent  float64
	fallbackGlyph sfnt.GlyphIndex
}

var fontMetrics = sync.OnceValues(func() (metrics, error) {
	f, err := textFont()
	if err != nil {
		return metrics{}, err //nolint:wrapcheck // only parses an embedded font
	}
	var b sfnt.Buffer
	upem := fixed.I(int(f.UnitsPerEm()))
	m, err := f.Metrics(&b, upem, font.HintingNone)
	if err != nil {
		return metrics{}, err //nolint:wrapcheck // as above
	}
	q, _ := f.GlyphIndex(&b, '?')
	return metrics{upem: upem, cap: fx(m.CapHeight), descent: fx(m.Descent), fallbackGlyph: q}, nil
})

func fx(v fixed.Int26_6) float64 { return float64(v) / 64 }

// Outline is the text as filled glyph outlines in drawing units, or nil when
// there's nothing to draw.
func (t *Text) Outline() *Path {
	m, err := fontMetrics()
	if err != nil || t.Height <= 0 || t.Content == "" || m.cap <= 0 {
		return nil
	}
	f, _ := textFont()
	var b sfnt.Buffer
	glyphs := &Path{}
	pen := 0.0
	for _, r := range t.Content {
		idx, err := f.GlyphIndex(&b, r)
		if err != nil || idx == 0 {
			idx = m.fallbackGlyph
		}
		segs, err := f.LoadGlyph(&b, idx, m.upem, nil)
		if err != nil {
			continue
		}
		appendGlyph(glyphs, segs, pen)
		adv, err := f.GlyphAdvance(&b, idx, m.upem, font.HintingNone)
		if err == nil {
			pen += fx(adv)
		}
	}
	if len(glyphs.Segs) == 0 {
		return nil
	}
	k := t.Height / m.cap
	width := pen * k * t.widthFactor()
	x0, _, _, _ := t.box(width)
	var y0 float64
	switch t.VAlign {
	case Baseline:
	case Bottom:
		y0 = m.descent * k
	case Middle:
		y0 = -t.Height / 2
	case Top:
		y0 = -t.Height
	}
	place := Scale(k*t.widthFactor(), k).
		Then(Translate(x0, y0)).
		Then(Rotate(t.Angle)).
		Then(Translate(t.At.X, t.At.Y))
	out, _ := glyphs.Transformed(place).(*Path)
	out.Fill, out.Color = true, t.Color
	return out
}

// appendGlyph adds a glyph's contours at pen, flipping sfnt's downward Y.
// TrueType contours are quadratic; each becomes the equivalent cubic.
func appendGlyph(p *Path, segs sfnt.Segments, pen float64) {
	at := func(q fixed.Point26_6) Point { return Point{pen + fx(q.X), -fx(q.Y)} }
	var last Point
	for _, s := range segs {
		switch s.Op {
		case sfnt.SegmentOpMoveTo:
			if len(p.Segs) > 0 {
				p.Close()
			}
			last = at(s.Args[0])
			p.MoveTo(last)
		case sfnt.SegmentOpLineTo:
			last = at(s.Args[0])
			p.LineTo(last)
		case sfnt.SegmentOpQuadTo:
			c, end := at(s.Args[0]), at(s.Args[1])
			p.CubicTo(last.Add(c.Sub(last).Mul(2.0/3)), end.Add(c.Sub(end).Mul(2.0/3)), end)
			last = end
		case sfnt.SegmentOpCubeTo:
			last = at(s.Args[2])
			p.CubicTo(at(s.Args[0]), at(s.Args[1]), last)
		}
	}
	if len(p.Segs) > 0 {
		p.Close()
	}
}

// TextWidth is how long the text runs along its baseline, in drawing units.
func (t *Text) TextWidth() float64 {
	m, err := fontMetrics()
	if err != nil || m.cap <= 0 {
		return float64(len([]rune(t.Content))) * avgAdvance * t.Height * t.widthFactor()
	}
	f, _ := textFont()
	var b sfnt.Buffer
	pen := 0.0
	for _, r := range t.Content {
		idx, err := f.GlyphIndex(&b, r)
		if err != nil || idx == 0 {
			idx = m.fallbackGlyph
		}
		if adv, err := f.GlyphAdvance(&b, idx, m.upem, font.HintingNone); err == nil {
			pen += fx(adv)
		}
	}
	return math.Abs(pen * t.Height / m.cap * t.widthFactor())
}
