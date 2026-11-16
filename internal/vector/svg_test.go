package vector

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"strconv"
	"strings"
	"testing"
)

// element is one parsed SVG element: its name, attributes and text.
type element struct {
	name  string
	attrs map[string]string
	text  string
}

// parseSVG checks the output is well-formed XML and returns its elements in
// document order.
func parseSVG(t *testing.T, data []byte) []element {
	t.Helper()
	dec := xml.NewDecoder(bytes.NewReader(data))
	var out []element
	open := -1
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("not well-formed XML: %v\n%s", err, data)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			e := element{name: tok.Name.Local, attrs: map[string]string{}}
			for _, a := range tok.Attr {
				e.attrs[a.Name.Local] = a.Value
			}
			out = append(out, e)
			open = len(out) - 1
		case xml.CharData:
			if open >= 0 {
				out[open].text += string(tok)
			}
		case xml.EndElement:
			open = -1
		}
	}
	if len(out) == 0 || out[0].name != "svg" {
		t.Fatalf("root is not <svg>: %s", data)
	}
	return out
}

func writeSVG(t *testing.T, d *Drawing, b Rect) []element {
	t.Helper()
	var buf bytes.Buffer
	if err := WriteSVG(&buf, d, b); err != nil {
		t.Fatal(err)
	}
	return parseSVG(t, buf.Bytes())
}

func floats(t *testing.T, s string) []float64 {
	t.Helper()
	fs := strings.Fields(s)
	out := make([]float64, len(fs))
	for i, f := range fs {
		v, err := strconv.ParseFloat(f, 64)
		if err != nil {
			t.Fatalf("%q: %v", s, err)
		}
		out[i] = v
	}
	return out
}

func byName(els []element, name string) []element {
	var out []element
	for _, e := range els {
		if e.name == name {
			out = append(out, e)
		}
	}
	return out
}

func TestWriteSVGViewBoxAndBackground(t *testing.T) {
	d := &Drawing{Items: []Item{line(Point{X: 0, Y: 0}, Point{X: 100, Y: 50})}}
	els := writeSVG(t, d, Rect{})
	if ns := els[0].attrs["xmlns"]; ns != "http://www.w3.org/2000/svg" {
		t.Errorf("xmlns = %q", ns)
	}
	// 2% padding of the long side, and y flipped: drawing y 0..50 becomes
	// SVG y -50..0.
	vb := floats(t, els[0].attrs["viewBox"])
	want := []float64{-2, -52, 104, 54}
	if len(vb) != 4 {
		t.Fatalf("viewBox %v", vb)
	}
	for i := range want {
		if !near(vb[i], want[i], 1e-9) {
			t.Errorf("viewBox %v, want %v", vb, want)
			break
		}
	}
	bg := els[1]
	if bg.name != "rect" || bg.attrs["fill"] != "#fff" {
		t.Fatalf("first child %+v, want a white rect", bg)
	}
	got := floats(t, bg.attrs["x"]+" "+bg.attrs["y"]+" "+bg.attrs["width"]+" "+bg.attrs["height"])
	for i := range want {
		if !near(got[i], want[i], 1e-9) {
			t.Errorf("background %v, want the viewBox %v", got, want)
			break
		}
	}
}

func TestWriteSVGBoundsArgument(t *testing.T) {
	// An explicit bounds (the raster's, after trimming) wins over the
	// drawing's own extent.
	d := &Drawing{Items: []Item{line(Point{X: 0, Y: 0}, Point{X: 1000, Y: 1000})}}
	b := Rect{}.Extend(Point{X: 0, Y: 0}).Extend(Point{X: 10, Y: 10})
	vb := floats(t, writeSVG(t, d, b)[0].attrs["viewBox"])
	if !near(vb[2], 10.4, 1e-9) || !near(vb[3], 10.4, 1e-9) {
		t.Errorf("viewBox %v, want 10.4 square", vb)
	}
}

