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
