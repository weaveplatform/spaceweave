package svgdraw

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/weaveplatform/spaceweave/internal/vector"
)

func readDoc(t *testing.T, doc string) *vector.Drawing {
	t.Helper()
	d, err := Read(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	return d
}

func read(t *testing.T, body string) *vector.Drawing {
	t.Helper()
	return readDoc(
		t,
		`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink">`+body+`</svg>`,
	)
}

func pathsOf(t *testing.T, d *vector.Drawing) []*vector.Path {
	t.Helper()
	out := make([]*vector.Path, 0, len(d.Items))
	for _, it := range d.Items {
		p, ok := it.(*vector.Path)
		if !ok {
			t.Fatalf("item %T is not a path", it)
		}
		out = append(out, p)
	}
	return out
}

func onePath(t *testing.T, d *vector.Drawing) *vector.Path {
	t.Helper()
	ps := pathsOf(t, d)
	if len(ps) != 1 {
		t.Fatalf("got %d paths, want 1: %v", len(ps), ps)
	}
	return ps[0]
}

func checkBounds(t *testing.T, it vector.Item, minPt, maxPt vector.Point) {
	t.Helper()
	b := it.Bounds()
	if !nearPt(b.Min, minPt) || !nearPt(b.Max, maxPt) {
		t.Fatalf("bounds %v..%v, want %v..%v", b.Min, b.Max, minPt, maxPt)
	}
}

var (
	red   = vector.Color{R: 255}
	blue  = vector.Color{B: 255}
	lime  = vector.Color{G: 255}
	white = vector.Color{R: 255, G: 255, B: 255}
)

func TestReadErrors(t *testing.T) {
	cases := []struct {
		name, doc string
		want      error
	}{
		{"empty", "", ErrNotSVG},
		{"plain text", "hello", ErrNotSVG},
		{"other root", "<html><svg/></html>", ErrNotSVG},
		{"malformed before the root", "<svg", ErrNotSVG},
		{"two roots", "<svg/><svg/>", ErrNotSVG},
		{"too deep", "<svg>" + strings.Repeat("<g>", maxDepth) + "</svg>", ErrTooDeep},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, err := Read(strings.NewReader(c.doc))
			if !errors.Is(err, c.want) || d != nil {
				t.Fatalf("Read = %v, %v; want %v", d, err, c.want)
			}
		})
	}
}

func TestReadDepthLimitBoundary(t *testing.T) {
	// The root and maxDepth-1 groups below it are maxDepth open elements,
	// the most the parser allows; the rectangle at the bottom is one too many.
	inner := strings.Repeat("<g>", maxDepth-1) + strings.Repeat("</g>", maxDepth-1)
	if _, err := Read(strings.NewReader("<svg>" + inner + "</svg>")); err != nil {
		t.Fatalf("Read at the limit: %v", err)
	}
	deep := strings.Repeat(
		"<g>",
		maxDepth-1,
	) + `<rect width="1" height="1"/>` + strings.Repeat(
		"</g>",
		maxDepth-1,
	)
	if _, err := Read(strings.NewReader("<svg>" + deep + "</svg>")); !errors.Is(err, ErrTooDeep) {
		t.Fatalf("Read past the limit = %v, want ErrTooDeep", err)
	}
}

func TestReadTooBig(t *testing.T) {
	if testing.Short() {
		t.Skip("parses two million elements")
	}
	t.Parallel()
	var b strings.Builder
	b.WriteString("<svg>")
	// Elements nest no deeper than one, so only the count limit can trip.
	for range maxNodes {
		b.WriteString("<g/>")
	}
	b.WriteString("</svg>")
	if _, err := Read(strings.NewReader(b.String())); !errors.Is(err, ErrTooBig) {
		t.Fatalf("Read = %v, want ErrTooBig", err)
	}
}

func TestReadKeepsWhatCameBeforeAnError(t *testing.T) {
	d := readDoc(t, `<svg><rect width="1" height="1"/><`)
	if len(d.Items) != 1 {
		t.Fatalf("got %d items, want the rectangle before the error", len(d.Items))
	}
}