func TestWriteSVGPaths(t *testing.T) {
	hair := line(Point{X: 0, Y: 0}, Point{X: 1600, Y: 0})
	hair.Color = Color{R: 0xff, G: 0x80}
	wide := line(Point{X: 0, Y: 10}, Point{X: 100, Y: 10})
	wide.Width = 3
	filled := &Path{Fill: true, Color: Color{R: 1, G: 2, B: 3}}
	filled.MoveTo(Point{X: 0, Y: 0})
	filled.CubicTo(Point{X: 0, Y: 5}, Point{X: 5, Y: 5}, Point{X: 5, Y: 0})
	filled.Close()
	invisible := line(Point{}, Point{X: 1})
	invisible.Stroke = false
	d := &Drawing{Items: []Item{hair, wide, filled, invisible, &Path{Stroke: true}}}
	paths := byName(writeSVG(t, d, Rect{}), "path")
	if len(paths) != 3 {
		t.Fatalf("%d paths, want 3 (unpainted and empty ones skipped)", len(paths))
	}

	// A widthless line is drawn at the hairline, 1/1600 of the long side.
	h := paths[0]
	if h.attrs["stroke"] != "#ff8000" || h.attrs["fill"] != "none" {
		t.Errorf("hairline attrs %v", h.attrs)
	}
	if w := floats(t, h.attrs["stroke-width"])[0]; !near(w, 1, 1e-9) {
		t.Errorf("hairline width %v, want 1", w)
	}
	// SVG's y runs down, so drawing y=10 is written as -10.
	if d := paths[1].attrs["d"]; d != "M0 -10L100 -10" {
		t.Errorf("wide line d = %q", d)
	}
	if w := floats(t, paths[1].attrs["stroke-width"])[0]; w != 3 {
		t.Errorf("wide line width %v, want 3", w)
	}

	f := paths[2]
	if f.attrs["fill"] != "#010203" || f.attrs["fill-rule"] != "evenodd" {
		t.Errorf("fill attrs %v", f.attrs)
	}
	if _, ok := f.attrs["stroke"]; ok {
		t.Error("fill-only path has a stroke")
	}
	if d := f.attrs["d"]; d != "M0 0C0 -5 5 -5 5 0Z" {
		t.Errorf("filled d = %q", d)
	}
}

func TestWriteSVGText(t *testing.T) {
	d := &Drawing{Items: []Item{
		&Text{
			At: Point{X: 10, Y: 20}, Height: 7, Content: `A & <B> "c"`, Align: Center, VAlign: Top,
			Angle: math.Pi / 2, WidthFactor: 2, Color: Color{R: 0x12, G: 0x34, B: 0x56},
		},
		&Text{At: Point{X: 0, Y: 0}, Height: 7, Content: "plain"},
		&Text{At: Point{X: 5, Y: 5}, Height: 7, Content: ""},
		&Text{At: Point{X: 5, Y: 5}, Height: 0, Content: "zero"},
	}}
	texts := byName(writeSVG(t, d, Rect{}), "text")
	if len(texts) != 2 {
		t.Fatalf("%d texts, want 2 (empty and zero-height skipped)", len(texts))
	}
	a := texts[0]
	if a.text != `A & <B> "c"` {
		t.Errorf("content %q did not survive escaping", a.text)
	}
	// Height is the cap height; font-size is the em, about 1/0.7 of it.
	if fs := floats(t, a.attrs["font-size"])[0]; !near(fs, 10, 1e-6) {
		t.Errorf("font-size %v, want 10", fs)
	}
	if a.attrs["text-anchor"] != "middle" || a.attrs["dominant-baseline"] != "text-before-edge" ||
		a.attrs["fill"] != "#123456" {
		t.Errorf("attrs %v", a.attrs)
	}
	// Counter-clockwise in drawing space is a negative SVG rotation.
	if tr := a.attrs["transform"]; tr != "translate(10 -20) rotate(-90) scale(2 1)" {
		t.Errorf("transform %q", tr)
	}
	b := texts[1]
	if b.attrs["text-anchor"] != "start" || b.attrs["dominant-baseline"] != "alphabetic" {
		t.Errorf("default alignment %v", b.attrs)
	}
	if strings.Contains(b.attrs["transform"], "scale") {
		t.Errorf("WidthFactor 0 still scaled: %q", b.attrs["transform"])
	}
}

