// Package svgdraw reads an SVG file into a vector.Drawing: the shapes, text
// and embedded images a floor plan exported from a CAD or illustration
// tool is made of. It is not a browser: no scripting, filters, masks or
// external resources, and only simple CSS selectors.
package svgdraw

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // embedded images
	_ "image/png"
	"io"
	"strings"

	"golang.org/x/text/encoding/htmlindex"

	"github.com/weaveplatform/spaceweave/internal/vector"
)

// Errors.
var (
	ErrNotSVG  = errors.New("not an SVG file")
	ErrTooBig  = errors.New("the SVG has too many elements")
	ErrTooDeep = errors.New("the SVG nests too deeply")
)

// Limits on a document, so a hostile file can't exhaust memory or the stack.
const (
	maxNodes = 2_000_000
	maxDepth = 256
	maxUses  = 32 // <use> chains followed before giving up
	// The elements walked in all, counting each time a <use> repeats one,
	// are capped at visitsPerNode × the file's elements + extraVisits.
	// maxUses only limits a chain's length: a group that uses itself twice
	// per level would otherwise be walked 2^maxUses times. The allowance
	// covers heavy symbol reuse (a 100-element chair placed 2,000 times is
	// 200k) while stopping a small fan-out bomb in a fraction of a second.
	visitsPerNode = 4
	extraVisits   = 250_000
)

type node struct {
	name     string
	attrs    map[string]string
	children []*node
	text     strings.Builder
}

// Read parses an SVG document.
func Read(r io.Reader) (*vector.Drawing, error) {
	root, err := parse(r)
	if err != nil {
		return nil, err
	}
	w := &walker{ids: map[string]*node{}, out: &vector.Drawing{}}
	w.index(root)
	w.budget = visitsPerNode*w.nodes + extraVisits
	w.out.Unit, w.out.Paper = paperUnit(root)
	// SVG's Y axis points down; the drawing's points up.
	top := viewBox(root).Then(vector.Scale(1, -1))
	w.element(root, top, defaultStyle(), 0)
	if w.visits > w.budget {
		return nil, fmt.Errorf(
			"%w: its <use> elements repeat more than %d elements",
			ErrTooBig,
			w.budget,
		)
	}
	switch {
	case w.skippedImages == 1:
		w.out.Notes = append(
			w.out.Notes,
			"1 linked image was left out; only embedded ones are drawn.",
		)
	case w.skippedImages > 1:
		w.out.Notes = append(w.out.Notes, fmt.Sprintf(
			"%d linked images were left out; only embedded ones are drawn.", w.skippedImages))
	}
	return w.out, nil
}

// charsetReader decodes a document declared in a legacy encoding
// (windows-1252, ISO-8859-1, ...). Passing its bytes through would stop the
// XML decoder at the first non-ASCII byte, and the rest of the drawing would
// be lost. An encoding we don't know is read as it is.
func charsetReader(label string, in io.Reader) (io.Reader, error) {
	enc, err := htmlindex.Get(label)
	if err != nil {
		return in, nil //nolint:nilerr // read unknown encodings as they are
	}
	return enc.NewDecoder().Reader(in), nil
}

func parse(r io.Reader) (*node, error) {
	dec := xml.NewDecoder(r)
	dec.Strict = false
	dec.CharsetReader = charsetReader
	var stack []*node
	var root *node
	count := 0
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if root != nil {
				break // keep what was read, as browsers do
			}
			return nil, fmt.Errorf("%w: %w", ErrNotSVG, err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			count++
			if count > maxNodes {
				return nil, ErrTooBig
			}
			if len(stack) >= maxDepth {
				return nil, ErrTooDeep
			}
			n := &node{name: t.Name.Local, attrs: map[string]string{}}
			for _, a := range t.Attr {
				n.attrs[a.Name.Local] = a.Value
			}
			if len(stack) == 0 {
				if root != nil || n.name != "svg" {
					return nil, ErrNotSVG
				}
				root = n
			} else {
				p := stack[len(stack)-1]
				p.children = append(p.children, n)
			}
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text.Write(t)
			}
		}
	}
	if root == nil {
		return nil, ErrNotSVG
	}
	return root, nil
}

