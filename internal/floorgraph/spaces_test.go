package floorgraph

import (
	"bytes"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/plan"
)

// officeFloor is a 20 × 16 m floor with a 4 × 4 m office in its top-left
// corner, entered through a 0.9 m door in its right-hand wall, and a label
// in the office. The open floor round it has no label.
func officeFloor() Input {
	return Input{
		Walls: append(box(0, 0, 1000, 800),
			wall(0, 200, 200, 200),
			wall(200, 0, 200, 80), wall(200, 125, 200, 200),
		),
		Openings: []Opening{{A: P(200, 80), B: P(200, 125), Kind: Door, Weight: 1}},
		Labels:   []Label{{At: P(100, 100), Text: "Office", Height: 8, Kind: LabelName}},
		Scale:    scale50,
		Coverage: -1,
	}
}

// areaM is a trace room's area in m² at scale50.
func areaM(rm plan.Room) float64 { return polyArea(rm.Polygon) / (50 * 50) }

func near(got, want, tol float64) bool { return math.Abs(got-want) <= tol }

func TestSpacesOffice(t *testing.T) {
	r := mustBuild(t, officeFloor())
	office := roomNamed(t, r, "Office")
	if office.Kind == plan.KindZone || !near(areaM(office), 16, 0.3) {
		t.Errorf("office kind %q, %.2f m², want a 16 m² room", office.Kind, areaM(office))
	}
	// The rest of the floor has no label and no walls of its own: a zone.
	floor := roomNamed(t, r, openPlanName)
	if floor.Kind != plan.KindZone || !near(areaM(floor), 320-16, 1) {
		t.Errorf("floor kind %q, %.2f m², want a 304 m² zone", floor.Kind, areaM(floor))
	}
	if len(r.Trace.Rooms) != 2 {
		t.Errorf("rooms %v", roomNames(r))
	}
}

// A partition drawn 0.2 m short of the façade leaves a gap no face closes;
// the seal does, and the office keeps to its walls.
func TestSpacesSmallWallGap(t *testing.T) {
	in := officeFloor()
	in.Walls[5] = wall(200, 13, 200, 80)
	r := mustBuild(t, in)
	if office := roomNamed(t, r, "Office"); office.Kind == plan.KindZone || !near(areaM(office), 16, 0.5) {
		t.Errorf("office kind %q, %.2f m², want a 16 m² room", office.Kind, areaM(office))
	}
}

// A gap wider than a door closes when the front end found a doorway in it
// (door ink, a leaf), and is a walkway, joining the floor, when not.
func TestSpacesDoorwayAndWalkway(t *testing.T) {
	in := officeFloor()
	in.Walls[5], in.Walls[6] = wall(200, 0, 200, 40), wall(200, 120, 200, 200)
	in.Labels = nil
	in.Labels = append(in.Labels, Label{At: P(600, 600), Text: "Workspace", Height: 8, Kind: LabelName})
	in.Openings = []Opening{{A: P(200, 40), B: P(200, 120), Kind: Gap, Weight: 0.6, Doorway: true}}
	r := mustBuild(t, in)
	if names := roomNames(r); !slices.Contains(names, "Room 1") || !slices.Contains(names, "Workspace") {
		t.Errorf("with a doorway, rooms %v, want the store and the floor", names)
	}
	in.Openings[0].Doorway = false
	r = mustBuild(t, in)
	if names := roomNames(r); len(names) != 1 || names[0] != "Workspace" {
		t.Errorf("with a walkway, rooms %v, want the floor alone", names)
	}
	if ws := roomNamed(t, r, "Workspace"); ws.Kind != plan.KindZone || !near(areaM(ws), 320, 1) {
		t.Errorf("Workspace kind %q, %.2f m², want the whole 320 m² floor as a zone", ws.Kind, areaM(ws))
	}
	// A door kind closes it whatever its width.
	in.Openings[0].Kind = DoubleDoor
	r = mustBuild(t, in)
	if names := roomNames(r); len(names) != 2 {
		t.Errorf("with a double door, rooms %v", names)
	}
}

// Two names in one walled room: no wall is invented between them; the room
// takes the better and a caveat lists the other.
func TestSpacesTwoLabelsOneRoom(t *testing.T) {
	in := officeFloor()
	in.Labels = append(in.Labels, Label{At: P(60, 160), Text: "Print", Height: 5, Kind: LabelName})
	r := mustBuild(t, in)
	if names := roomNames(r); len(names) != 2 || !slices.Contains(names, "Office") {
		t.Errorf("rooms %v", names)
	}
	want := `"Office" also holds the label "Print", with no wall between them`
	if !slices.ContainsFunc(r.Caveats, func(c string) bool { return strings.Contains(c, want) }) {
		t.Errorf("caveats %v", r.Caveats)
	}
}

// An unlabelled cupboard is numbered; a sliver behind it is not a room.
func TestSpacesUnlabelledCupboard(t *testing.T) {
	in := officeFloor()
	in.Walls = append(in.Walls,
		wall(400, 0, 400, 50), wall(400, 50, 450, 50), wall(450, 50, 450, 0))
	r := mustBuild(t, in)
	cupboard := roomNamed(t, r, "Room 1")
	if cupboard.Kind == plan.KindZone || !near(areaM(cupboard), 1, 0.15) {
		t.Errorf("cupboard kind %q, %.2f m², want a 1 m² room", cupboard.Kind, areaM(cupboard))
	}
	if len(r.Trace.Rooms) != 3 {
		t.Errorf("rooms %v", roomNames(r))
	}
}

// A label in the open floor names the zone; a core standing in it is a
// room of its own, and the zone goes round it.
func TestSpacesZoneNamedByLabel(t *testing.T) {
	in := officeFloor()
	in.Walls = append(in.Walls, box(500, 300, 700, 500)...)
	in.Labels = append(in.Labels,
		Label{At: P(800, 700), Text: "Workspace", Height: 8, Kind: LabelName},
		Label{At: P(600, 400), Text: "Core", Height: 8, Kind: LabelName})
	r := mustBuild(t, in)
	ws := roomNamed(t, r, "Workspace")
	if ws.Kind != plan.KindZone || !near(areaM(ws), 320-16-16, 1) {
		t.Errorf("Workspace kind %q, %.2f m², want a 288 m² zone", ws.Kind, areaM(ws))
	}
	if core := roomNamed(t, r, "Core"); core.Kind == plan.KindZone || !near(areaM(core), 16, 0.3) {
		t.Errorf("core kind %q, %.2f m²", core.Kind, areaM(core))
	}
}

// The same evidence with its labels in another order builds the same bytes.
func TestSpacesDeterministic(t *testing.T) {
	in := officeFloor()
	in.Walls = append(in.Walls, box(500, 300, 700, 500)...)
	in.Labels = append(in.Labels,
		Label{At: P(60, 160), Text: "Print", Height: 5, Kind: LabelName},
		Label{At: P(800, 700), Text: "Workspace", Height: 8, Kind: LabelName})
	a := marshal(t, in)
	slices.Reverse(in.Labels)
	slices.Reverse(in.Walls)
	if b := marshal(t, in); !bytes.Equal(a, b) {
		t.Error("labels and walls in another order built different plans")
	}
}
