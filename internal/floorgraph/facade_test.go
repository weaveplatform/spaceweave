package floorgraph

import (
	"slices"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/plan"
)

// curtained is a 20 × 16 m floor with a curtain wall 0.8 m outside its top
// edge, mullions every 2 m and a cill line 0.2 m in from the glass; and,
// against the right-hand façade, a 1 m wide WC with a door and a 1 m wide
// riser with a name.
func curtained() Input {
	in := Input{
		Walls: append(box(0, 0, 1000, 800),
			wall(0, -40, 1000, -40), wall(0, -40, 0, 0), wall(1000, -40, 1000, 0),
			wall(0, -30, 1000, -30),
			wall(950, 0, 950, 150), wall(950, 150, 1000, 150),
			wall(950, 300, 950, 450), wall(950, 300, 1000, 300), wall(950, 450, 1000, 450),
		),
		Openings: []Opening{{A: P(950, 50), B: P(950, 90), Kind: Door, Weight: 1}},
		Labels:   []Label{{At: P(975, 375), Text: "RISER", Height: 8}},
		Scale:    scale50,
		Coverage: -1,
	}
	for x := 100.0; x < 1000; x += 100 {
		in.Walls = append(in.Walls, wall(x, -40, x, 0))
	}
	return in
}

func TestFacadeStripsFolded(t *testing.T) {
	r := mustBuild(t, curtained())
	names := roomNames(r)
	if len(r.Trace.Rooms) != 3 || !slices.Contains(names, "RISER") || !slices.Contains(names, openPlanName) {
		t.Errorf("rooms %v", names)
	}
	if !slices.Contains(r.Caveats, "10 narrow strips along the façade were taken as part of the wall, not rooms.") {
		t.Errorf("caveats %v", r.Caveats)
	}
	// Its lines stay as drawn: they are what the façade is built of.
	if n := len(wallsOf(r, interior)); n < 10 {
		t.Errorf("%d interior walls, want the mullions", n)
	}
}

func TestOneFacadeStrip(t *testing.T) {
	in := Input{
		Walls:    append(box(0, 0, 400, 300), wall(0, 30, 400, 30)),
		Scale:    scale50,
		Coverage: -1,
	}
	r := mustBuild(t, in)
	if len(r.Trace.Rooms) != 1 ||
		!slices.Contains(r.Caveats, "1 narrow strip along the façade was taken as part of the wall, not a room.") {
		t.Errorf("rooms %v caveats %v", roomNames(r), r.Caveats)
	}
}

func TestLabelKinds(t *testing.T) {
	// A name beats a larger number; a number and text from a layer that
	// says nothing rank alike, the larger first.
	rooms := func() []room {
		return []room{{poly: []Point{P(0, 0), P(100, 0), P(100, 100), P(0, 100)}, areaM: 4}}
	}
	for _, tc := range []struct {
		labels []Label
		want   string
	}{
		{[]Label{
			{At: P(20, 20), Text: "R1-01", Height: 20, Kind: LabelNumber},
			{At: P(50, 50), Text: "Kitchen", Height: 5, Kind: LabelName},
		}, "Kitchen"},
		{[]Label{
			{At: P(20, 20), Text: "R1-01", Height: 20, Kind: LabelNumber},
			{At: P(50, 50), Text: "Kitchen", Height: 5, Kind: LabelText},
		}, "R1-01"},
		{[]Label{
			{At: P(20, 20), Text: "R1-01", Height: 5, Kind: LabelNumber},
			{At: P(50, 50), Text: "Kitchen", Height: 20, Kind: LabelText},
		}, "Kitchen"},
	} {
		rs := rooms()
		nameRooms(rs, tc.labels)
		if rs[0].name != tc.want {
			t.Errorf("%v named %q, want %q", tc.labels, rs[0].name, tc.want)
		}
	}
}

func TestOpenPlanNames(t *testing.T) {
	big := func() []room {
		return []room{{poly: []Point{P(0, 0), P(1000, 0), P(1000, 1000), P(0, 1000)}, areaM: 400}}
	}
	for _, tc := range []struct {
		labels []Label
		want   string
	}{
		// A core or WC drawn without walls names it, not the open plan.
		{[]Label{{At: P(50, 50), Text: "CORE 2", Kind: LabelName}}, ""},
		{[]Label{{At: P(50, 50), Text: "WCs", Kind: LabelText}}, ""},
		{[]Label{{At: P(50, 50), Text: "Workspace", Kind: LabelName}}, "Workspace"},
		// Several numbers are the numbers of areas in it; one is its own.
		{[]Label{
			{At: P(50, 50), Text: "R2-01", Kind: LabelNumber},
			{At: P(500, 500), Text: "R2-02", Kind: LabelNumber},
		}, ""},
		{[]Label{
			{At: P(50, 50), Text: "R2-01", Kind: LabelNumber},
			{At: P(500, 500), Text: "R2-01", Kind: LabelNumber},
		}, "R2-01"},
	} {
		rs := big()
		nameRooms(rs, tc.labels)
		if rs[0].name != tc.want {
			t.Errorf("%v named %q, want %q", tc.labels, rs[0].name, tc.want)
		}
	}
	// Built, an unnamed open plan is called so; a small room is numbered.
	in := Input{
		Walls:    append(box(0, 0, 1000, 800), box(0, 0, 100, 100)...),
		Scale:    scale50,
		Coverage: -1,
	}
	r := mustBuild(t, in)
	if names := roomNames(r); !slices.Contains(names, openPlanName) || !slices.Contains(names, "Room 1") ||
		!slices.Contains(r.Caveats, "1 large room with no name was called Open plan.") {
		t.Errorf("rooms %v caveats %v", names, r.Caveats)
	}
	// The open plan is a zone, the walled room a room; a large named room
	// is open plan too.
	for _, rm := range r.Trace.Rooms {
		if want := map[string]string{openPlanName: plan.KindZone}[rm.Name]; rm.Kind != want {
			t.Errorf("%s is kind %q, want %q", rm.Name, rm.Kind, want)
		}
	}
	in.Labels = []Label{{At: P(500, 400), Text: "Workspace", Kind: LabelName}}
	r = mustBuild(t, in)
	kindOf := func(name string) string {
		return r.Trace.Rooms[slices.IndexFunc(r.Trace.Rooms, func(rm plan.Room) bool {
			return rm.Name == name
		})].Kind
	}
	if k := kindOf("Workspace"); k != plan.KindZone {
		t.Errorf("Workspace is kind %q", k)
	}
	// A meeting room that big is a conference suite, walled.
	in.Labels[0].Text = "Training Room"
	r = mustBuild(t, in)
	if k := kindOf("Training Room"); k != "" {
		t.Errorf("Training Room is kind %q", k)
	}
}

// Boxes on the outside of a curtain wall (planters, louvre housings) go
// with it, though wider than a strip.
func TestOutboardOfTheFacade(t *testing.T) {
	in := curtained()
	in.Walls = append(in.Walls,
		wall(200, -40, 200, -120), wall(200, -120, 400, -120), wall(400, -120, 400, -40),
		wall(300, -120, 300, -40))
	r := mustBuild(t, in)
	if len(r.Trace.Rooms) != 3 ||
		!slices.Contains(r.Caveats, "12 narrow strips along the façade were taken as part of the wall, not rooms.") {
		t.Errorf("rooms %v caveats %v", roomNames(r), r.Caveats)
	}
}