func TestReadDeclaredCharset(t *testing.T) {
	// Exporters declare legacy charsets; without a CharsetReader the
	// decoder refuses them outright.
	d := readDoc(t, `<?xml version="1.0" encoding="windows-1252"?><svg><text>Cafe</text></svg>`)
	if tx, ok := d.Items[0].(*vector.Text); !ok || tx.Content != "Cafe" {
		t.Fatalf("got %+v", d.Items[0])
	}
}

func TestReadLegacyCharsetBytes(t *testing.T) {
	d := readDoc(
		t,
		`<?xml version="1.0" encoding="windows-1252"?><svg><text>Caf`+"\xe9"+`</text><rect width="1" height="1"/></svg>`,
	)
	if len(d.Items) != 2 {
		t.Fatalf("got %d items, want the text and the rectangle", len(d.Items))
	}
	if tx, ok := d.Items[0].(*vector.Text); !ok || tx.Content != "Caf\u00e9" {
		t.Fatalf("got %+v", d.Items[0])
	}
}

func TestPaperUnits(t *testing.T) {
	cases := []struct {
		name, attrs string
		unit        float64
		paper       bool
	}{
		{"mm with viewBox", `width="210mm" height="297mm" viewBox="0 0 210 297"`, 0.001, true},
		{"cm with a finer viewBox", `width="21cm" viewBox="0,0,2100,2970"`, 0.0001, true},
		{"inches without viewBox", `width="8.5in"`, 0.0254, true},
		{"points", `width="72pt" viewBox="0 0 72 72"`, 0.0254 / 72, true},
		{"picas", `width="6pc" viewBox="0 0 1 1"`, 0.0254, true},
		{
			"empty viewBox width falls back to the unit",
			`width="210mm" viewBox="0 0 0 297"`,
			0.001,
			true,
		},
		{"malformed viewBox falls back to the unit", `width="10mm" viewBox="0 0 10"`, 0.001, true},
		{"px is not a paper size", `width="800px" viewBox="0 0 800 600"`, 0, false},
		{"user units", `width="800"`, 0, false},
		{"percent", `width="100%"`, 0, false},
		{"negative", `width="-5mm"`, 0, false},
		{"no width", ``, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := readDoc(t, "<svg "+c.attrs+"/>")
			if !near(d.Unit, c.unit) || d.Paper != c.paper {
				t.Fatalf("Unit, Paper = %v, %v; want %v, %v", d.Unit, d.Paper, c.unit, c.paper)
			}
		})
	}
}

func TestYFlip(t *testing.T) {
	d := read(t, `<circle cx="10" cy="10" r="1"/><circle cx="10" cy="90" r="1"/>`)
	ps := pathsOf(t, d)
	if len(ps) != 2 {
		t.Fatalf("got %d paths", len(ps))
	}
	top, bottom := ps[0].Bounds().Center(), ps[1].Bounds().Center()
	// Near the top of the page in SVG means higher up in the drawing.
	if top.Y <= bottom.Y {
		t.Fatalf("top circle at y=%v, bottom at y=%v: Y was not flipped", top.Y, bottom.Y)
	}
	if !nearPt(top, pt(10, -10)) || !nearPt(bottom, pt(10, -90)) {
		t.Fatalf("centres %v, %v", top, bottom)
	}
}

func TestViewBoxOrigin(t *testing.T) {
	d := readDoc(
		t,
		`<svg viewBox="100 200 50 50"><rect x="100" y="200" width="10" height="20"/></svg>`,
	)
	checkBounds(t, onePath(t, d), pt(0, -20), pt(10, 0))
}