// paperUnit is metres per user unit when the root's width is in an absolute
// unit (mm, cm, in, pt, pc): a scale on paper, not of the building.
func paperUnit(root *node) (float64, bool) {
	v, unit, ok := length(root.attrs["width"])
	m := metresPer[unit]
	if !ok || m == 0 || v <= 0 {
		return 0, false
	}
	vb := strings.Fields(strings.ReplaceAll(root.attrs["viewBox"], ",", " "))
	if len(vb) == 4 {
		if w := num(vb[2]); w > 0 {
			return v * m / w, true
		}
	}
	return m, true
}

// viewBox maps the root's viewBox onto its width and height, so user units
// keep their proportions; without a viewBox, user units are the drawing's.
func viewBox(n *node) vector.Affine {
	vb := strings.Fields(strings.ReplaceAll(n.attrs["viewBox"], ",", " "))
	if len(vb) != 4 {
		return vector.Identity
	}
	return vector.Translate(-num(vb[0]), -num(vb[1]))
}

type style struct {
	fill, stroke paint
	color        vector.Color
	strokeWidth  float64
	fontSize     float64
	anchor       string
	hidden       bool
}

func defaultStyle() style {
	return style{
		fill:        paint{set: true},
		stroke:      paint{set: true, none: true},
		strokeWidth: 1,
		fontSize:    16,
	}
}

type walker struct {
	ids           map[string]*node
	rules         []rule
	out           *vector.Drawing
	uses          int
	visits        int
	nodes, budget int
	skippedImages int
}

func (w *walker) index(n *node) {
	w.nodes++
	if id := n.attrs["id"]; id != "" {
		w.ids[id] = n
	}
	if n.name == "style" {
		w.rules = append(w.rules, parseCSS(n.text.String())...)
	}
	for _, c := range n.children {
		w.index(c)
	}
}

// props gathers an element's properties: presentation attributes first,
// then matching CSS rules (tag, class, id, in that order of strength), then
// its style attribute.
func (w *walker) props(n *node) map[string]string {
	p := map[string]string{}
	for _, k := range []string{
		"fill", "stroke", "stroke-width", "color", "font-size", "text-anchor",
		"display", "visibility", "opacity", "fill-opacity", "stroke-opacity",
	} {
		if v, ok := n.attrs[k]; ok {
			p[k] = v
		}
	}
	classes := strings.Fields(n.attrs["class"])
	for _, kind := range []byte{0, '.', '#'} {
		for _, r := range w.rules {
			if matches(r.sel, kind, n, classes) {
				for k, v := range r.props {
					p[k] = v
				}
			}
		}
	}
	parseStyle(n.attrs["style"], p)
	return p
}

func matches(sel string, kind byte, n *node, classes []string) bool {
	switch kind {
	case '.':
		if strings.HasPrefix(sel, ".") {
			for _, c := range classes {
				if sel[1:] == c {
					return true
				}
			}
		}
	case '#':
		return strings.HasPrefix(sel, "#") && sel[1:] == n.attrs["id"]
	default:
		return sel == n.name || sel == "*"
	}
	return false
}

func (w *walker) style(n *node, parent style) style {
	s := parent
	p := w.props(n)
	if c, ok := parseColor(strings.ToLower(strings.TrimSpace(p["color"]))); ok {
		s.color = c
	}
	if f := parsePaint(p["fill"], s.color); f.set {
		s.fill = f
	}
	if st := parsePaint(p["stroke"], s.color); st.set {
		s.stroke = st
	}
	if v, ok := p["stroke-width"]; ok {
		s.strokeWidth = num(v)
	}
	if v, ok := p["font-size"]; ok {
		if f, _, ok := length(v); ok && f > 0 {
			s.fontSize = f
		}
	}
	if v, ok := p["text-anchor"]; ok {
		s.anchor = strings.TrimSpace(v)
	}
	// A transparent element draws nothing; display:none hides its children
	// too, visibility:hidden only itself (but nothing here overrides it).
	for _, k := range []string{"opacity", "fill-opacity"} {
		if v, ok := p[k]; ok && strings.TrimSpace(v) == "0" {
			if k == "opacity" {
				s.hidden = true
			} else {
				s.fill = paint{set: true, none: true}
			}
		}
	}
	if v, ok := p["stroke-opacity"]; ok && strings.TrimSpace(v) == "0" {
		s.stroke = paint{set: true, none: true}
	}
	if strings.TrimSpace(p["display"]) == "none" || strings.TrimSpace(p["visibility"]) == "hidden" {
		s.hidden = true
	}
	return s
}

