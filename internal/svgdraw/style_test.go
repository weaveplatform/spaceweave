package svgdraw

import (
	"math"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/vector"
)

func TestParsePaint(t *testing.T) {
	current := vector.Color{R: 1, G: 2, B: 3}
	cases := []struct {
		in   string
		want paint
	}{
		{"", paint{}},
		{"inherit", paint{}},
		{"bogus", paint{}},
		{"none", paint{set: true, none: true}},
		{" Transparent ", paint{set: true, none: true}},
		{"currentColor", paint{set: true, color: current}},
		{"url(#grad)", paint{set: true, color: gradientGrey}},
		{"RED", paint{set: true, color: vector.Color{R: 255}}},
		{"#0f0", paint{set: true, color: vector.Color{G: 255}}},
	}
	for _, c := range cases {
		if got := parsePaint(c.in, current); got != c.want {
			t.Errorf("parsePaint(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestParseColor(t *testing.T) {
	cases := []struct {
		in   string
		want vector.Color
		ok   bool
	}{
		{"black", vector.Color{}, true},
		{"grey", vector.Color{R: 128, G: 128, B: 128}, true},
		{"orange", vector.Color{R: 255, G: 165}, true},
		{"#abc", vector.Color{R: 0xaa, G: 0xbb, B: 0xcc}, true},
		// The alpha digit of #rgba and the alpha byte of #rrggbbaa are dropped.
		{"#abcd", vector.Color{R: 0xaa, G: 0xbb, B: 0xcc}, true},
		{"#123456", vector.Color{R: 0x12, G: 0x34, B: 0x56}, true},
		{"#12345678", vector.Color{R: 0x12, G: 0x34, B: 0x56}, true},
		{"#12345", vector.Color{}, false},
		{"#ggg", vector.Color{}, false},
		{"#", vector.Color{}, false},
		{"rgb(10, 20, 30)", vector.Color{R: 10, G: 20, B: 30}, true},
		{"rgb(10 20 30 / 0.5)", vector.Color{R: 10, G: 20, B: 30}, true},
		{"rgba(10,20,30,0.5)", vector.Color{R: 10, G: 20, B: 30}, true},
		{"rgb(100%, 0%, 20%)", vector.Color{R: 255, G: 0, B: 51}, true},
		// Out-of-range channels clamp rather than wrap.
		{"rgb(300, -5, 255)", vector.Color{R: 255, G: 0, B: 255}, true},
		{"rgb(1,2)", vector.Color{}, false},
		{"rgb(a,b,c)", vector.Color{}, false},
		{"rgb(1,2,3", vector.Color{}, false},
		{"rgb)1,2,3(", vector.Color{}, false},
		{"chartreuse", vector.Color{}, false},
	}
	for _, c := range cases {
		got, ok := parseColor(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("parseColor(%q) = %+v, %v; want %+v, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestParseTransform(t *testing.T) {
	s2 := math.Sqrt2 / 2
	cases := []struct {
		name, in string
		from, to vector.Point
	}{
		{"none", "", pt(3, 4), pt(3, 4)},
		{"matrix", "matrix(1 2 3 4 5 6)", pt(1, 1), pt(1+3+5, 2+4+6)},
		{"matrix with the wrong arity is ignored", "matrix(1 2 3)", pt(1, 1), pt(1, 1)},
		{"translate", "translate(5,6)", pt(1, 1), pt(6, 7)},
		{"translate x only", "translate(5)", pt(1, 1), pt(6, 1)},
		{"scale uniform", "scale(2)", pt(1, 3), pt(2, 6)},
		{"scale", "scale(2 3)", pt(1, 1), pt(2, 3)},
		{"scale default", "scale()", pt(1, 3), pt(1, 3)},
		{"rotate", "rotate(90)", pt(1, 0), pt(0, 1)},
		{"rotate 45", "rotate(45)", pt(1, 0), pt(s2, s2)},
		{"rotate about a centre", "rotate(90 10 10)", pt(20, 10), pt(10, 20)},
		{"rotate centre is fixed", "rotate(37, 10, 10)", pt(10, 10), pt(10, 10)},
		{"skewX", "skewX(45)", pt(0, 1), pt(1, 1)},
		{"skewY", "skewY(45)", pt(1, 0), pt(1, 1)},
		{"unknown is ignored", "perspective(3)", pt(1, 2), pt(1, 2)},
		{"unclosed is ignored", "translate(5", pt(1, 2), pt(1, 2)},
		{"a non-number ends the arguments", "translate(5 x 7)", pt(1, 2), pt(6, 2)},
		{"a bad number ends the arguments", "translate(5 - 7)", pt(1, 2), pt(6, 2)},
		// The rightmost transform applies first: scale, then translate.
		{"composed", "translate(10,0) scale(2)", pt(1, 1), pt(12, 2)},
		{"composed other way", "scale(2), translate(10,0)", pt(1, 1), pt(22, 2)},
		{"composed three", "translate(1 1)\nrotate(90)\tscale(2)", pt(1, 0), pt(1, 3)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseTransform(c.in).Apply(c.from); !nearPt(got, c.to) {
				t.Fatalf("%s maps %v to %v, want %v", c.in, c.from, got, c.to)
			}
		})
	}
}

func TestLength(t *testing.T) {
	cases := []struct {
		in   string
		v    float64
		unit string
		ok   bool
	}{
		{"12", 12, "", true},
		{" 12.5PX ", 12.5, "px", true},
		{"210mm", 210, "mm", true},
		{"-3 cm", -3, "cm", true},
		{"1e3", 1000, "", true},
		{"50%", 50, "%", true},
		{"", 0, "", false},
		{"mm", 0, "", false},
		{"1e400", 0, "", false},
	}
	for _, c := range cases {
		v, unit, ok := length(c.in)
		if v != c.v || unit != c.unit || ok != c.ok {
			t.Errorf(
				"length(%q) = %v, %q, %v; want %v, %q, %v",
				c.in,
				v,
				unit,
				ok,
				c.v,
				c.unit,
				c.ok,
			)
		}
	}
	if num("7in") != 7 || num("x") != 0 {
		t.Errorf("num drops the unit and reads garbage as 0")
	}
}

func TestPoints(t *testing.T) {
	cases := []struct {
		in   string
		want []vector.Point
	}{
		{"0,0 10,10 20-5", []vector.Point{pt(0, 0), pt(10, 10), pt(20, -5)}},
		// An odd coordinate out has no partner and is dropped.
		{"0 0 10 10 20", []vector.Point{pt(0, 0), pt(10, 10)}},
		{"0 0 1e", []vector.Point{pt(0, 0)}},
		{"", nil},
	}
	for _, c := range cases {
		got := points(c.in)
		if len(got) != len(c.want) {
			t.Fatalf("points(%q) = %v, want %v", c.in, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("points(%q) = %v, want %v", c.in, got, c.want)
			}
		}
	}
}

func TestParseStyle(t *testing.T) {
	p := map[string]string{"fill": "red"}
	parseStyle(" Fill : blue !important ; stroke:none;junk; ;stroke-width: 2", p)
	want := map[string]string{"fill": "blue", "stroke": "none", "stroke-width": "2"}
	if len(p) != len(want) {
		t.Fatalf("got %v, want %v", p, want)
	}
	for k, v := range want {
		if p[k] != v {
			t.Fatalf("%s = %q, want %q (%v)", k, p[k], v, p)
		}
	}
}

func TestParseCSS(t *testing.T) {
	css := `/* a comment { with braces } */
		rect, .wall,#door { fill: red }
		g > rect { fill: blue }
		a:hover { fill: blue }
		[x] { fill: blue }
		.a .b { fill: blue }
		* { stroke: black }
		/* unterminated`
	rules := parseCSS(css)
	var sels []string
	for _, r := range rules {
		sels = append(sels, r.sel)
	}
	want := []string{"rect", ".wall", "#door", "*"}
	if len(sels) != len(want) {
		t.Fatalf("selectors %q, want %q", sels, want)
	}
	for i := range want {
		if sels[i] != want[i] {
			t.Fatalf("selectors %q, want %q", sels, want)
		}
	}
	if rules[0].props["fill"] != "red" || rules[3].props["stroke"] != "black" {
		t.Fatalf("rule bodies %v", rules)
	}
	if got := parseCSS("rect { fill: red"); len(got) != 0 {
		t.Fatalf("an unclosed rule gave %v", got)
	}
	if got := parseCSS("} rect {"); len(got) != 0 {
		t.Fatalf("a close before an open gave %v", got)
	}
}

func TestStripComments(t *testing.T) {
	cases := map[string]string{
		"a/*x*/b/*y*/c": "abc",
		"a/* open":      "a",
		"plain":         "plain",
	}
	for in, want := range cases {
		if got := stripComments(in); got != want {
			t.Errorf("stripComments(%q) = %q, want %q", in, got, want)
		}
	}
}