func TestShapes(t *testing.T) {
	cases := []struct {
		name, body string
		segs       int
		min, max   vector.Point
		closed     bool
	}{
		{"rect", `<rect x="5" y="7" width="10" height="10"/>`, 5, pt(5, -17), pt(15, -7), true},
		{"circle", `<circle cx="10" cy="20" r="5"/>`, 6, pt(5, -25), pt(15, -15), true},
		{"ellipse", `<ellipse cx="10" cy="20" rx="5" ry="2"/>`, 6, pt(5, -22), pt(15, -18), true},
		{
			"line",
			`<line x1="1" y1="2" x2="3" y2="4" stroke="black"/>`,
			2,
			pt(1, -4),
			pt(3, -2),
			false,
		},
		{"polyline", `<polyline points="0,0 10,0 10,10"/>`, 3, pt(0, -10), pt(10, 0), false},
		{"polygon", `<polygon points="0,0 10,0 10,10"/>`, 4, pt(0, -10), pt(10, 0), true},
		{"path", `<path d="M0 0 L4 0 L4 4 Z"/>`, 4, pt(0, -4), pt(4, 0), true},
		{
			"group transform",
			`<g transform="translate(10 20)"><rect width="1" height="1"/></g>`,
			5,
			pt(10, -21),
			pt(11, -20),
			true,
		},
		{
			"element transform",
			`<rect transform="scale(2)" width="1" height="1"/>`,
			5,
			pt(0, -2),
			pt(2, 0),
			true,
		},
		{
			"nested svg offset",
			`<svg x="3" y="4"><rect width="1" height="1"/></svg>`,
			5,
			pt(3, -5),
			pt(4, -4),
			true,
		},
		{"link", `<a href="#"><rect width="1" height="1"/></a>`, 5, pt(0, -1), pt(1, 0), true},
		{"switch", `<switch><rect width="1" height="1"/></switch>`, 5, pt(0, -1), pt(1, 0), true},
		{
			"unknown element passes through",
			`<sodipodi:layer><rect width="1" height="1"/></sodipodi:layer>`,
			5,
			pt(0, -1),
			pt(1, 0),
			true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := onePath(t, read(t, c.body))
			if len(p.Segs) != c.segs {
				t.Fatalf("got %d segments, want %d: %v", len(p.Segs), c.segs, p.Segs)
			}
			if got := p.Segs[len(p.Segs)-1].Op == vector.Close; got != c.closed {
				t.Fatalf("closed = %v, want %v", got, c.closed)
			}
			checkBounds(t, p, c.min, c.max)
		})
	}
}

func TestShapesThatDrawNothing(t *testing.T) {
	cases := map[string]string{
		"zero width rect":   `<rect width="0" height="5"/>`,
		"zero radius":       `<circle r="0"/>`,
		"flat ellipse":      `<ellipse rx="5" ry="0"/>`,
		"unstroked line":    `<line x1="0" y1="0" x2="5" y2="5"/>`,
		"empty path":        `<path d=""/>`,
		"empty points":      `<polygon points=""/>`,
		"defs":              `<defs><rect width="1" height="1"/></defs>`,
		"symbol":            `<symbol><rect width="1" height="1"/></symbol>`,
		"title":             `<title><rect width="1" height="1"/></title>`,
		"no paint":          `<rect width="1" height="1" fill="none"/>`,
		"zero stroke width": `<rect width="1" height="1" fill="none" stroke="red" stroke-width="0"/>`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if d := read(t, body); len(d.Items) != 0 {
				t.Fatalf("drew %d items", len(d.Items))
			}
		})
	}
}

