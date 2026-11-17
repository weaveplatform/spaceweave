package floorgraph

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/plan"
)

func polyOf(ps [][2]float64) []Point {
	out := make([]Point, len(ps))
	for i, p := range ps {
		out[i] = P(p[0], p[1])
	}
	return out
}

// atrium is a 20 × 16 m floor with a 6 × 5 m void drawn as loose edges, not
// walls: a slab edge and, 0.2 m in, a balustrade, each with a gap of a few
// centimetres where it stops short of the next line, and a lift shaft
// standing in the void.
func atrium() Input {
	return Input{
		Walls: append(box(0, 0, 1000, 800), box(420, 300, 480, 350)...),
		Edges: []Segment{
			{A: P(300, 250), B: P(600, 250)}, {A: P(600, 250), B: P(600, 500)},
			{A: P(600, 500), B: P(300, 500)}, {A: P(300, 500), B: P(300, 253)},
			{A: P(310, 260), B: P(588, 260)}, {A: P(590, 262), B: P(590, 490)},
			{A: P(590, 490), B: P(310, 490)}, {A: P(310, 490), B: P(310, 260)},
		},
		Voids:    []Point{P(450, 420), P(350, 300)},
		Labels:   []Label{{At: P(450, 420), Text: "ATRIUM", Height: 10, Kind: LabelName}},
		Openings: []Opening{{A: P(0, 300), B: P(0, 345), Kind: Door, Weight: 1}},
		Scale:    scale50,
		Coverage: -1,
	}
}

func TestVoidLeftOpen(t *testing.T) {
	r := mustBuild(t, atrium())
	if len(r.Trace.Voids) != 1 {
		t.Fatalf("voids %v", r.Trace.Voids)
	}
	if slices.Contains(roomNames(r), "ATRIUM") {
		t.Errorf("the void's label named a room: %v", roomNames(r))
	}
	// The void is a hole: no room has its middle, but the shaft in it is a
	// room still.
	for _, rm := range r.Trace.Rooms {
		if inside(P(450, 420), polyOf(rm.Polygon)) {
			t.Errorf("room %s %q floors the void", rm.ID, rm.Name)
		}
	}
	if !slices.ContainsFunc(r.Trace.Rooms, func(rm plan.Room) bool { return inside(P(450, 325), polyOf(rm.Polygon)) }) {
		t.Errorf("the shaft in the void is no room: %v", roomNames(r))
	}
	v := polyOf(r.Trace.Voids[0])
	// The hole is inside the innermost line, the balustrade: 5.6 × 4.6 m
	// less the 1.2 × 1 m shaft.
	if a := math.Abs(signedArea(v)) / (50 * 50); a < 23 || a > 25 {
		t.Errorf("void %.1f m², want about 24.6", a)
	}
	if !slices.Contains(r.Caveats, "1 void (an atrium, a light well) was left open, not floored.") {
		t.Errorf("caveats %v", r.Caveats)
	}
}

func TestVoidMissed(t *testing.T) {
	for name, in := range map[string]Input{
		// A gap a stair's width wide: the floor round it leaks in, and
		// that is no void.
		"leak": func() Input {
			in := atrium()
			in.Edges[0].A, in.Edges[4].A = P(400, 250), P(400, 260)
			return in
		}(),
		"no scale": func() Input {
			in := atrium()
			in.Scale = Scale{}
			return in
		}(),
		"on a line": func() Input {
			in := atrium()
			in.Voids = []Point{P(300, 400)}
			return in
		}(),
		"boxed in": func() Input {
			in := atrium()
			in.Voids = []Point{P(800, 600)}
			in.Edges = append(in.Edges, box(785, 585, 815, 615)...)
			return in
		}(),
		"outside": func() Input {
			in := atrium()
			in.Voids = []Point{P(1200, 400)}
			return in
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			r, err := Build(in)
			if err != nil {
				t.Fatal(err)
			}
			checkResult(t, r)
			if len(r.Trace.Voids) != 0 || !slices.ContainsFunc(r.Caveats, func(c string) bool {
				return strings.Contains(c, "in no closed area, so the floor round")
			}) {
				t.Errorf("voids %v caveats %v", r.Trace.Voids, r.Caveats)
			}
		})
	}
}

// A void drawn with walls round it (a light well's walls) needs no edges.
func TestVoidInWalls(t *testing.T) {
	in := Input{
		Walls:    append(box(0, 0, 1000, 800), box(300, 250, 600, 500)...),
		Voids:    []Point{P(450, 375), P(450, 375), P(math.NaN(), 0)},
		Scale:    scale50,
		Coverage: -1,
	}
	r := mustBuild(t, in)
	if len(r.Trace.Voids) != 1 || len(r.Trace.Rooms) != 1 {
		t.Errorf("voids %v rooms %v", r.Trace.Voids, roomNames(r))
	}
}

func TestRdpRingDegenerate(t *testing.T) {
	if rdpRing([]Point{P(0, 0), P(1, 0), P(2, 0)}, 1) != nil {
		t.Error("three points are no ring")
	}
	if rdpRing([]Point{P(0, 0), P(1, 0), P(2, 0), P(3, 0)}, 0.1) != nil {
		t.Error("a flat ring has no area")
	}
}
