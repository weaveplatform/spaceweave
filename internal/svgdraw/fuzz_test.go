package svgdraw

import (
	"strings"
	"testing"
)

// FuzzRead checks that no document makes Read panic. Hostile or broken
// files arrive through the upload API, so a panic here would take down a
// conversion worker rather than reject one file.
func FuzzRead(f *testing.F) {
	for _, seed := range []string{
		`<svg/>`,
		`<svg width="210mm" viewBox="0 0 210 297"><rect x="1" y="2" width="3" height="4" fill="red" stroke="#00f"/></svg>`,
		`<svg><path d="M0 0 L10 0 Q5 5 0 10 T-5 -5 C1 2 3 4 5 6 S7 8 9 10 A5 5 30 1 0 20 20 h1v1z m1.5.5l1e-3-2"/></svg>`,
		`<svg><style>.a{fill:lime} #b{display:none} rect{stroke:currentColor}</style>` +
			`<g transform="rotate(30 5 5) skewX(10) matrix(1 0 0 1 2 3)" color="rgb(10%,20,30)"><rect class="a" width="1" height="1"/></g></svg>`,
		`<svg><defs><symbol id="s"><circle r="2"/></symbol></defs><use href="#s" x="1"/><use id="u" xlink:href="#u"/></svg>`,
		`<svg><text x="1 2" y="3" font-size="12px" text-anchor="middle">a<tspan dx="1">b</tspan></text>` +
			`<image href="data:image/png;base64,iVBORw0KGgo="/><image href="x.png"/></svg>`,
		`<svg><polygon points="0,0 1,1 2"/><polyline points="1e"/><ellipse rx="1" ry="2"/><line x2="1" stroke="red"/></svg>`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, doc string) {
		d, err := Read(strings.NewReader(doc))
		if (err == nil) == (d == nil) {
			t.Fatalf("Read = %v, %v: want exactly one of a drawing and an error", d, err)
		}
	})
}
