package floorgraph

import (
	"bytes"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/plan"
)

func explicitBox(x0, y0, x1, y1 float64, name, number, kind string) Space {
	return Space{
		Polygon: [][2]float64{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}},
		Name:    name,
		Number:  number,
		Kind:    kind,
	}
}

func TestPackageSpaces(t *testing.T) {
	in := officeFloor()
	in.Labels = nil
	in.Spaces = []Space{
		explicitBox(0, 0, 200, 200, "Unlabelled office", "01", plan.KindRoom),
		explicitBox(200, 0, 1000, 800, "Desk neighbourhood", "02", plan.KindZone),
	}
	r := mustBuild(t, in)
	if office := roomNamed(
		t,
		r,
		"Unlabelled office",
	); office.Number != "01" ||
		office.Kind == plan.KindZone {
		t.Fatalf("source identity lost: %+v", office)
	}
	if zone := roomNamed(
		t,
		r,
		"Desk neighbourhood",
	); zone.Number != "02" ||
		zone.Kind != plan.KindZone {
		t.Fatalf("source zone lost: %+v", zone)
	}
	a := marshal(t, in)
	slices.Reverse(in.Spaces)
	if !bytes.Equal(a, marshal(t, in)) {
		t.Fatal("space ordering changed trace")
	}
}

func TestPackageSpacesPartialAndVoid(t *testing.T) {
	in := officeFloor()
	in.Spaces = []Space{explicitBox(500, 0, 1000, 800, "East", "02", plan.KindZone)}
	r := mustBuild(t, in)
	if east := roomNamed(t, r, "East"); !near(areaM(east), 160, 0.1) {
		t.Fatal(areaM(east))
	}
	roomNamed(t, r, "Office")
	var total float64
	for _, rm := range r.Trace.Rooms {
		total += areaM(rm)
	}
	if !near(total, 320, 1) {
		t.Fatalf("partial package lost floor: %.2f", total)
	}
	// A source zone spanning a core must keep its hole, rather than fill it.
	in.Walls = append(in.Walls, box(600, 300, 700, 400)...)
	in.Voids = []Point{P(650, 350)}
	r = mustBuild(t, in)
	want := 160 - polyArea(r.Trace.Voids[0])/(50*50)
	if east := roomNamed(t, r, "East"); !near(areaM(east), want, 0.01) {
		t.Fatal(areaM(east))
	}
}

func TestPackageSpaceSnapTrials(t *testing.T) {
	for _, mode := range []SpaceSnap{SpaceSnapWalls, SpaceSnapNone, SpaceSnap300mm} {
		t.Run([]string{"none", "walls", "300mm"}[mode], func(t *testing.T) {
			in := officeFloor()
			in.SpaceSnap = mode
			in.Spaces = []Space{explicitBox(3, 3, 197, 197, "Meeting", "", plan.KindRoom)}
			r := mustBuild(t, in)
			want := 16.0
			if mode == SpaceSnapNone {
				want = math.Pow(194.0/50, 2)
			}
			if got := areaM(roomNamed(t, r, "Meeting")); !near(got, want, 0.01) {
				t.Fatalf("area %v, want %v", got, want)
			}
		})
	}
}

func TestPackageSpaceInvalidAndNumber(t *testing.T) {
	in := officeFloor()
	in.Spaces = []Space{
		{Name: "Broken", Polygon: [][2]float64{{0, 0}, {100, 100}, {0, 100}, {100, 0}}},
	}
	r := mustBuild(t, in)
	roomNamed(t, r, "Office")
	if len(r.Caveats) == 0 {
		t.Fatal("invalid source outline wasn't reported")
	}
	in.Spaces = []Space{explicitBox(0, 0, 200, 200, "", "0.01", plan.KindRoom)}
	roomNamed(t, mustBuild(t, in), "0.01")
}

func TestContainingZoneKeepsSmallerRooms(t *testing.T) {
	in := officeFloor()
	in.Spaces = []Space{
		explicitBox(0, 0, 1000, 800, "Whole-floor zone", "01", plan.KindZone),
		explicitBox(0, 0, 200, 200, "Protected room", "02", plan.KindRoom),
	}
	r := mustBuild(t, in)
	if len(r.Trace.Rooms) != 2 {
		t.Fatalf("containing zone swallowed its room: %v", roomNames(r))
	}
	if got := areaM(roomNamed(t, r, "Whole-floor zone")); !near(got, 304, 0.01) {
		t.Fatalf("zone area %v", got)
	}
	if got := areaM(roomNamed(t, r, "Protected room")); !near(got, 16, 0.01) {
		t.Fatalf("room area %v", got)
	}
	if !slices.ContainsFunc(
		r.Caveats,
		func(s string) bool { return strings.Contains(s, "lost 16.00 m²") },
	) {
		t.Fatal("substantial source clipping wasn't reported")
	}
}

func TestPackageSpaceBeyondInferredFloor(t *testing.T) {
	// An unclosed bay is pruned from the wall graph's outer face. Its
	// registered outline still supplies floor supported by the drawing.
	in := Input{
		Size: [2]int{700, 450},
		Walls: append(box(0, 0, 500, 400),
			wall(500, 0, 650, 0), wall(650, 0, 650, 100)),
		Scale:  scale50,
		Spaces: []Space{explicitBox(500, 0, 650, 100, "Bay", "01", plan.KindRoom)},
	}
	r := mustBuild(t, in)
	if bay := roomNamed(t, r, "Bay"); !near(areaM(bay), 6, 0.01) {
		t.Fatalf("registered bay lost floor: %.3f m²", areaM(bay))
	}
}

