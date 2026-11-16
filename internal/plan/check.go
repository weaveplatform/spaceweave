package plan

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// MaxProblems bounds one check's report, like test/invariants.mjs, so a
// systematic mistake doesn't flood the tracer's context.
const MaxProblems = 60

// Check finds structural mistakes in a trace before conversion: unknown
// kinds and types, missing fields, repeated ids, impossible numbers. Geometric
// checks (inside the building, rooms overlapping) need metres and are left to
// the site's validator.
func Check(t *Trace) []string {
	var out []string
	add := func(format string, args ...any) {
		if len(out) < MaxProblems {
			out = append(out, fmt.Sprintf(format, args...))
		}
	}
	checkHeader(t, add)
	checkWalls(t.Walls, add)
	checkRooms(t, add)
	for _, it := range t.Furniture {
		checkItem(it, add)
	}
	return out
}

type report = func(format string, args ...any)

func finite(vs ...float64) bool {
	for _, v := range vs {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

// name is how a problem refers to a wall or room: its id, or its index when
// the id is missing (which is itself reported).
func name(kind, id string, i int, add report) string {
	if id == "" {
		id = fmt.Sprintf("#%d", i)
		add("%s %s has no id", kind, id)
	}
	return id
}

func checkHeader(t *Trace, add report) {
	if strings.TrimSpace(t.Title) == "" {
		add("title is empty")
	}
	if c := t.Calibration.PxPerMetre; !(c[0] > 0 && c[1] > 0 && finite(c[0], c[1])) {
		add("calibration.pxPerMetre must be two positive numbers, got %v", c)
	}
	if strings.TrimSpace(t.Calibration.Evidence) == "" {
		add("calibration.evidence is empty: say what the scale was measured from")
	}
	if th := t.Thickness; !(th.Exterior > 0 && th.Interior > 0 && th.Glass > 0) {
		add("thickness needs exterior, interior and glass in metres, all positive")
	}
	if len(t.Outline) > 0 && len(t.Outline) < 3 {
		add("outline needs at least 3 points")
	}
}

func checkWalls(walls []Wall, add report) {
	if len(walls) == 0 {
		add("no walls")
		return
	}
	ids := map[string]bool{}
	exterior := false
	for i, w := range walls {
		id := name("wall", w.ID, i, add)
		if ids[w.ID] && w.ID != "" {
			add("wall id %s is repeated", w.ID)
		}
		ids[w.ID] = true
		switch w.Kind {
		case "exterior":
			exterior = true
		case "interior", "glass":
		default:
			add("wall %s: kind must be exterior, interior or glass, got %q", id, w.Kind)
		}
		if !finite(w.From[0], w.From[1], w.To[0], w.To[1], w.Thickness) || w.Thickness < 0 {
			add("wall %s: coordinates and thickness must be finite, thickness ≥ 0", id)
		}
		if w.Ext != nil && !(*w.Ext >= 0 && *w.Ext < 2) {
			add("wall %s: ext is metres past each end, 0 to 2", id)
		}
		for _, o := range w.Openings {
			checkOpening(id, o, add)
		}
	}
	if !exterior {
		add("no exterior walls")
	}
}

func checkRooms(t *Trace, add report) {
	ids := map[string]bool{}
	for i, r := range t.Rooms {
		id := name("room", r.ID, i, add)
		if ids[r.ID] && r.ID != "" {
			add("room id %s is repeated", r.ID)
		}
		ids[r.ID] = true
		if strings.TrimSpace(r.Name) == "" {
			add("room %s has no name", id)
		}
		if !slices.Contains(Zones, r.Zone) {
			add("room %s: zone must be one of %v, got %q", id, Zones, r.Zone)
		}
		if !slices.Contains(Floors, r.Floor) {
			add("room %s: floor must be one of %v, got %q", id, Floors, r.Floor)
		}
		if len(r.Polygon) < 3 {
			add("room %s: polygon needs at least 3 points", id)
		}
	}
	for _, f := range t.FloorPatches {
		if !slices.Contains(Floors, f.Floor) || len(f.Polygon) < 3 {
			add("floor patch %s: needs a floor (%v) and at least 3 points", f.ID, Floors)
		}
	}
}

func checkOpening(wall string, o Opening, add report) {
	switch o.Kind {
	case "door", "double-door":
		if !slices.Contains([]string{"N", "S", "E", "W"}, o.Swing) {
			add(
				"wall %s: %s at %g–%g needs swing N, S, E or W (the side the leaf opens into)",
				wall,
				o.Kind,
				o.A,
				o.B,
			)
		}
	case "window", "opening":
	default:
		add(
			"wall %s: opening kind must be door, double-door, window or opening, got %q",
			wall,
			o.Kind,
		)
	}
	if o.Hinge != "" && o.Hinge != "a" && o.Hinge != "b" {
		add("wall %s: hinge must be a or b, got %q", wall, o.Hinge)
	}
	if !(o.A < o.B) {
		add("wall %s: opening needs a < b, got %g–%g", wall, o.A, o.B)
	}
}

func checkItem(it Item, add report) {
	where := fmt.Sprintf("%s at (%g, %g)", it.Type, it.X, it.Y)
	spec, ok := Types[it.Type]
	if !ok {
		names := make([]string, 0, len(Types))
		for k := range Types {
			names = append(names, k)
		}
		slices.Sort(names)
		add("furniture type %q is not one of %v", it.Type, names)
		return
	}
	fields := map[string]*float64{"w": it.W, "d": it.D, "h": it.H, "r": it.R}
	for _, f := range spec.Required {
		if f == "mat" {
			if !slices.Contains(Materials, it.Mat) {
				add("%s: mat must be one of %v", where, Materials)
			}
			continue
		}
		if v := fields[f]; v == nil || !(*v > 0) {
			add("%s: needs %s in metres, > 0", where, f)
		}
	}
	sizes := []struct {
		name string
		v    *float64
	}{{"w", it.W}, {"d", it.D}, {"h", it.H}, {"r", it.R}, {"size", it.Size}, {"rise", it.Rise}}
	for _, sz := range sizes {
		// Nothing in an office is 50 m across; a bigger number is pixels.
		if sz.v != nil && !(*sz.v > 0 && *sz.v < 50) {
			add("%s: %s must be a positive size in metres (pixels by mistake?)", where, sz.name)
		}
	}
	if it.Z0 != nil && it.Z1 != nil && !(*it.Z1 > *it.Z0) {
		add("%s: z1 must be above z0", where)
	}
	if it.Steps != nil && (*it.Steps < 2 || *it.Steps > 40) {
		add("%s: steps must be 2–40", where)
	}
}
