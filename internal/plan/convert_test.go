package plan

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func load(t *testing.T, name string, v any) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}

// roundTrip converts a plan through JSON into generic values, the way the
// site reads it, so the comparison below sees what the renderer would.
func roundTrip(t *testing.T, v any) map[string]any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// near compares JSON values, numbers within tol. Keys present only in got
// must be defaults Convert adds (see Types).
func near(t *testing.T, path string, got, want any, tol float64) {
	t.Helper()
	switch w := want.(type) {
	case float64:
		g, ok := got.(float64)
		if !ok || math.Abs(g-w) > tol {
			t.Errorf("%s: got %v, want %v", path, got, w)
		}
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			t.Errorf("%s: got %v, want %v", path, got, w)
			return
		}
		for i := range w {
			near(t, path+"["+itoa(i)+"]", g[i], w[i], tol)
		}
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			t.Errorf("%s: got %v, want an object", path, got)
			return
		}
		for k, wv := range w {
			near(t, path+"."+k, g[k], wv, tol)
		}
	default:
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v, want %v", path, got, want)
		}
	}
}

func itoa(i int) string { b, _ := json.Marshal(i); return string(b) }

// The hand-traced plans, written out as pixel traces by testdata/gen.mjs, must
// convert back to their own data within a centimetre and a tenth of a degree.
// Corporate is the squashed image: it exercises the per-axis scale and the
// conversion of angles.
func TestConvertGolden(t *testing.T) {
	for _, name := range []string{"office", "corporate"} {
		t.Run(name, func(t *testing.T) {
			var tr Trace
			load(t, name+".trace.json", &tr)
			if problems := Check(&tr); len(problems) > 0 {
				t.Fatalf("golden trace fails Check: %v", problems)
			}
			p, err := Convert(&tr, Meta{URL: "generated/x/source.png", Credit: "test", SizePx: [2]int{100, 100}})
			if err != nil {
				t.Fatal(err)
			}
			var want map[string]any
			load(t, name+".expected.json", &want)
			got := roundTrip(t, p)
			for _, key := range []string{"WIDTH", "DEPTH", "OUTLINE", "ENTRANCE", "walls", "rooms", "floorPatches"} {
				if want[key] != nil {
					near(t, key, got[key], want[key], 0.01)
				}
			}
			// Furniture angles go through the scale twice; compare them on their own.
			gotItems, wantItems := got["furniture"].([]any), want["furniture"].([]any)
			if len(gotItems) != len(wantItems) {
				t.Fatalf("furniture: %d items, want %d", len(gotItems), len(wantItems))
			}
			for i := range wantItems {
				w, g := wantItems[i].(map[string]any), gotItems[i].(map[string]any)
				for k, wv := range w {
					tol := 0.01
					if k == "rot" || k == "facing" {
						tol = 0.11
						wv = math.Mod(wv.(float64)+540, 360) - 180
						g[k] = math.Mod(g[k].(float64)+540, 360) - 180
					}
					near(t, "furniture["+itoa(i)+"]."+k, g[k], wv, tol)
				}
			}
		})
	}
}