func TestWriteSVGTextAlignments(t *testing.T) {
	for _, tc := range []struct {
		a      Align
		v      VAlign
		anchor string
		base   string
	}{
		{Right, Bottom, "end", "text-after-edge"},
		{Left, Middle, "start", "central"},
	} {
		d := &Drawing{Items: []Item{&Text{Height: 1, Content: "x", Align: tc.a, VAlign: tc.v}}}
		e := byName(writeSVG(t, d, Rect{}), "text")[0]
		if e.attrs["text-anchor"] != tc.anchor || e.attrs["dominant-baseline"] != tc.base {
			t.Errorf("align %d/%d: %v", tc.a, tc.v, e.attrs)
		}
	}
}

func TestWriteSVGImage(t *testing.T) {
	src := image.NewGray(image.Rect(0, 0, 3, 2))
	src.SetGray(1, 0, color.Gray{Y: 200})
	im := &Image{Img: src, M: Scale(10, -10).Then(Translate(0, 20))}
	imgs := byName(writeSVG(t, &Drawing{Items: []Item{im}}, Rect{}), "image")
	if len(imgs) != 1 {
		t.Fatalf("%d images", len(imgs))
	}
	e := imgs[0]
	if e.attrs["width"] != "3" || e.attrs["height"] != "2" {
		t.Errorf("size %sx%s", e.attrs["width"], e.attrs["height"])
	}
	// M then the y flip: pixel (0,0) lands at drawing (0,20), SVG (0,-20).
	if tr := e.attrs["transform"]; tr != "matrix(10 0 0 10 0 -20)" {
		t.Errorf("transform %q", tr)
	}
	href := e.attrs["href"]
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(href, prefix) {
		t.Fatalf("href %.40q is not a PNG data URI", href)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(href, prefix))
	if err != nil {
		t.Fatal(err)
	}
	got, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds() != src.Bounds() {
		t.Fatalf("embedded image %v", got.Bounds())
	}
	if y := color.GrayModel.Convert(got.At(1, 0)).(color.Gray).Y; y != 200 { //nolint:forcetypeassert // GrayModel returns Gray
		t.Errorf("embedded pixel %d, want 200", y)
	}
}

func TestWriteSVGPointDrawing(t *testing.T) {
	// A drawing with no size must still get a positive viewBox; a zero one
	// makes renderers draw nothing.
	p := &Path{Stroke: true}
	p.MoveTo(Point{X: 3, Y: 3})
	vb := floats(t, writeSVG(t, &Drawing{Items: []Item{p}}, Rect{})[0].attrs["viewBox"])
	if vb[2] <= 0 || vb[3] <= 0 {
		t.Errorf("viewBox %v", vb)
	}
}

func TestWriteSVGEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteSVG(&buf, &Drawing{Items: []Item{&Path{}}}, Rect{}); !errors.Is(err, ErrEmpty) {
		t.Errorf("err = %v, want ErrEmpty", err)
	}
	if buf.Len() != 0 {
		t.Errorf("wrote %d bytes for an empty drawing", buf.Len())
	}
}

func TestWriteSVGImageEncodeError(t *testing.T) {
	// A zero-size image can't be a PNG; the error must surface rather than
	// leave a broken <image>.
	bad := &Image{Img: image.NewGray(image.Rect(0, 0, 0, 0)), M: Identity}
	d := &Drawing{Items: []Item{line(Point{}, Point{X: 1, Y: 1}), bad}}
	err := WriteSVG(io.Discard, d, Rect{})
	if err == nil || !strings.Contains(err.Error(), "embed image") {
		t.Errorf("err = %v, want an embed image error", err)
	}
}

var errFull = errors.New("disk full")

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errFull }

func TestWriteSVGWriteError(t *testing.T) {
	d := &Drawing{Items: []Item{line(Point{}, Point{X: 1, Y: 1})}}
	if err := WriteSVG(failWriter{}, d, Rect{}); !errors.Is(err, errFull) {
		t.Errorf("err = %v, want the writer's error", err)
	}
}

func TestWriteSVGTextControlCharacter(t *testing.T) {
	// html.EscapeString leaves control characters alone, and XML 1.0 forbids
	// them; a DXF text carrying one (e.g. a stray \x01 from a code page
	// conversion) makes the whole SVG unreadable.
	d := &Drawing{Items: []Item{&Text{Height: 1, Content: "a\x01b"}}}
	writeSVG(t, d, Rect{})
}
