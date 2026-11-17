package rasterfront

import (
	"errors"
	"image"
	"math"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/autotrace/synth"
	"github.com/weaveplatform/spaceweave/internal/floorgraph"
	"github.com/weaveplatform/spaceweave/internal/plan"
	"github.com/weaveplatform/spaceweave/internal/raster"
)

func gray(w, h int, v uint8) *image.Gray {
	g := image.NewGray(image.Rect(0, 0, w, h))
	for i := range g.Pix {
		g.Pix[i] = v
	}
	return g
}

func hasNote(in floorgraph.Input, s string) bool {
	for _, n := range in.Notes {
		if strings.Contains(n, s) {
			return true
		}
	}
	return false
}

func TestReadErrors(t *testing.T) {
	if _, err := Read(nil, Options{}); !errors.Is(err, ErrNoImage) {
		t.Errorf("nil source: %v", err)
	}
	if _, err := Read(&raster.Source{}, Options{}); !errors.Is(err, ErrNoImage) {
		t.Errorf("no grey image: %v", err)
	}
	if _, err := Read(raster.NewSource(gray(50, 50, 255)), Options{}); !errors.Is(err, ErrNoInk) {
		t.Errorf("blank: %v", err)
	}
	if _, err := Read(
		raster.NewSource(gray(50, 50, 0)),
		Options{MaxPixels: 100},
	); !errors.Is(
		err,
		ErrTooLarge,
	) {
		t.Errorf("too large: %v", err)
	}
}

func TestReadDegenerateImages(t *testing.T) {
	noise := gray(120, 90, 255)
	rng := rand.New(rand.NewPCG(1, 2))
	for i := range noise.Pix {
		noise.Pix[i] = uint8(rng.IntN(256))
	}
	tiny := gray(3, 2, 255)
	tiny.Pix[0] = 0
	for name, g := range map[string]*image.Gray{
		"all black": gray(60, 40, 0),
		"one pixel": gray(1, 1, 0),
		"tiny":      tiny,
		"noise":     noise,
	} {
		in, err := Read(raster.NewSource(g), Options{})
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if in.Size != [2]int{g.Rect.Dx(), g.Rect.Dy()} {
			t.Errorf("%s: size %v", name, in.Size)
		}
		if in.Coverage < 0 || in.Coverage > 1 {
			t.Errorf("%s: coverage %v", name, in.Coverage)
		}
	}
	in, _ := Read(raster.NewSource(gray(60, 40, 0)), Options{})
	if !hasNote(in, "over half the image is ink") {
		t.Errorf("all black: notes %v", in.Notes)
	}
}

// A plan drawn only in hairlines has no walls this front end can read; it
// says so rather than failing.
func TestReadHairlinesOnly(t *testing.T) {
	tr := synth.New(40).
		Wall("glass", [2]float64{40, 40}, [2]float64{400, 40}).
		Wall("glass", [2]float64{400, 40}, [2]float64{400, 300}).
		Room("Lab", [2]float64{40, 40}, [2]float64{400, 40}, [2]float64{400, 300}, [2]float64{40, 300}).
		Trace()
	in := readRender(t, tr, synth.Options{Text: true, AntiAlias: true}, Options{})
	if len(in.Walls) != 0 || !hasNote(in, "no walls found") {
		t.Fatalf("walls %v notes %v", in.Walls, in.Notes)
	}
	if in.Coverage <= 0 {
		t.Errorf("coverage %v: the label is text and should count", in.Coverage)
	}
}

