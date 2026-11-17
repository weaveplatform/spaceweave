package rasterfront

import (
	"math"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/autotrace/score"
	"github.com/weaveplatform/spaceweave/internal/autotrace/synth"
)

// Walls drawn as their faces, or hatched, are filled and read as walls.
func TestReadOutlinedAndHatchedWalls(t *testing.T) {
	tr := fixture(t, "office")
	for _, style := range []synth.WallStyle{synth.Outline, synth.Hatch} {
		so := everything
		so.Walls = style
		in := readRender(t, tr, so, Options{})
		if !hasNote(in, "were filled") {
			t.Errorf("style %d: notes %v", style, in.Notes)
		}
		r, p := score.Walls(asTrace(in, true), tr, tolPx)
		if r < 0.95 || p < 0.85 {
			t.Errorf("style %d: recall %.3f precision %.3f, want ≥ 0.95, ≥ 0.85", style, r, p)
		}
	}
	// Solid walls are left as they are.
	if in := readRender(t, tr, everything, Options{}); hasNote(in, "were filled") {
		t.Errorf("solid walls filled: %v", in.Notes)
	}
}

func TestExteriorWidth(t *testing.T) {
	// Panes narrower than the wall outnumber it; a stair far wider is
	// dropped; the widest tenth by length is the wall's 9 px.
	ext := []strip{{4, 500}, {4, 300}, {9, 100}, {40, 50}, {5, 100}}
	if w := exteriorWidth(ext); w != 9 {
		t.Errorf("width %v, want 9", w)
	}
	// Strips of no length leave the widest.
	if w := exteriorWidth([]strip{{3, 0}, {5, 0}, {7, 0}}); w != 7 {
		t.Errorf("width %v, want 7", w)
	}
}

func TestWallWidthWithoutStrips(t *testing.T) {
	// One room in a frame: no strip, so no wall width and nothing filled.
	w, h := 40, 30
	ink := make([]bool, w*h)
	for y := range h {
		for x := range w {
			ink[y*w+x] = x == 5 || x == 34 || y == 5 || y == 24
		}
	}
	if out, n := fillWalls(w, h, ink, 0); n != 0 || &out[0] != &ink[0] {
		t.Errorf("filled %d px", n)
	}
	label, rs := regions(w, h, ink)
	measureRegions(w, h, ink, label, rs)
	if ww := wallWidth(w, h, ink, label, rs); ww != 0 {
		t.Errorf("wall width %v", ww)
	}
	if out, n := fillWalls(0, 0, nil, 0); n != 0 || out != nil {
		t.Error("an empty image filled")
	}
}

// A dense strip is a room's worth of floor, not the walls' outline network.
func TestNetworkWidth(t *testing.T) {
	g := region{area: 400, ridge: 100, width: 4, maxD: 4, box: [4]int{0, 0, 99, 3}}
	if _, ok := networkWidth([]region{g}); ok {
		t.Error("a solid bar taken as a network")
	}
	g.box = [4]int{0, 0, 99, 99}
	if ww, ok := networkWidth([]region{g}); !ok || ww != 4 {
		t.Errorf("network width %v %v", ww, ok)
	}
	if _, ok := networkWidth(nil); ok {
		t.Error("no strips taken as a network")
	}
	if w := (region{maxD: 9}).widest(); math.Abs(w-5) > 1e-9 {
		t.Errorf("widest %v", w)
	}
}

func TestCount(t *testing.T) {
	if s := count(1, "wall", "walls"); s != "1 wall" {
		t.Error(s)
	}
	if s := count(3, "wall", "walls"); s != "3 walls" {
		t.Error(s)
	}
}

func TestCrossesOffTheMask(t *testing.T) {
	m := mask{w: 4, h: 4, px: make([]bool, 16)}
	m.px[5] = true
	if m.crosses(horizontal, -1, 0, 3) || m.crosses(horizontal, 4, 0, 3) {
		t.Error("a line off the mask crosses")
	}
	if !m.crosses(horizontal, 1, 0, 3) || m.crosses(horizontal, 2, 0, 3) {
		t.Error("crosses misses or invents a pixel")
	}
}