func TestFillAndStroke(t *testing.T) {
	t.Run("default is a black fill", func(t *testing.T) {
		p := onePath(t, read(t, `<rect width="1" height="1"/>`))
		if !p.Fill || p.Stroke || p.Color != vector.Black {
			t.Fatalf("fill %v stroke %v colour %v", p.Fill, p.Stroke, p.Color)
		}
	})
	t.Run("stroke only", func(t *testing.T) {
		p := onePath(
			t,
			read(t, `<rect width="1" height="1" fill="none" stroke="#f00" stroke-width="3"/>`),
		)
		if p.Fill || !p.Stroke || p.Color != red || p.Width != 3 {
			t.Fatalf("fill %v stroke %v colour %v width %v", p.Fill, p.Stroke, p.Color, p.Width)
		}
	})
	t.Run("same colour is one path", func(t *testing.T) {
		p := onePath(t, read(t, `<rect width="1" height="1" fill="blue" stroke="#0000ff"/>`))
		if !p.Fill || !p.Stroke || p.Color != blue {
			t.Fatalf("fill %v stroke %v colour %v", p.Fill, p.Stroke, p.Color)
		}
	})
	t.Run("different colours split", func(t *testing.T) {
		ps := pathsOf(t, read(t, `<rect width="1" height="1" fill="red" stroke="rgb(0,0,255)"/>`))
		if len(ps) != 2 {
			t.Fatalf("got %d paths", len(ps))
		}
		if !ps[0].Fill || ps[0].Stroke || ps[0].Color != red {
			t.Fatalf(
				"fill path: fill %v stroke %v colour %v",
				ps[0].Fill,
				ps[0].Stroke,
				ps[0].Color,
			)
		}
		if ps[1].Fill || !ps[1].Stroke || ps[1].Color != blue {
			t.Fatalf(
				"stroke path: fill %v stroke %v colour %v",
				ps[1].Fill,
				ps[1].Stroke,
				ps[1].Color,
			)
		}
	})
	t.Run("stroke width scales with the transform", func(t *testing.T) {
		p := onePath(
			t,
			read(t, `<g transform="scale(3)"><line x2="1" stroke="black" stroke-width="2px"/></g>`),
		)
		if !near(p.Width, 6) {
			t.Fatalf("width %v, want 6", p.Width)
		}
	})
	t.Run("gradient fill is grey", func(t *testing.T) {
		p := onePath(t, read(t, `<rect width="1" height="1" fill="url(#g)"/>`))
		if p.Color != gradientGrey {
			t.Fatalf("colour %v", p.Color)
		}
	})
	t.Run("unknown paint keeps the inherited one", func(t *testing.T) {
		p := onePath(t, read(t, `<g fill="red"><rect width="1" height="1" fill="bogus"/></g>`))
		if p.Color != red {
			t.Fatalf("colour %v", p.Color)
		}
	})
	t.Run("inherit", func(t *testing.T) {
		ps := pathsOf(
			t,
			read(
				t,
				`<g fill="lime" stroke="red"><rect width="1" height="1" fill="inherit" stroke="inherit"/></g>`,
			),
		)
		if len(ps) != 2 || ps[0].Color != lime || ps[1].Color != red {
			t.Fatalf("got %d paths, want a lime fill and a red outline", len(ps))
		}
	})
}

func TestCurrentColor(t *testing.T) {
	d := read(t, `<g color="red">
		<rect width="1" height="1" fill="currentColor"/>
		<rect width="1" height="1" fill="currentColor" color="BLUE"/>
		<rect width="1" height="1" fill="currentColor" color="nonsense"/>
		<line x2="1" stroke="currentColor" style="color: #00ff00"/>
	</g>`)
	ps := pathsOf(t, d)
	want := []vector.Color{red, blue, red, lime}
	if len(ps) != len(want) {
		t.Fatalf("got %d paths", len(ps))
	}
	for i, w := range want {
		if ps[i].Color != w {
			t.Errorf("path %d colour %v, want %v", i, ps[i].Color, w)
		}
	}
}

func TestCSSPrecedence(t *testing.T) {
	d := read(t, `<style>
		/* tag, then class, then id, then the style attribute, whatever the sheet order */
		#b { fill: blue }
		.a { fill: lime }
		rect { fill: red }
		.hide, circle { display: none }
		* { stroke-width: 4 }
	</style>
	<rect width="1" height="1"/>
	<rect width="1" height="1" fill="white"/>
	<rect width="1" height="1" class="x a"/>
	<rect width="1" height="1" class="a" id="b"/>
	<rect width="1" height="1" class="a" id="b" style="fill: #fff"/>
	<rect width="1" height="1" class="hide"/>
	<circle r="5"/>
	<line x2="1" stroke="black"/>`)
	ps := pathsOf(t, d)
	// A presentation attribute is weaker than any CSS rule.
	want := []vector.Color{red, red, lime, blue, white, vector.Black}
	if len(ps) != len(want) {
		t.Fatalf("got %d paths, want %d", len(ps), len(want))
	}
	for i, w := range want {
		if ps[i].Color != w {
			t.Errorf("path %d colour %v, want %v", i, ps[i].Color, w)
		}
	}
	if ps[5].Width != 4 {
		t.Errorf("universal rule: width %v, want 4", ps[5].Width)
	}
}