// skipped are elements whose content is never drawn directly.
var skipped = map[string]bool{
	"defs": true, "symbol": true, "clipPath": true, "mask": true, "pattern": true, "marker": true,
	"linearGradient": true, "radialGradient": true, "filter": true, "style": true, "script": true,
	"title": true, "desc": true, "metadata": true, "foreignObject": true,
}

//nolint:gocyclo // one case per element
func (w *walker) element(n *node, m vector.Affine, parent style, depth int) {
	w.visits++
	if depth > maxDepth || w.visits > w.budget {
		return
	}
	s := w.style(n, parent)
	if s.hidden {
		return
	}
	if t, ok := n.attrs["transform"]; ok {
		m = parseTransform(t).Then(m)
	}
	switch n.name {
	case "svg":
		if depth > 0 {
			m = vector.Translate(num(n.attrs["x"]), num(n.attrs["y"])).Then(m)
		}
		w.children(n, m, s, depth)
	case "g", "a", "switch":
		w.children(n, m, s, depth)
	case "use":
		w.use(n, m, s, depth)
	case "path":
		p := &vector.Path{}
		parsePath(n.attrs["d"], p)
		w.shape(p, m, s)
	case "rect":
		x, y, wd, ht := num(
			n.attrs["x"],
		), num(
			n.attrs["y"],
		), num(
			n.attrs["width"],
		), num(
			n.attrs["height"],
		)
		if wd > 0 && ht > 0 {
			p := &vector.Path{}
			p.Polyline(
				[]vector.Point{
					{X: x, Y: y},
					{X: x + wd, Y: y},
					{X: x + wd, Y: y + ht},
					{X: x, Y: y + ht},
				},
				true,
			)
			w.shape(p, m, s)
		}
	case "circle":
		if r := num(n.attrs["r"]); r > 0 {
			p := &vector.Path{}
			p.Circle(vector.Point{X: num(n.attrs["cx"]), Y: num(n.attrs["cy"])}, r)
			w.shape(p, m, s)
		}
	case "ellipse":
		rx, ry := num(n.attrs["rx"]), num(n.attrs["ry"])
		if rx > 0 && ry > 0 {
			p := &vector.Path{}
			p.EllipseArc(
				vector.Point{X: num(n.attrs["cx"]), Y: num(n.attrs["cy"])},
				vector.Point{X: rx},
				ry/rx,
				0,
				6.283185307179586,
				false,
			)
			p.Close()
			w.shape(p, m, s)
		}
	case "line":
		p := &vector.Path{}
		p.Polyline(
			[]vector.Point{
				{X: num(n.attrs["x1"]), Y: num(n.attrs["y1"])},
				{X: num(n.attrs["x2"]), Y: num(n.attrs["y2"])},
			},
			false,
		)
		s.fill = paint{set: true, none: true}
		w.shape(p, m, s)
	case "polyline", "polygon":
		p := &vector.Path{}
		p.Polyline(points(n.attrs["points"]), n.name == "polygon")
		w.shape(p, m, s)
	case "text":
		w.text(n, m, s)
	case "image":
		w.image(n, m)
	default:
		if !skipped[n.name] {
			w.children(n, m, s, depth)
		}
	}
}

func (w *walker) children(n *node, m vector.Affine, s style, depth int) {
	for _, c := range n.children {
		w.element(c, m, s, depth+1)
	}
}

