package score

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/plan"
)

func trace(walls ...plan.Wall) *plan.Trace { return &plan.Trace{Walls: walls} }

func wall(x0, y0, x1, y1 float64, ops ...plan.Opening) plan.Wall {
	return plan.Wall{From: [2]float64{x0, y0}, To: [2]float64{x1, y1}, Openings: ops}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestWallsIdentical(t *testing.T) {
	tr := trace(wall(0, 0, 100, 0), wall(100, 0, 100, 50))
	r, p := Walls(tr, tr, 1)
	if !near(r, 1) || !near(p, 1) {
		t.Fatalf("recall %v precision %v, want 1, 1", r, p)
	}
}

func TestWallsHalf(t *testing.T) {
	want := trace(wall(0, 0, 100, 0))
	got := trace(wall(0, 1, 50, 1), wall(0, 40, 50, 40))
	r, p := Walls(got, want, 1.2)
	if !near(r, 0.5) || !near(p, 0.5) {
		t.Fatalf("recall %v precision %v, want 0.5, 0.5", r, p)
	}
	// Outside the tolerance nothing counts.
	r, p = Walls(got, want, 0.5)
	if r != 0 || p != 0.0 {
		t.Fatalf("recall %v precision %v, want 0, 0", r, p)
	}
}

func TestWallsEmpty(t *testing.T) {
	r, p := Walls(trace(), trace(wall(0, 0, 10, 0)), 3)
	if r != 0 || p != 1 {
		t.Fatalf("recall %v precision %v, want 0, 1", r, p)
	}
	// A zero-length wall is one sample at its point.
	if c := Covered([]Segment{{{5, 5}, {5, 5}}}, []Segment{{{5, 5}, {5, 5}}}, 0); c != 1 {
		t.Fatalf("covered %v", c)
	}
}

func TestDistClampsToEnds(t *testing.T) {
	s := Segment{{0, 0}, {10, 0}}
	for _, c := range []struct {
		p    [2]float64
		want float64
	}{{[2]float64{-3, 4}, 5}, {[2]float64{13, 4}, 5}, {[2]float64{5, 2}, 2}} {
		if d := dist(c.p, s); !near(d, c.want) {
			t.Errorf("dist(%v) = %v, want %v", c.p, d, c.want)
		}
	}
}

func TestRooms(t *testing.T) {
	want := &plan.Trace{
		Rooms: []plan.Room{{Name: "Office"}, {Name: "Office"}, {Name: "Women's WC"}, {Name: "Lab"}},
	}
	got := &plan.Trace{
		Rooms: []plan.Room{{Name: "office"}, {Name: "WOMEN S  wc"}, {Name: "Kitchen"}},
	}
	m := Rooms(got, want)
	if m.Matched != 2 || m.Want != 4 || m.Got != 3 {
		t.Fatalf("got %+v", m)
	}
	if len(m.Missing) != 2 || m.Missing[0] != "Office" || m.Missing[1] != "Lab" {
		t.Errorf("missing %v", m.Missing)
	}
	if len(m.Extra) != 1 || m.Extra[0] != "Kitchen" {
		t.Errorf("extra %v", m.Extra)
	}
	if m := Rooms(want, want); m.Matched != 4 || m.Missing != nil || m.Extra != nil {
		t.Errorf("self match %+v", m)
	}
}

func TestOpenings(t *testing.T) {
	want := trace(
		wall(0, 0, 200, 0,
			plan.Opening{A: 20, B: 50, Kind: "door"},
			plan.Opening{A: 100, B: 160, Kind: "window"}),
		wall(0, 0, 0, 100, plan.Opening{A: 40, B: 70, Kind: "door"}),
	)
	got := trace(
		wall(0, 1, 200, 1,
			plan.Opening{A: 22, B: 52, Kind: "door"},
			plan.Opening{A: 100, B: 160, Kind: "opening"}),
		// A door at the right place but on a crossing wall is not a match.
		wall(-50, 55, 50, 55, plan.Opening{A: -15, B: 15, Kind: "door"}),
		wall(3, 3, 3, 3, plan.Opening{A: 1, B: 2, Kind: "door"}), // degenerate: ignored
	)
	m := Openings(got, want, 4)
	// Both doors default to hanging from a, so the matched one is hinged
	// right; neither has a swing to agree on.
	if d := m["door"]; d != (OpeningMatch{Matched: 1, Want: 2, Got: 2, Located: 1, Hinged: 1}) {
		t.Errorf("door %+v", d)
	}
	// The window was found, as an opening: located, not matched.
	if w := m["window"]; w != (OpeningMatch{Want: 1, Located: 1}) {
		t.Errorf("window %+v", w)
	}
	if g := m["opening"]; g != (OpeningMatch{Got: 1}) {
		t.Errorf("opening %+v", g)
	}
}

func TestOnWallAndSkew(t *testing.T) {
	w := wall(0, 0, 10, 100)
	if p := OnWall(w, 50); !near(p[0], 5) || !near(p[1], 50) {
		t.Errorf("vertical-ish: %v", p)
	}
	w = wall(0, 0, 100, 10)
	if p := OnWall(w, 50); !near(p[0], 50) || !near(p[1], 5) {
		t.Errorf("horizontal-ish: %v", p)
	}
	if s := skew(0, math.Pi); !near(s, 0) {
		t.Errorf("opposite directions are one line: %v", s)
	}
	if s := skew(0.1, -0.1+math.Pi); !near(s, 0.2) {
		t.Errorf("skew %v", s)
	}
}

func TestFixtureAgainstItself(t *testing.T) {
	b, err := os.ReadFile("../../plan/testdata/office.trace.json")
	if err != nil {
		t.Fatal(err)
	}
	var tr plan.Trace
	if err := json.Unmarshal(b, &tr); err != nil {
		t.Fatal(err)
	}
	r, p := Walls(&tr, &tr, 0.5)
	if !near(r, 1) || !near(p, 1) {
		t.Errorf("walls %v %v", r, p)
	}
	if m := Rooms(&tr, &tr); m.Matched != len(tr.Rooms) {
		t.Errorf("rooms %+v", m)
	}
	for k, m := range Openings(&tr, &tr, 1) {
		if m.Matched != m.Want || m.Want != m.Got {
			t.Errorf("%s %+v", k, m)
		}
	}
}

// A matched door counts as swung only when it opens to the same side, and as
// hinged only when it hangs from the same end; double doors have no hinge.
func TestOpeningSwingAndHinge(t *testing.T) {
	want := trace(wall(0, 0, 300, 0,
		plan.Opening{A: 20, B: 50, Kind: "door", Swing: "S", Hinge: "b"},
		plan.Opening{A: 100, B: 130, Kind: "door", Swing: "N"},
		plan.Opening{A: 200, B: 260, Kind: "double-door", Swing: "S"},
	))
	got := trace(wall(0, 0, 300, 0,
		plan.Opening{A: 20, B: 50, Kind: "door", Swing: "S", Hinge: "b"},
		plan.Opening{A: 100, B: 130, Kind: "door", Swing: "S", Hinge: "b"},
		plan.Opening{A: 200, B: 260, Kind: "double-door", Swing: "S"},
	))
	m := Openings(got, want, 4)
	if d := m["door"]; d != (OpeningMatch{Matched: 2, Want: 2, Got: 2, Located: 2, Swung: 1, Hinged: 1}) {
		t.Errorf("door %+v", d)
	}
	if d := m["double-door"]; d != (OpeningMatch{Matched: 1, Want: 1, Got: 1, Located: 1, Swung: 1}) {
		t.Errorf("double door %+v", d)
	}
}

func box(name string, x0, y0, x1, y1 float64) plan.Room {
	return plan.Room{Name: name, Polygon: [][2]float64{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}}
}

func TestRoomAreas(t *testing.T) {
	want := &plan.Trace{Rooms: []plan.Room{
		box("Office", 0, 0, 100, 100),   // traced exactly, named
		box("Store", 100, 0, 200, 100),  // traced a little small, unnamed
		box("Open A", 0, 100, 100, 200), // merged with Open B by the tracer
		box("Open B", 100, 100, 200, 200),
		box("Hall", 200, 0, 300, 200), // split in two by the tracer
		box("Lost", 400, 0, 500, 100), // missed
	}}
	got := &plan.Trace{Rooms: []plan.Room{
		box("office", 0, 0, 100, 100),
		box("", 102, 2, 198, 98),
		box("", 0, 100, 200, 200),
		box("", 200, 0, 300, 100),
		box("", 200, 100, 300, 200),
		box("", 600, 0, 700, 100), // nothing there
	}}
	m := RoomAreas(got, want)
	if m.Want != 6 || m.Got != 6 || m.Exact != 2 || m.Covered != 5 || m.Named != 1 {
		t.Errorf("match %+v", m)
	}
	// Want covers 70 000 px² (the hall is twice the others); got covers
	// all of it but Lost and a 784 px² sliver of the Store: 59 216.
	// Got covers 69 216, the room at x = 600 outside every want room.
	if !near(m.AreaRecall, 59216/70000.0) || !near(m.AreaPrecision, 59216/69216.0) {
		t.Errorf("area recall %v precision %v", m.AreaRecall, m.AreaPrecision)
	}
	if m := RoomAreas(&plan.Trace{}, &plan.Trace{}); m != (AreaMatch{AreaRecall: 1, AreaPrecision: 1}) {
		t.Errorf("empty %+v", m)
	}
	// A degenerate room has no area and matches nothing.
	flat := &plan.Trace{Rooms: []plan.Room{box("", 0, 0, 100, 0)}}
	if m := RoomAreas(flat, flat); m.Exact != 0 || m.Covered != 0 {
		t.Errorf("flat %+v", m)
	}
}