func TestHiding(t *testing.T) {
	cases := []struct {
		name, body string
		paths      int
	}{
		{"display none hides the subtree", `<g display="none"><rect width="1" height="1"/></g>`, 0},
		{
			"display none in style",
			`<g style="display:none"><g><rect width="1" height="1"/></g></g>`,
			0,
		},
		{"visibility hidden", `<rect width="1" height="1" visibility="hidden"/>`, 0},
		{"opacity 0", `<g opacity="0"><rect width="1" height="1"/></g>`, 0},
		{"opacity half still draws", `<rect width="1" height="1" opacity="0.5"/>`, 1},
		{"fill-opacity 0 without stroke", `<rect width="1" height="1" fill-opacity="0"/>`, 0},
		{
			"stroke-opacity 0 with stroke only",
			`<rect width="1" height="1" fill="none" stroke="red" stroke-opacity=" 0 "/>`,
			0,
		},
		{"hidden tspan", `<text>a<tspan display="none">b</tspan></text>`, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := read(t, c.body)
			n := 0
			for _, it := range d.Items {
				if _, ok := it.(*vector.Path); ok {
					n++
				}
			}
			if n != c.paths {
				t.Fatalf("drew %d paths, want %d", n, c.paths)
			}
		})
	}
	t.Run("fill-opacity 0 keeps the stroke", func(t *testing.T) {
		p := onePath(t, read(t, `<rect width="1" height="1" fill-opacity="0" stroke="red"/>`))
		if p.Fill || !p.Stroke || p.Color != red {
			t.Fatalf("fill %v stroke %v colour %v", p.Fill, p.Stroke, p.Color)
		}
	})
	t.Run("stroke-opacity 0 keeps the fill", func(t *testing.T) {
		p := onePath(
			t,
			read(t, `<rect width="1" height="1" fill="red" stroke="blue" stroke-opacity="0"/>`),
		)
		if !p.Fill || p.Stroke || p.Color != red {
			t.Fatalf("fill %v stroke %v colour %v", p.Fill, p.Stroke, p.Color)
		}
	})
}

func TestUse(t *testing.T) {
	defs := `<defs>
		<rect id="r" width="10" height="10"/>
		<rect id="moved" transform="translate(100 0)" width="1" height="1"/>
		<symbol id="s"><circle cx="1" cy="1" r="1"/></symbol>
		<symbol id="red-sym" fill="red"><rect width="1" height="1"/></symbol>
	</defs>`
	cases := []struct {
		name, body string
		min, max   vector.Point
	}{
		{"href with offset", `<use href="#r" x="5" y="7"/>`, pt(5, -17), pt(15, -7)},
		{"xlink href", `<use xlink:href="#r" x="1" y="2"/>`, pt(1, -12), pt(11, -2)},
		{
			"use transform then offset",
			`<use href="#r" transform="scale(2)" x="1"/>`,
			pt(2, -20),
			pt(22, 0),
		},
		{"target transform applies inside", `<use href="#moved" x="1"/>`, pt(101, -1), pt(102, 0)},
		{"symbol", `<use href="#s" x="10" y="10"/>`, pt(10, -12), pt(12, -10)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			checkBounds(t, onePath(t, read(t, defs+c.body)), c.min, c.max)
		})
	}
	t.Run("symbol style applies", func(t *testing.T) {
		if p := onePath(t, read(t, defs+`<use href="#red-sym"/>`)); p.Color != red {
			t.Fatalf("colour %v", p.Color)
		}
	})
	t.Run("use style is inherited", func(t *testing.T) {
		if p := onePath(t, read(t, defs+`<use href="#r" fill="blue"/>`)); p.Color != blue {
			t.Fatalf("colour %v", p.Color)
		}
	})
	for name, body := range map[string]string{
		"missing target":    `<use href="#nope"/>`,
		"href without hash": `<use href="r"/>`,
		"external file":     `<use href="other.svg#r"/>`,
		"no href":           `<use/>`,
	} {
		t.Run(name, func(t *testing.T) {
			if d := read(t, defs+body); len(d.Items) != 0 {
				t.Fatalf("drew %d items", len(d.Items))
			}
		})
	}
}