func TestConvertSource(t *testing.T) {
	tr := &Trace{
		Title:       "Squashed",
		Calibration: Calibration{PxPerMetre: [2]float64{40, 50}},
		Walls:       []Wall{{ID: "a", Kind: "exterior", From: [2]float64{100, 200}, To: [2]float64{500, 200}}},
		Entrance:    [2]float64{100, 200},
	}
	p, err := Convert(tr, Meta{URL: "u", Credit: "c", SizePx: [2]int{640, 480}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Source.PxPerMetre, []float64{40, 50}) || p.Source.OriginPx != [2]float64{100, 200} || p.Width != 10 || p.Depth != 0 {
		t.Fatalf("source %+v width %v depth %v", p.Source, p.Width, p.Depth)
	}
	if p.Source.Title != "Squashed" || p.Source.SizePx != [2]int{640, 480} || p.Source.URL != "u" || p.Source.Credit != "c" {
		t.Fatalf("meta not carried: %+v", p.Source)
	}
	if p.Outline != nil || len(p.FloorPatches) != 0 || p.FloorPatches == nil {
		t.Fatal("no outline, and floorPatches must be an empty array, not null")
	}

	tr.Calibration.PxPerMetre = [2]float64{40, 40}
	p, _ = Convert(tr, Meta{})
	if p.Source.PxPerMetre != 40.0 {
		t.Fatalf("equal scales are written as one number, got %v", p.Source.PxPerMetre)
	}
}

func TestConvertErrors(t *testing.T) {
	for _, c := range [][2]float64{{0, 40}, {40, -1}, {math.Inf(1), 40}, {math.NaN(), 40}} {
		tr := &Trace{Calibration: Calibration{PxPerMetre: c}, Walls: []Wall{{}}}
		if _, err := Convert(tr, Meta{}); !errors.Is(err, ErrNoCalibration) {
			t.Errorf("%v: want ErrNoCalibration, got %v", c, err)
		}
	}
	if _, err := Convert(&Trace{Calibration: Calibration{PxPerMetre: [2]float64{1, 1}}}, Meta{}); !errors.Is(err, ErrNoWalls) {
		t.Fatalf("want ErrNoWalls, got %v", err)
	}
}

// On a squashed image a wall that looks horizontal in pixels can be mostly
// vertical in metres; its openings must land on the metre axis, in order, with
// the hinge still on the same physical end.
func TestConvertOpeningSwitchesAxis(t *testing.T) {
	tr := &Trace{
		Calibration: Calibration{PxPerMetre: [2]float64{100, 10}},
		Walls: []Wall{{
			ID: "w", Kind: "interior", From: [2]float64{0, 100}, To: [2]float64{100, 0}, // 100 px across, 100 px up
			Openings: []Opening{{A: 20, B: 40, Kind: "door", Swing: "N", Hinge: "a"}, {A: 60, B: 80, Kind: "window"}},
		}},
	}
	p, err := Convert(tr, Meta{})
	if err != nil {
		t.Fatal(err)
	}
	w := p.Walls[0]
	// In metres the wall runs (0, 10) → (1, 0): 1 m across, 10 m up, so y dominates.
	if w.From != [2]float64{0, 10} || w.To != [2]float64{1, 0} {
		t.Fatalf("wall %v → %v", w.From, w.To)
	}
	door, window := w.Openings[0], w.Openings[1]
	if door.A != 6 || door.B != 8 || door.Hinge != "b" {
		t.Fatalf("door %+v: want 6–8 on y with the hinge moved to b", door)
	}
	if window.A != 2 || window.B != 4 || window.Sill != defaultSill || window.Head != defaultHead || window.Hinge != "b" {
		t.Fatalf("window %+v", window)
	}
}

func TestConvertZeroSpanOpening(t *testing.T) {
	tr := &Trace{
		Calibration: Calibration{PxPerMetre: [2]float64{10, 10}},
		Walls:       []Wall{{ID: "w", Kind: "interior", From: [2]float64{5, 5}, To: [2]float64{5, 5}, Openings: []Opening{{A: 1, B: 2, Kind: "opening"}}}},
	}
	p, err := Convert(tr, Meta{})
	if err != nil {
		t.Fatal(err)
	}
	if o := p.Walls[0].Openings[0]; o.A != 0 || o.B != 0 {
		t.Fatalf("a zero-length wall puts its openings at its start, got %+v", o)
	}
}

func TestConvertItemDefaultsAndAngles(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	steps := 12
	s := scale{sx: 1, sy: 2}
	got := convertItem(s, Item{Type: "stairs", X: 4, Y: 8, W: f(1), D: f(3), Facing: f(45), Steps: &steps, Label: "Up", Mat: "wood"})
	want := map[string]any{"type": "stairs", "x": 4.0, "y": 4.0, "w": 1.0, "d": 3.0, "rise": 1.5, "steps": 12, "facing": 26.6, "label": "Up", "mat": "wood"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
	desk := convertItem(s, Item{Type: "desk", W: f(1), D: f(1)})
	if desk["rot"] != 0.0 || desk["h"] != 0.75 {
		t.Fatalf("desk defaults: %v", desk)
	}
	plant := convertItem(s, Item{Type: "plant", R: f(0.5), H: f(1), Size: f(1), Z0: f(0), Z1: f(1), Rise: f(1), Rot: f(90)})
	if plant["r"] != 0.5 || plant["rot"] != 90.0 {
		t.Fatalf("plant: %v", plant)
	}
}

func TestDescribe(t *testing.T) {
	p := &Plan{Width: 12.345, Depth: 6, Walls: []MWall{{Openings: []Opening{{}, {}}}}, Rooms: []MRoom{{}}}
	if got := Describe(p); !strings.HasPrefix(got, "12.35 × 6.00 m; 1 walls, 2 openings, 1 rooms, 0 furniture") {
		t.Fatal(got)
	}
}