func TestPackageSpaceResidualWidth(t *testing.T) {
	in := officeFloor()
	// Cutting the inferred office back by 20 cm leaves a wall-face strip,
	// not a second office. Uncovered usable floor still needs inference.
	in.Spaces = []Space{explicitBox(0, 0, 190, 200, "Registered office", "01", plan.KindRoom)}
	r := mustBuild(t, in)
	if names := roomNames(r); len(names) != 2 || slices.Contains(names, "Office") {
		t.Fatalf("clipping promoted a wall strip to a room: %v", names)
	}
	roomNamed(t, r, openPlanName)

	// A deliberately supplied narrow space has semantic evidence that an
	// inferred clipping fragment lacks, so it must retain its identity.
	in.Spaces = []Space{explicitBox(0, 0, 10, 400, "Service strip", "02", plan.KindZone)}
	if strip := roomNamed(t, mustBuild(t, in), "Service strip"); !near(areaM(strip), 1.6, 0.01) {
		t.Fatalf("explicit narrow space changed: %.3f m²", areaM(strip))
	}
}

func TestPackageSpaceImageBounds(t *testing.T) {
	in := Input{
		Size: [2]int{600, 450},
		Walls: append(box(0, 0, 500, 400),
			wall(500, 0, 650, 0), wall(650, 0, 650, 100)),
		Scale:  scale50,
		Spaces: []Space{explicitBox(500, 0, 650, 100, "Bay", "01", plan.KindRoom)},
	}
	r := mustBuild(t, in)
	if bay := roomNamed(t, r, "Bay"); !near(areaM(bay), 4, 0.01) {
		t.Fatalf("source escaped image bounds: %.3f m²", areaM(bay))
	}
	if !slices.ContainsFunc(r.Caveats, func(c string) bool {
		return strings.Contains(c, "image boundary")
	}) {
		t.Fatal("image clipping was not reported")
	}
}

func TestPackageSpaceWithoutClosedWalls(t *testing.T) {
	in := Input{
		Walls:  []Segment{wall(0, 0, 500, 0), wall(500, 0, 500, 400)},
		Scale:  scale50,
		Spaces: []Space{explicitBox(0, 0, 500, 400, "Documented floor", "01", plan.KindZone)},
	}
	if rm := roomNamed(t, mustBuild(t, in), "Documented floor"); !near(areaM(rm), 80, 0.01) {
		t.Fatalf("source lost because walls did not close: %.3f m²", areaM(rm))
	}
}

func TestPackageFootprintRetainsBaysAndRecesses(t *testing.T) {
	in := Input{
		Size: [2]int{700, 450},
		Walls: append(box(0, 0, 500, 400),
			wall(500, 0, 650, 0), wall(650, 0, 650, 100)),
		Scale:  scale50,
		Spaces: []Space{explicitBox(500, 0, 650, 100, "Bay", "01", plan.KindRoom)},
	}
	r := mustBuild(t, in)
	if len(r.Trace.Footprint) != 1 || len(r.Trace.Footprint[0]) != 1 {
		t.Fatalf("registered package has no slab footprint: %v", r.Trace.Footprint)
	}
	poly := points(r.Trace.Footprint[0][0])
	if !inside(P(600, 50), poly) || inside(P(600, 300), poly) {
		t.Fatal("slab lost the bay or filled the adjacent recess")
	}
	// Its 86 m² L shape grows to the wall faces, 6 cm beyond centrelines.
	if area := math.Abs(signedArea(poly)) / (50 * 50); !near(area, 88.5344, .001) {
		t.Fatalf("slab area %.5f m²", area)
	}
}

func TestPackageFootprintHolesAndSeparateParts(t *testing.T) {
	in := officeFloor()
	in.Walls = append(in.Walls, box(600, 300, 700, 400)...)
	in.Walls = append(in.Walls, wall(1200, 0, 1300, 0), wall(1300, 0, 1300, 100))
	in.Voids = []Point{P(650, 350)}
	in.Spaces = []Space{
		explicitBox(0, 0, 1000, 800, "Main floor", "01", plan.KindZone),
		explicitBox(1200, 0, 1300, 100, "Separate bay", "02", plan.KindRoom),
	}
	r := mustBuild(t, in)
	if len(r.Trace.Footprint) != 2 {
		t.Fatalf("separate floor parts lost: %v", r.Trace.Footprint)
	}
	var holes int
	var holeArea float64
	for _, polygon := range r.Trace.Footprint {
		for _, ring := range polygon[1:] {
			holes++
			holeArea += polyArea(ring)
		}
	}
	if holes != 1 || !near(holeArea, polyArea(r.Trace.Voids[0]), .001) {
		t.Fatalf("void changed while offsetting slab: %d holes, area %f", holes, holeArea)
	}
}

func TestPackageRejectsUnusableSourceCoordinates(t *testing.T) {
	for _, poly := range [][][2]float64{
		{{0, 0}},
		{{0, 0}, {1, math.NaN()}, {0, 1}},
		{{0, 0}, {maxCoord + 1, 0}, {0, 1}},
	} {
		in := officeFloor()
		in.Spaces = []Space{{Name: "Invalid", Polygon: poly}}
		r := mustBuild(t, in)
		roomNamed(t, r, "Office")
		if !slices.ContainsFunc(
			r.Caveats,
			func(s string) bool { return strings.Contains(s, "invalid outline") },
		) {
			t.Fatal("unusable source outline was not reported")
		}
	}
}