// useChain is uses u1..un, each pointing at the one before, ending in a
// rectangle u0, all inside defs so only the use drawn last counts.
func useChain(n int) string {
	var b strings.Builder
	b.WriteString(`<defs><rect id="u0" width="1" height="1"/>`)
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, `<use id="u%d" href="#u%d"/>`, i, i-1)
	}
	b.WriteString(`</defs>`)
	fmt.Fprintf(&b, `<use href="#u%d"/>`, n)
	return b.String()
}

func TestUseLimits(t *testing.T) {
	t.Run("self reference ends", func(t *testing.T) {
		if d := read(t, `<use id="u" href="#u"/>`); len(d.Items) != 0 {
			t.Fatalf("drew %d items", len(d.Items))
		}
	})
	t.Run("mutual reference ends", func(t *testing.T) {
		d := read(
			t,
			`<g id="a"><rect width="1" height="1"/><use href="#b"/></g><g id="b"><use href="#a"/></g>`,
		)
		// The rectangle is drawn each time a is reached: from a itself after
		// 0, 2, ... maxUses uses and from b after 1, 3, ... maxUses-1.
		if len(d.Items) != maxUses+1 {
			t.Fatalf("drew %d items", len(d.Items))
		}
	})
	// The use drawn at the top counts too, so a chain of n in defs follows
	// n+1 uses.
	t.Run("chain at the limit is drawn", func(t *testing.T) {
		if d := read(t, useChain(maxUses-1)); len(d.Items) != 1 {
			t.Fatalf("drew %d items, want 1", len(d.Items))
		}
	})
	t.Run("chain past the limit is not", func(t *testing.T) {
		if d := read(t, useChain(maxUses)); len(d.Items) != 0 {
			t.Fatalf("drew %d items, want 0", len(d.Items))
		}
	})
	t.Run("depth through uses is bounded", func(t *testing.T) {
		// Each document nests within the parser's limit, but drawing the
		// deep group through a use buried just as deep goes past it.
		const n = maxDepth - 10
		deep := `<defs><g id="deep">` + strings.Repeat("<g>", n) + `<rect width="1" height="1"/>` +
			strings.Repeat("</g>", n) + `</g></defs>`
		d := read(t, deep+strings.Repeat("<g>", n)+`<use href="#deep"/>`+strings.Repeat("</g>", n))
		if len(d.Items) != 0 {
			t.Fatalf("drew %d items past the depth limit", len(d.Items))
		}
	})
}

func TestUseFanOutIsBounded(t *testing.T) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, err := Read(
			strings.NewReader(`<svg><g id="g"><use href="#g"/><use href="#g"/></g></svg>`),
		)
		if !errors.Is(err, ErrTooBig) {
			t.Errorf("a use fan-out is too big: %v", err)
		}
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second): // a hang check; the race detector is slow
		t.Fatal("Read did not return")
	}
}

