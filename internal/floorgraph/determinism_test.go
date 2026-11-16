package floorgraph

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"runtime"
	"slices"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/plan"
)

func marshal(t *testing.T, in Input) []byte {
	t.Helper()
	r, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// The same evidence in any order, with walls drawn either way round, on any
// number of threads, gives the same bytes.
func TestDeterministic(t *testing.T) {
	inputs := synthetic()
	for _, name := range []string{"office", "corporate"} {
		inputs[name] = evidence(loadTrace(t, name))
	}
	rng := rand.New(rand.NewPCG(1, 2))
	for name, in := range inputs {
		t.Run(name, func(t *testing.T) {
			want := marshal(t, in)
			if got := marshal(t, in); !bytes.Equal(got, want) {
				t.Fatal("two builds differ")
			}
			for range 3 {
				sh := in
				sh.Walls, sh.Openings, sh.Labels = slices.Clone(
					in.Walls,
				), slices.Clone(
					in.Openings,
				), slices.Clone(
					in.Labels,
				)
				rng.Shuffle(
					len(sh.Walls),
					func(i, j int) { sh.Walls[i], sh.Walls[j] = sh.Walls[j], sh.Walls[i] },
				)
				rng.Shuffle(
					len(sh.Openings),
					func(i, j int) { sh.Openings[i], sh.Openings[j] = sh.Openings[j], sh.Openings[i] },
				)
				rng.Shuffle(
					len(sh.Labels),
					func(i, j int) { sh.Labels[i], sh.Labels[j] = sh.Labels[j], sh.Labels[i] },
				)
				for i := range sh.Walls {
					if rng.IntN(2) == 0 {
						sh.Walls[i].A, sh.Walls[i].B = sh.Walls[i].B, sh.Walls[i].A
					}
				}
				if got := marshal(t, sh); !bytes.Equal(got, want) {
					t.Fatalf("shuffled input gives a different trace:\n%s\n%s", got, want)
				}
			}
			prev := runtime.GOMAXPROCS(1)
			got := marshal(t, in)
			runtime.GOMAXPROCS(prev)
			if !bytes.Equal(got, want) {
				t.Fatal("GOMAXPROCS changes the trace")
			}
		})
	}
}

// fuzzInput decodes bytes into evidence on a coarse grid, so random input
// often makes walls that meet.
func fuzzInput(data []byte) Input {
	next := func() int {
		if len(data) == 0 {
			return 0
		}
		b := data[0]
		data = data[1:]
		return int(b)
	}
	in := Input{Coverage: -1}
	if s := next(); s > 0 {
		in.Scale = Scale{
			PxPerMetre: [2]float64{float64(10 + s), float64(10 + s + next()%3)},
			Exact:      s%2 == 0,
		}
	}
	// A 4 px grid, nudged off it by a value's residue so some walls just
	// miss each other.
	coord := func() float64 { v := next(); return float64(v*4) + float64(v%3)*0.37 }
	pt := func() Point { return P(coord(), coord()) }
	for n := next() % 24; n > 0; n-- {
		a, b := pt(), pt()
		k := next()
		in.Walls = append(
			in.Walls,
			Segment{
				A:         a,
				B:         b,
				Thickness: float64(k % 16),
				Glass:     k&16 != 0,
				Weight:    float64(k%5) / 4,
			},
		)
	}
	kinds := []string{Door, DoubleDoor, Window, Gap, "arch"}
	for n := next() % 8; n > 0; n-- {
		a, b := pt(), pt()
		k := next()
		in.Openings = append(
			in.Openings,
			Opening{A: a, B: b, Kind: kinds[k%5], HingeA: k&8 != 0, Leafed: k&16 != 0, Leaf: pt()},
		)
	}
	texts := []string{"Office", "WC", "3.60", "Kitchen", ""}
	for n := next() % 4; n > 0; n-- {
		in.Labels = append(
			in.Labels,
			Label{At: pt(), Text: texts[next()%5], Height: float64(next() % 20)},
		)
	}
	return in
}

// Random evidence never panics, and whatever builds passes plan.Check,
// converts, and passes the site validator's geometry.
func FuzzBuild(f *testing.F) {
	f.Add([]byte{})
	f.Add(
		[]byte{
			40,
			0,
			6,
			0,
			0,
			100,
			0,
			6,
			100,
			0,
			100,
			80,
			6,
			100,
			80,
			0,
			80,
			6,
			0,
			80,
			0,
			0,
			6,
			50,
			0,
			50,
			80,
			6,
			1,
			20,
			0,
			30,
			0,
			16,
		},
	)
	f.Add(
		[]byte{
			0,
			3,
			10,
			10,
			60,
			12,
			5,
			60,
			12,
			55,
			70,
			21,
			55,
			70,
			10,
			10,
			3,
			2,
			30,
			11,
			40,
			13,
			0,
		},
	)
	f.Add([]byte{7, 1, 9, 0, 0, 50, 50, 255, 1, 0, 0, 1, 1, 3})
	f.Fuzz(func(t *testing.T, data []byte) {
		in := fuzzInput(data)
		r, err := Build(in)
		if err != nil {
			if !errors.Is(err, ErrNoWalls) && !errors.Is(err, ErrFlat) {
				t.Fatalf("unexpected error %v for %+v", err, in)
			}
			return
		}
		if p := plan.Check(r.Trace); len(p) > 0 {
			t.Fatalf("plan.Check: %v", p)
		}
		p, err := plan.Convert(r.Trace, plan.Meta{URL: "x", Credit: "x", SizePx: [2]int{1, 1}})
		if err != nil {
			t.Fatal(err)
		}
		if errs := siteCheck(p); len(errs) > 0 {
			t.Fatalf("site validator: %v for %+v", errs, in)
		}
	})
}

// With the half-edge rule in faces, a bounded face has positive shoelace
// area and the outer face negative, in image coordinates as in any other.
func TestFaceSigns(t *testing.T) {
	g := node([]seg{
		newSeg(P(0, 0), P(10, 0), P(1, 0), 0, 1, 1, false),
		newSeg(P(10, 0), P(10, 10), P(0, 1), 1, 1, 1, false),
		newSeg(P(0, 10), P(10, 10), P(1, 0), 0, 1, 1, false),
		newSeg(P(0, 0), P(0, 10), P(0, 1), 1, 1, 1, false),
	})
	fs := g.faces(g.prune())
	if len(fs) != 2 {
		t.Fatalf("%d faces", len(fs))
	}
	areas := []float64{fs[0].area, fs[1].area}
	slices.Sort(areas)
	if areas[0] != -100 || areas[1] != 100 {
		t.Fatalf("areas %v", areas)
	}
}
