package svgdraw

import (
	"math"
	"strconv"
	"strings"

	"github.com/weaveplatform/spaceweave/internal/vector"
)

// paint is a fill or stroke: none, or a colour.
type paint struct {
	set   bool
	none  bool
	color vector.Color
}

var named = map[string]vector.Color{
	"black": {}, "white": {R: 255, G: 255, B: 255}, "red": {R: 255}, "green": {G: 128},
	"lime": {G: 255}, "blue": {B: 255}, "yellow": {R: 255, G: 255}, "cyan": {G: 255, B: 255},
	"aqua": {G: 255, B: 255}, "magenta": {R: 255, B: 255}, "fuchsia": {R: 255, B: 255},
	"gray": {R: 128, G: 128, B: 128}, "grey": {R: 128, G: 128, B: 128},
	"silver": {R: 192, G: 192, B: 192}, "lightgray": {R: 211, G: 211, B: 211},
	"lightgrey": {R: 211, G: 211, B: 211}, "darkgray": {R: 169, G: 169, B: 169},
	"darkgrey": {R: 169, G: 169, B: 169}, "maroon": {R: 128}, "navy": {B: 128},
	"olive": {R: 128, G: 128}, "purple": {R: 128, B: 128}, "teal": {G: 128, B: 128},
	"orange": {R: 255, G: 165}, "brown": {R: 165, G: 42, B: 42},
}

// gradientGrey stands in for a gradient or pattern fill: something is there,
// and it is lighter than ink.
var gradientGrey = vector.Color{R: 200, G: 200, B: 200}

// parsePaint reads a fill or stroke value; current is the color property.
func parsePaint(v string, current vector.Color) paint {
	v = strings.ToLower(strings.TrimSpace(v))
	switch {
	case v == "" || v == "inherit":
		return paint{}
	case v == "none" || v == "transparent":
		return paint{set: true, none: true}
	case v == "currentcolor":
		return paint{set: true, color: current}
	case strings.HasPrefix(v, "url("):
		return paint{set: true, color: gradientGrey}
	}
	if c, ok := parseColor(v); ok {
		return paint{set: true, color: c}
	}
	return paint{}
}

func parseColor(v string) (vector.Color, bool) {
	if c, ok := named[v]; ok {
		return c, true
	}
	if strings.HasPrefix(v, "#") {
		h := v[1:]
		if len(h) == 3 || len(h) == 4 {
			h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
		}
		if len(h) >= 6 {
			n, err := strconv.ParseUint(h[:6], 16, 32)
			if err == nil {
				// The bytes of a 24-bit value, so each conversion keeps the
				// low byte by design.
				return vector.Color{
					R: uint8(n >> 16), //nolint:gosec // see above
					G: uint8(n >> 8),  //nolint:gosec // see above
					B: uint8(n),       //nolint:gosec // see above
				}, true
			}
		}
		return vector.Color{}, false
	}
	if strings.HasPrefix(v, "rgb") {
		open, end := strings.IndexByte(v, '('), strings.IndexByte(v, ')')
		if open < 0 || end < open {
			return vector.Color{}, false
		}
		parts := strings.FieldsFunc(
			v[open+1:end],
			func(r rune) bool { return r == ',' || r == ' ' || r == '/' },
		)
		if len(parts) < 3 {
			return vector.Color{}, false
		}
		var ch [3]uint8
		for i := range ch {
			p := parts[i]
			f, err := strconv.ParseFloat(strings.TrimSuffix(p, "%"), 64)
			if err != nil {
				return vector.Color{}, false
			}
			if strings.HasSuffix(p, "%") {
				f *= 2.55
			}
			ch[i] = uint8(math.Round(math.Max(0, math.Min(255, f))))
		}
		return vector.Color{R: ch[0], G: ch[1], B: ch[2]}, true
	}
	return vector.Color{}, false
}

// parseStyle splits a style attribute or a CSS rule body into properties.
func parseStyle(s string, into map[string]string) {
	for decl := range strings.SplitSeq(s, ";") {
		k, v, ok := strings.Cut(decl, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(v), "!important"))
		into[strings.ToLower(strings.TrimSpace(k))] = v
	}
}

