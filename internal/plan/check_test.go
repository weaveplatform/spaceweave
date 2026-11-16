package plan

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func f(v float64) *float64 { return &v }

func validTrace() *Trace {
	return &Trace{
		Title:       "Box",
		Calibration: Calibration{PxPerMetre: [2]float64{10, 10}, Evidence: "door widths"},
		Thickness:   Thickness{Exterior: 0.3, Interior: 0.1, Glass: 0.05},
		Entrance:    [2]float64{0, 50},
		Walls: []Wall{
			{ID: "n", Kind: "exterior", From: [2]float64{0, 0}, To: [2]float64{100, 0}, Openings: []Opening{{A: 10, B: 20, Kind: "window"}}},
			{ID: "s", Kind: "exterior", From: [2]float64{0, 100}, To: [2]float64{100, 100}, Ext: f(0)},
			{ID: "w", Kind: "exterior", From: [2]float64{0, 0}, To: [2]float64{0, 100}, Openings: []Opening{{A: 40, B: 50, Kind: "door", Swing: "E", Hinge: "b"}}},
			{ID: "e", Kind: "exterior", From: [2]float64{100, 0}, To: [2]float64{100, 100}},
			{ID: "g", Kind: "glass", From: [2]float64{50, 0}, To: [2]float64{50, 100}, Openings: []Opening{{A: 10, B: 20, Kind: "opening"}}},
		},
		Rooms: []Room{
			{ID: "a", Name: "West", Zone: "work", Floor: "carpet", Polygon: [][2]float64{{0, 0}, {50, 0}, {50, 100}, {0, 100}}},
			{ID: "b", Name: "East", Zone: "meet", Floor: "wood", Polygon: [][2]float64{{50, 0}, {100, 0}, {100, 100}, {50, 100}}, Label: &[2]float64{75, 50}},
		},
		FloorPatches: []Patch{{ID: "t", Name: "Tiles", Floor: "tile", Polygon: [][2]float64{{0, 0}, {10, 0}, {10, 10}}}},
		Furniture: []Item{
			{Type: "desk", X: 20, Y: 20, W: f(1.6), D: f(0.8), Rot: f(90)},
			{Type: "chair", X: 20, Y: 30, Facing: f(270), Label: "Ann"},
			{Type: "roundTable", X: 70, Y: 70, R: f(0.6)},
			{Type: "sofa", X: 80, Y: 20, W: f(2), Facing: f(180)},
			{Type: "block", X: 60, Y: 60, W: f(1), D: f(1), H: f(1), Mat: "wood"},
			{Type: "chair", X: 30, Y: 30, Size: f(0.6)},
		},
	}
}

func TestCheckAcceptsAValidTrace(t *testing.T) {
	if problems := Check(validTrace()); len(problems) != 0 {
		t.Fatal(problems)
	}
}

func TestCheckFindsEachMistake(t *testing.T) {
	cases := []struct {
		name   string
		break_ func(*Trace)
		want   string
	}{
		{"title", func(t *Trace) { t.Title = " " }, "title is empty"},
		{"scale", func(t *Trace) { t.Calibration.PxPerMetre[1] = math.NaN() }, "pxPerMetre must be two positive"},
		{"evidence", func(t *Trace) { t.Calibration.Evidence = "" }, "evidence is empty"},
		{"thickness", func(t *Trace) { t.Thickness.Glass = 0 }, "thickness needs exterior"},
		{"outline", func(t *Trace) { t.Outline = [][2]float64{{0, 0}} }, "outline needs at least 3"},
		{"wall id", func(t *Trace) { t.Walls[0].ID = "" }, "wall #0 has no id"},
		{"repeat", func(t *Trace) { t.Walls[1].ID = "n" }, "wall id n is repeated"},
		{"kind", func(t *Trace) { t.Walls[4].Kind = "brick" }, `kind must be exterior, interior or glass, got "brick"`},
		{"finite", func(t *Trace) { t.Walls[0].To[0] = math.Inf(1) }, "must be finite"},
		{"ext", func(t *Trace) { t.Walls[0].Ext = f(-1) }, "ext is metres"},
		{"swing", func(t *Trace) { t.Walls[2].Openings[0].Swing = "up" }, "needs swing N, S, E or W"},
		{"opening kind", func(t *Trace) { t.Walls[0].Openings[0].Kind = "hatch" }, `opening kind must be door`},
		{"hinge", func(t *Trace) { t.Walls[2].Openings[0].Hinge = "c" }, "hinge must be a or b"},
		{"order", func(t *Trace) { t.Walls[0].Openings[0].A = 30 }, "opening needs a < b"},
		{"no exterior", func(t *Trace) {
			for i := range t.Walls {
				t.Walls[i].Kind = "interior"
			}
		}, "no exterior walls"},
		{"no walls", func(t *Trace) { t.Walls = nil }, "no walls"},
		{"room id", func(t *Trace) { t.Rooms[0].ID = "" }, "room #0 has no id"},
		{"room repeat", func(t *Trace) { t.Rooms[1].ID = "a" }, "room id a is repeated"},
		{"room name", func(t *Trace) { t.Rooms[0].Name = "" }, "room a has no name"},
		{"zone", func(t *Trace) { t.Rooms[0].Zone = "lounge" }, "zone must be one of"},
		{"floor", func(t *Trace) { t.Rooms[0].Floor = "lava" }, "floor must be one of"},
		{"polygon", func(t *Trace) { t.Rooms[0].Polygon = t.Rooms[0].Polygon[:2] }, "polygon needs at least 3"},
		{"patch", func(t *Trace) { t.FloorPatches[0].Floor = "grass" }, "floor patch t"},
		{"type", func(t *Trace) { t.Furniture[0].Type = "hovercraft" }, `furniture type "hovercraft" is not one of [armchair basin`},
		{"required", func(t *Trace) { t.Furniture[0].W = nil }, "desk at (20, 20): needs w"},
		{"mat", func(t *Trace) { t.Furniture[4].Mat = "gold" }, "mat must be one of"},
		{"pixels", func(t *Trace) { t.Furniture[0].D = f(80) }, "d must be a positive size in metres (pixels by mistake?)"},
		{"z", func(t *Trace) {
			t.Furniture = append(t.Furniture, Item{Type: "board", W: f(1), D: f(0.05), Z0: f(2), Z1: f(1)})
		}, "z1 must be above z0"},
		{"steps", func(t *Trace) {
			n := 1
			t.Furniture = append(t.Furniture, Item{Type: "stairs", W: f(1), D: f(3), Steps: &n})
		}, "steps must be 2–40"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tr := validTrace()
			c.break_(tr)
			problems := Check(tr)
			if !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, c.want) }) {
				t.Fatalf("want %q in %q", c.want, problems)
			}
		})
	}
}