func (w *walker) use(n *node, m vector.Affine, s style, depth int) {
	href := n.attrs["href"]
	target := w.ids[strings.TrimPrefix(href, "#")]
	if target == nil || !strings.HasPrefix(href, "#") || w.uses >= maxUses {
		return
	}
	w.uses++
	defer func() { w.uses-- }()
	m = vector.Translate(num(n.attrs["x"]), num(n.attrs["y"])).Then(m)
	if target.name == "symbol" {
		w.children(target, m, w.style(target, s), depth+1)
		return
	}
	w.element(target, m, s, depth+1)
}

func (w *walker) shape(p *vector.Path, m vector.Affine, s style) {
	if len(p.Segs) == 0 {
		return
	}
	p.Fill = !s.fill.none
	p.Stroke = !s.stroke.none && s.strokeWidth > 0
	if !p.Fill && !p.Stroke {
		return
	}
	if p.Fill && p.Stroke && s.fill.color != s.stroke.color {
		// One path has one colour: draw the fill and the outline apart.
		f := *p
		f.Stroke, f.Color = false, s.fill.color
		w.out.Add(f.Transformed(m))
		p.Fill = false
	}
	if p.Fill {
		p.Color = s.fill.color
	} else {
		p.Color = s.stroke.color
	}
	p.Width = s.strokeWidth
	w.out.Add(p.Transformed(m))
}

// capHeight is a typical cap height as a fraction of the font size.
const capHeight = 0.7

func (w *walker) text(n *node, m vector.Affine, s style) {
	x, y := num(firstOf(n.attrs["x"])), num(firstOf(n.attrs["y"]))
	w.textRun(strings.TrimSpace(n.text.String()), x, y, m, s)
	for _, c := range n.children {
		if c.name != "tspan" {
			continue
		}
		cs := w.style(c, s)
		if cs.hidden {
			continue
		}
		cx, cy := x, y
		if v, ok := c.attrs["x"]; ok {
			cx = num(firstOf(v))
		}
		if v, ok := c.attrs["y"]; ok {
			cy = num(firstOf(v))
		}
		cx += num(firstOf(c.attrs["dx"]))
		cy += num(firstOf(c.attrs["dy"]))
		w.textRun(strings.TrimSpace(c.text.String()), cx, cy, m, cs)
	}
}

// firstOf is the first of a list of coordinates (per-glyph positions are
// more than a plan view needs).
func firstOf(s string) string {
	if f := strings.FieldsFunc(s, func(r rune) bool { return r == ' ' || r == ',' }); len(f) > 0 {
		return f[0]
	}
	return ""
}

func (w *walker) textRun(content string, x, y float64, m vector.Affine, s style) {
	content = strings.Join(strings.Fields(content), " ")
	if content == "" {
		return
	}
	align := map[string]vector.Align{"middle": vector.Center, "end": vector.Right}[s.anchor]
	c := s.fill.color
	if s.fill.none {
		c = s.stroke.color
	}
	t := &vector.Text{
		At:      vector.Point{X: x, Y: y},
		Height:  s.fontSize * capHeight,
		Align:   align,
		Content: content,
		Color:   c,
	}
	w.out.Add(t.Transformed(m))
}

func (w *walker) image(n *node, m vector.Affine) {
	href := n.attrs["href"]
	_, data, ok := strings.Cut(href, ";base64,")
	if !strings.HasPrefix(href, "data:image/") || !ok {
		w.skippedImages++
		return
	}
	raw, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(data), ""))
	if err != nil {
		return
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return
	}
	b := img.Bounds()
	wd, ht := num(n.attrs["width"]), num(n.attrs["height"])
	if wd <= 0 || ht <= 0 {
		wd, ht = float64(b.Dx()), float64(b.Dy())
	}
	place := vector.Scale(wd/float64(b.Dx()), ht/float64(b.Dy())).
		Then(vector.Translate(num(n.attrs["x"]), num(n.attrs["y"]))).Then(m)
	w.out.Add(&vector.Image{Img: img, M: place})
}
