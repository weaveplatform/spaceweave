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
	if d := m["door"]; d != (OpeningMatch{Matched: 1, Want: 2, Got: 2}) {
		t.Errorf("door %+v", d)
	}
	if w := m["window"]; w != (OpeningMatch{Want: 1}) {
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
