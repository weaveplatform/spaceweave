package vector

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"image/png"
	"io"
	"math"
	"strconv"
	"strings"
)

// hairline is the width of a line with none of its own, as a fraction of the
// drawing's long side: about a pixel when the whole drawing fills a screen.
// vector-effect="non-scaling-stroke" would say "one pixel" exactly, but
// renderers that ignore it (rsvg, many viewers) then draw a line one drawing
// unit wide, a metre or more.
const hairline = 1.0 / 1600

func WriteSVG(w io.Writer, d *Drawing, bounds Rect) error {
	if bounds.Empty() {
		bounds, _ = Extent(d.Items)
	}
	if bounds.Empty() {
		return ErrEmpty
	}
	bw := bufio.NewWriter(w)
	size := bounds.Size()
	pad := math.Max(size.X, size.Y) * 0.02
	// SVG's Y axis points down: drawing y becomes -y.
	vx, vy := bounds.Min.X-pad, -bounds.Max.Y-pad
	vw, vh := size.X+2*pad, size.Y+2*pad
	fmt.Fprintf(bw, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="%s %s %s %s">`+"\n",
		num(vx), num(vy), num(math.Max(vw, 1e-9)), num(math.Max(vh, 1e-9)))
	fmt.Fprintf(bw, `<rect x="%s" y="%s" width="%s" height="%s" fill="#fff"/>`+"\n",
		num(vx), num(vy), num(vw), num(vh))
	flip := Scale(1, -1)
	thin := math.Max(size.X, size.Y) * hairline
	for _, it := range d.Items {
		switch it := it.(type) {
		case *Path:
			writePath(
				bw,
				it.Transformed(flip).(*Path),
				thin,
			) //nolint:forcetypeassert // a Path stays a Path
		case *Text:
			writeText(bw, it)
		case *Image:
			if err := writeImage(bw, it, flip); err != nil {
				return err
			}
		}
	}
	fmt.Fprintln(bw, "</svg>")
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("write svg: %w", err)
	}
	return nil
}

func num(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// short rounds to a precision that is invisible at any sensible zoom and
// keeps a big drawing's file small.
func short(v float64) string { return strconv.FormatFloat(v, 'g', 7, 64) }

func hex(c Color) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

func writePath(w io.Writer, p *Path, thin float64) {
	if len(p.Segs) == 0 || (!p.Stroke && !p.Fill) {
		return
	}
	var d bytes.Buffer
	for _, s := range p.Segs {
		switch s.Op {
		case MoveTo:
			fmt.Fprintf(&d, "M%s %s", short(s.P[0].X), short(s.P[0].Y))
		case LineTo:
			fmt.Fprintf(&d, "L%s %s", short(s.P[0].X), short(s.P[0].Y))
		case CubicTo:
			fmt.Fprintf(&d, "C%s %s %s %s %s %s", short(s.P[0].X), short(s.P[0].Y),
				short(s.P[1].X), short(s.P[1].Y), short(s.P[2].X), short(s.P[2].Y))
		case Close:
			d.WriteString("Z")
		}
	}
	fill := "none"
	if p.Fill {
		fill = hex(p.Color)
	}
	stroke := ""
	if p.Stroke {
		stroke = fmt.Sprintf(
			` stroke="%s" stroke-width="%s" stroke-linecap="round" stroke-linejoin="round"`,
			hex(p.Color),
			short(math.Max(p.Width, thin)),
		)
	}
	fmt.Fprintf(w, `<path d="%s" fill="%s" fill-rule="evenodd"%s/>`+"\n", d.String(), fill, stroke)
}

func writeText(w io.Writer, t *Text) {
	if t.Content == "" || t.Height <= 0 {
		return
	}
	anchor := map[Align]string{Left: "start", Center: "middle", Right: "end"}[t.Align]
	base := map[VAlign]string{
		Baseline: "alphabetic", Bottom: "text-after-edge",
		Middle: "central", Top: "text-before-edge",
	}[t.VAlign]
	// Rotation is clockwise in SVG's downward Y, so the angle flips sign.
	x, y := t.At.X, -t.At.Y
	tr := fmt.Sprintf(
		"translate(%s %s) rotate(%s)",
		short(x),
		short(y),
		short(-t.Angle*180/math.Pi),
	)
	if f := t.widthFactor(); f != 1 {
		tr += fmt.Sprintf(" scale(%s 1)", short(f))
	}
	// Font size is the em; Height is the cap height, about 0.7 of it.
	fmt.Fprintf(
		w,
		`<text transform="%s" font-family="sans-serif" font-size="%s" fill="%s" text-anchor="%s" dominant-baseline="%s">%s</text>`+"\n",
		tr,
		short(t.Height/0.7),
		hex(t.Color),
		anchor,
		base,
		html.EscapeString(xmlChars(t.Content)),
	)
}

// xmlChars drops the characters XML forbids (control characters other
// than tab and newlines, U+FFFE and U+FFFF); one of them in a CAD text would
// make the whole SVG unreadable.
func xmlChars(s string) string {
	return strings.Map(func(r rune) rune {
		if (r < 0x20 && r != '\t' && r != '\n' && r != '\r') || r == 0xFFFE || r == 0xFFFF {
			return -1
		}
		return r
	}, s)
}

func writeImage(w io.Writer, im *Image, flip Affine) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, im.Img); err != nil {
		return fmt.Errorf("embed image: %w", err)
	}
	b := im.Img.Bounds()
	m := im.M.Then(flip)
	fmt.Fprintf(
		w,
		`<image width="%d" height="%d" transform="matrix(%s %s %s %s %s %s)" href="data:image/png;base64,%s"/>`+"\n",
		b.Dx(),
		b.Dy(),
		short(m.A),
		short(m.B),
		short(m.C),
		short(m.D),
		short(m.E),
		short(m.F),
		base64.StdEncoding.EncodeToString(buf.Bytes()),
	)
	return nil
}