func TestReadIsDeterministic(t *testing.T) {
	src := raster.NewSource(synth.Render(fixture(t, "office"), everything))
	a, err := Read(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Read(src, Options{})
	if !reflect.DeepEqual(a, b) {
		t.Fatal("two reads of one image differ")
	}
}

// room is one 10 × 7.5 m room at 40 px/m with every kind of opening: a door
// in the south wall opening north, hinged at its west end; a window in the
// north wall; a pair of doors in the east wall opening west; and a plain gap
// in a partition.
func room(ppm [2]float64) *plan.Trace {
	tr := synth.New(40).
		Box(40, 40, 440, 340).
		Item(plan.Item{Type: "desk", X: 200, Y: 200, W: ptr(1.6), D: ptr(0.8)}).
		Trace()
	tr.Calibration.PxPerMetre = ppm
	tr.Walls[0].Openings = []plan.Opening{synth.Window(100, 180)}
	tr.Walls[1].Openings = []plan.Opening{synth.DoubleDoor(150, 222, "W")}
	tr.Walls[2].Openings = []plan.Opening{synth.Door(200, 236, "N", "a")}
	tr.Walls = append(tr.Walls, plan.Wall{
		ID: "p", Kind: "interior", From: [2]float64{240, 120}, To: [2]float64{240, 300},
		Openings: []plan.Opening{synth.Gap(190, 230)},
	})
	return tr
}

func ptr(v float64) *float64 { return &v }

func TestReadRoom(t *testing.T) {
	for _, c := range []struct {
		name string
		ppm  [2]float64
		o    Options
	}{
		{"square", [2]float64{40, 40}, Options{}},
		{"squashed, scale unknown", [2]float64{40, 46}, Options{}},
		{"squashed, scale given", [2]float64{40, 46}, Options{Scale: floorgraph.Scale{PxPerMetre: [2]float64{40, 46}}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			in := readRender(t, room(c.ppm), everything, c.o)
			if c.o.Scale.Known() != in.Scale.Known() {
				t.Errorf("scale %v", in.Scale)
			}
			byKind := map[string][]floorgraph.Opening{}
			for _, o := range in.Openings {
				byKind[o.Kind] = append(byKind[o.Kind], o)
			}
			if len(in.Openings) != 4 || len(byKind[floorgraph.Door]) != 1 ||
				len(byKind[floorgraph.DoubleDoor]) != 1 || len(byKind[floorgraph.Window]) != 1 ||
				len(byKind[floorgraph.Gap]) != 1 {
				t.Fatalf("openings %+v", in.Openings)
			}
			d := byKind[floorgraph.Door][0]
			if !near(d.A, 200, 340) || !near(d.B, 236, 340) || !d.HingeA || !d.Leafed ||
				d.Leaf.Y >= 340 {
				t.Errorf("door %+v", d)
			}
			dd := byKind[floorgraph.DoubleDoor][0]
			if !near(dd.A, 440, 150) || !near(dd.B, 440, 222) || !dd.Leafed || dd.Leaf.X >= 440 {
				t.Errorf("double door %+v", dd)
			}
			if w := byKind[floorgraph.Window][0]; !near(w.A, 100, 40) || !near(w.B, 180, 40) {
				t.Errorf("window %+v", w)
			}
			if g := byKind[floorgraph.Gap][0]; !near(g.A, 240, 190) || !near(g.B, 240, 230) ||
				g.Leafed {
				t.Errorf("gap %+v", g)
			}
			for _, s := range in.Walls {
				if s.Weight <= 0 || s.Weight > 1 || s.Thickness < 3 {
					t.Errorf("segment %+v", s)
				}
			}
		})
	}
}

func near(p floorgraph.Point, x, y float64) bool {
	return math.Abs(p.X-x) <= 2 && math.Abs(p.Y-y) <= 2
}

func TestSqDistMatchesBruteForce(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	w, h := 13, 9
	target := make([]bool, w*h)
	for i := range target {
		target[i] = rng.IntN(7) == 0
	}
	for _, border := range []bool{false, true} {
		got := sqDist(w, h, target, border)
		for y := range h {
			for x := range w {
				want := inf
				for yy := -1; yy <= h; yy++ {
					for xx := -1; xx <= w; xx++ {
						outside := xx < 0 || yy < 0 || xx >= w || yy >= h
						if (outside && border) || (!outside && target[yy*w+xx]) {
							want = min(want, (x-xx)*(x-xx)+(y-yy)*(y-yy))
						}
					}
				}
				if got[y*w+x] != want {
					t.Fatalf("border %v: (%d, %d) = %d, want %d", border, x, y, got[y*w+x], want)
				}
			}
		}
	}
	if d := sqDist(4, 3, make([]bool, 12), false); d[5] != inf {
		t.Errorf("no target: %d, want inf", d[5])
	}
	envelope(nil, nil, nil, nil) // an empty line is a no-op
}

func TestIsqrt(t *testing.T) {
	for _, n := range []int{0, 1, 2, 3, 4, 15, 16, 17, 1 << 40, 1<<52 + 12345} {
		r := isqrt(n)
		if r*r > n || (r+1)*(r+1) <= n {
			t.Errorf("isqrt(%d) = %d", n, r)
		}
	}
}

// Walls of eight widths in equal measure leave no width with the share the
// thinnest needs; the commonest stands in.
func TestWallBandSpread(t *testing.T) {
	w, h := 260, 220
	ink := make([]bool, w*h)
	y := 4
	for k := 2; k <= 9; k++ {
		for yy := y; yy < y+2*k-1; yy++ {
			for x := 10; x < 250; x++ {
				ink[yy*w+x] = true
			}
		}
		y += 2*k - 1 + 4
	}
	b, ok := wallBand(w, h, ink, sqDist(w, h, invert(ink), true))
	if !ok || b.lo < 2 || b.hi < 9 {
		t.Fatalf("band %+v ok %v", b, ok)
	}
}

func TestMergeJoinsCollinearPieces(t *testing.T) {
	ps := merge([]piece{
		{ax: horizontal, lo: 30, hi: 60, a0: 10, a1: 13, solid: 1},
		{ax: horizontal, lo: 0, hi: 28, a0: 10, a1: 13, solid: 0.5},
		{ax: vertical, lo: 0, hi: 28, a0: 10, a1: 13, solid: 1},
	})
	if len(ps) != 2 || ps[0].lo != 0 || ps[0].hi != 60 || math.Abs(ps[0].solid-0.75) > 0.02 {
		t.Fatalf("%+v", ps)
	}
	if overlap(ps[0], piece{ax: vertical, lo: 100, hi: 120, a0: 0, a1: 3}) != 0 {
		t.Error("disjoint pieces overlap")
	}
}