func TestText(t *testing.T) {
	d := read(t, `<text x="10 20 30" y="20" font-size="10px" fill="red">
		Hello   there
		<tspan dx="5" font-size="20">World</tspan>
		<tspan x="1" y="2" dy="3" text-anchor="middle">Mid</tspan>
		<tspan text-anchor="end" fill="none" stroke="blue">End</tspan>
		<tspan>  </tspan>
		<desc>not a run</desc>
	</text>
	<text font-size="huge">Default</text>
	<text/>`)
	type run struct {
		content string
		at      vector.Point
		height  float64
		align   vector.Align
		color   vector.Color
	}
	want := []run{
		{"Hello there", pt(10, -20), 7, vector.Left, red},
		{"World", pt(15, -20), 14, vector.Left, red},
		{"Mid", pt(1, -5), 7, vector.Center, red},
		{"End", pt(10, -20), 7, vector.Right, blue},
		{"Default", pt(0, 0), 16 * capHeight, vector.Left, vector.Black},
	}
	if len(d.Items) != len(want) {
		t.Fatalf("got %d items, want %d", len(d.Items), len(want))
	}
	for i, w := range want {
		tx, ok := d.Items[i].(*vector.Text)
		if !ok {
			t.Fatalf("item %d is %T", i, d.Items[i])
		}
		if tx.Content != w.content || !nearPt(tx.At, w.at) || !near(tx.Height, w.height) ||
			tx.Align != w.align ||
			tx.Color != w.color {
			t.Errorf("run %d = %q at %v height %v align %v colour %v; want %+v",
				i, tx.Content, tx.At, tx.Height, tx.Align, tx.Color, w)
		}
	}
}

func TestTextScales(t *testing.T) {
	d := read(t, `<g transform="scale(2)"><text font-size="10">x</text></g>`)
	if tx, ok := d.Items[0].(*vector.Text); !ok || !near(tx.Height, 14) {
		t.Fatalf("got %+v, want a text 14 high", d.Items[0])
	}
}

func pngDataURI(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestImage(t *testing.T) {
	uri := pngDataURI(t, 4, 2)
	t.Run("sized", func(t *testing.T) {
		d := read(t, `<image x="10" y="20" width="8" height="6" href="`+uri+`"/>`)
		im, ok := d.Items[0].(*vector.Image)
		if !ok || len(d.Items) != 1 {
			t.Fatalf("items %v", d.Items)
		}
		checkBounds(t, im, pt(10, -26), pt(18, -20))
		if len(d.Notes) != 0 {
			t.Fatalf("notes %q", d.Notes)
		}
	})
	t.Run("natural size", func(t *testing.T) {
		// Base64 broken across lines, as some exporters write it.
		wrapped := uri[:40] + "\n  " + uri[40:]
		d := read(t, `<image xlink:href="`+wrapped+`"/>`)
		checkBounds(t, d.Items[0], pt(0, -2), pt(4, 0))
	})
	t.Run("linked images are counted", func(t *testing.T) {
		d := read(
			t,
			`<image href="plan.png"/><image href="data:text/plain;base64,aGk="/><image href="data:image/png,raw"/>`,
		)
		if len(d.Items) != 0 || len(d.Notes) != 1 ||
			!strings.HasPrefix(d.Notes[0], "3 linked images") {
			t.Fatalf("items %d notes %q", len(d.Items), d.Notes)
		}
	})
	t.Run("broken embedded images are dropped quietly", func(t *testing.T) {
		d := read(
			t,
			`<image href="data:image/png;base64,!!!"/><image href="data:image/png;base64,aGVsbG8="/>`,
		)
		if len(d.Items) != 0 || len(d.Notes) != 0 {
			t.Fatalf("items %d notes %q", len(d.Items), d.Notes)
		}
	})
}

func TestReadUnknownCharsetAsIs(t *testing.T) {
	d := readDoc(
		t,
		`<?xml version="1.0" encoding="x-made-up"?><svg><rect width="1" height="1"/></svg>`,
	)
	if len(d.Items) != 1 {
		t.Fatalf("got %d items", len(d.Items))
	}
}

func TestOneLinkedImageNote(t *testing.T) {
	d := read(t, `<image href="plan.png" width="1" height="1"/>`)
	if len(d.Notes) != 1 ||
		d.Notes[0] != "1 linked image was left out; only embedded ones are drawn." {
		t.Fatalf("%q", d.Notes)
	}
}