func TestCheckCapsProblems(t *testing.T) {
	tr := validTrace()
	for range 100 {
		tr.Furniture = append(tr.Furniture, Item{Type: "hovercraft"})
	}
	if n := len(Check(tr)); n != MaxProblems {
		t.Fatalf("got %d problems, want the cap %d", n, MaxProblems)
	}
}

func TestShapes(t *testing.T) {
	tr := validTrace()
	tr.Outline = [][2]float64{{0, 0}, {100, 0}, {100, 100}, {0, 100}}
	tr.Walls = append(tr.Walls,
		Wall{ID: "odd", Kind: "brick", From: [2]float64{1, 1}, To: [2]float64{1, 1}, Openings: []Opening{{A: 0, B: 1}}},
		Wall{ID: "thick", Kind: "interior", From: [2]float64{0, 50}, To: [2]float64{50, 50}, Thickness: 1})
	shapes := Shapes(tr)
	// 2 rooms, 7 walls, 3 openings on non-degenerate walls, 6 items, outline.
	if len(shapes) != 2+7+3+6+1 {
		t.Fatalf("got %d shapes", len(shapes))
	}
	if shapes[0].Text != "West" || shapes[1].At != [2]float64{75, 50} {
		t.Fatalf("room labels: %+v %+v", shapes[0], shapes[1])
	}
	if w := shapes[len(shapes)-8]; w.Width != 10 {
		t.Fatalf("explicit thickness 1 m at 10 px/m is 10 px wide, got %v", w.Width)
	}
	// The desk is 1.6 × 0.8 m turned 90°: 8 px across, 16 px down at 10 px/m.
	desk := shapes[len(shapes)-7]
	minX, maxX, minY, maxY := desk.Points[0][0], desk.Points[0][0], desk.Points[0][1], desk.Points[0][1]
	for _, p := range desk.Points {
		minX, maxX = math.Min(minX, p[0]), math.Max(maxX, p[0])
		minY, maxY = math.Min(minY, p[1]), math.Max(maxY, p[1])
	}
	if math.Abs(maxX-minX-8) > 1e-9 || math.Abs(maxY-minY-16) > 1e-9 {
		t.Fatalf("desk footprint %v×%v", maxX-minX, maxY-minY)
	}
	// A missing scale still draws, at 1 px/m.
	tr.Calibration.PxPerMetre = [2]float64{}
	if len(Shapes(tr)) != len(shapes) {
		t.Fatal("shapes without a scale")
	}
	if c := centroid(nil); c != [2]float64{} {
		t.Fatal("centroid of nothing")
	}
}

func node(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	return path
}

func script(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "v.mjs")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// The real validator, on a real conversion: the site's own invariants must
// accept a valid trace and report a broken one.
func TestNodeValidatorAgainstTheSite(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	v := NodeValidator{Node: node(t), Script: filepath.Join(filepath.Dir(file), "..", "..", "web", "floorplan-iso", "tools", "validate.mjs")}

	p, err := Convert(validTrace(), Meta{URL: "generated/box/source.png", Credit: "test", SizePx: [2]int{100, 100}})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(p)
	if errs, err := v.Validate(context.Background(), b); err != nil || len(errs) != 0 {
		t.Fatalf("valid plan: %v %v", errs, err)
	}

	p.Rooms[0].Polygon = [][2]float64{{50, 50}, {60, 50}, {60, 60}}
	b, _ = json.Marshal(p)
	errs, err := v.Validate(context.Background(), b)
	if err != nil || len(errs) == 0 || !strings.Contains(errs[0], "room a has a corner outside the outline") {
		t.Fatalf("broken plan: %v %v", errs, err)
	}
}

func TestNodeValidatorFailures(t *testing.T) {
	n := node(t)
	cases := map[string]string{
		"exit":      `process.exit(3)`,
		"garbage":   `console.log("not json")`,
		"no reason": `console.log(JSON.stringify({ok: false, errors: []}))`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := NodeValidator{Node: n, Script: script(t, body)}.Validate(context.Background(), []byte("{}"))
			if !errors.Is(err, ErrValidator) {
				t.Fatalf("want ErrValidator, got %v", err)
			}
		})
	}
	if _, err := (NodeValidator{Node: filepath.Join(t.TempDir(), "no-node"), Script: "x"}).Validate(context.Background(), nil); !errors.Is(err, ErrValidator) {
		t.Fatalf("missing node: %v", err)
	}
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("TMP", filepath.Join(t.TempDir(), "missing"))
	if _, err := (NodeValidator{Node: n, Script: "x"}).Validate(context.Background(), nil); !errors.Is(err, ErrValidator) {
		t.Fatalf("no temp dir: %v", err)
	}
}