// rule is a CSS rule with a simple selector: a tag, .class or #id.
type rule struct {
	sel   string
	props map[string]string
}

// parseCSS reads a <style> sheet. Only simple selectors apply; anything
// with combinators or pseudo-classes is ignored, which costs little in
// drawings exported from CAD and illustration tools.
func parseCSS(css string) []rule {
	var out []rule
	css = stripComments(css)
	for {
		open := strings.IndexByte(css, '{')
		end := strings.IndexByte(css, '}')
		if open < 0 || end < open {
			return out
		}
		props := map[string]string{}
		parseStyle(css[open+1:end], props)
		for sel := range strings.SplitSeq(css[:open], ",") {
			sel = strings.TrimSpace(sel)
			if sel != "" && !strings.ContainsAny(sel, " >+~:[") {
				out = append(out, rule{sel: sel, props: props})
			}
		}
		css = css[end+1:]
	}
}

func stripComments(s string) string {
	for {
		a := strings.Index(s, "/*")
		if a < 0 {
			return s
		}
		b := strings.Index(s[a+2:], "*/")
		if b < 0 {
			return s[:a]
		}
		s = s[:a] + s[a+2+b+2:]
	}
}

// parseTransform reads a transform attribute. SVG lists transforms outer
// first, so each one applies before the ones to its left.
func parseTransform(s string) vector.Affine {
	m := vector.Identity
	for {
		s = strings.TrimLeft(s, " ,\t\n\r")
		open := strings.IndexByte(s, '(')
		end := strings.IndexByte(s, ')')
		if open < 0 || end < open {
			return m
		}
		name := strings.TrimSpace(s[:open])
		sc := &pathScanner{s: s[open+1 : end]}
		var a []float64
		for sc.hasNumber() {
			v, err := sc.number()
			if err != nil {
				break
			}
			a = append(a, v)
		}
		m = one(name, a).Then(m)
		s = s[end+1:]
	}
}

func one(name string, a []float64) vector.Affine {
	arg := func(i int, def float64) float64 {
		if i < len(a) {
			return a[i]
		}
		return def
	}
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	switch name {
	case "matrix":
		if len(a) == 6 {
			return vector.Affine{A: a[0], B: a[1], C: a[2], D: a[3], E: a[4], F: a[5]}
		}
	case "translate":
		return vector.Translate(arg(0, 0), arg(1, 0))
	case "scale":
		return vector.Scale(arg(0, 1), arg(1, arg(0, 1)))
	case "rotate":
		cx, cy := arg(1, 0), arg(2, 0)
		return vector.Translate(-cx, -cy).
			Then(vector.Rotate(rad(arg(0, 0)))).
			Then(vector.Translate(cx, cy))
	case "skewX":
		return vector.Affine{A: 1, C: math.Tan(rad(arg(0, 0))), D: 1}
	case "skewY":
		return vector.Affine{A: 1, B: math.Tan(rad(arg(0, 0))), D: 1}
	}
	return vector.Identity
}

// metresPer are the absolute CSS units.
var metresPer = map[string]float64{
	"mm": 0.001, "cm": 0.01, "in": 0.0254, "pt": 0.0254 / 72, "pc": 0.0254 / 6, "q": 0.00025,
}

// length reads a length; unit is "" for user units or px.
func length(s string) (v float64, unit string, ok bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	i := len(s)
	for i > 0 && (s[i-1] < '0' || s[i-1] > '9') && s[i-1] != '.' {
		i--
	}
	f, err := strconv.ParseFloat(s[:i], 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, "", false
	}
	return f, strings.TrimSpace(s[i:]), true
}

func num(s string) float64 {
	v, _, _ := length(s)
	return v
}

// points reads a polyline's or polygon's points.
func points(s string) []vector.Point {
	sc := &pathScanner{s: s}
	var out []vector.Point
	for sc.hasNumber() {
		x, err := sc.number()
		if err != nil {
			break
		}
		y, err := sc.number()
		if err != nil {
			break
		}
		out = append(out, vector.Point{X: x, Y: y})
	}
	return out
}
