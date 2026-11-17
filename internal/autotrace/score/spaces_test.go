package score

import (
	"math"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/plan"
)

func TestSpaceIdentity(t *testing.T) {
	room := func(name string) plan.Room {
		return plan.Room{
			Name:    name,
			Number:  "01",
			Polygon: [][2]float64{{0, 0}, {100, 0}, {100, 100}, {0, 100}},
		}
	}
	want := &plan.Trace{Rooms: []plan.Room{room("Office 01")}}
	for _, tc := range []struct {
		name           string
		rooms          []plan.Room
		matched, named int
	}{
		{"exact", []plan.Room{room(" office   01 ")}, 1, 1},
		{"prefix is insufficient", []plan.Room{room("Office")}, 1, 0},
		{"duplicates", []plan.Room{room("Office 01"), room("Office 01")}, 1, 1},
		{"missing", nil, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := SpaceIdentity(&plan.Trace{Rooms: tc.rooms}, want)
			if got.Matched != tc.matched || got.Named != tc.named || got.NameWant != 1 ||
				got.Got != len(tc.rooms) {
				t.Fatalf("%+v", got)
			}
		})
	}
	wrong := room("Office 01")
	wrong.Number = "02"
	if got := SpaceIdentity(&plan.Trace{Rooms: []plan.Room{wrong}}, want); got.Named != 0 {
		t.Fatalf("wrong number accepted: %+v", got)
	}
}

func TestSpaceIdentityReportsSplitsAndBoundaryError(t *testing.T) {
	room := func(x0, x1 float64) plan.Room {
		return plan.Room{
			Name:    "Zone",
			Kind:    plan.KindZone,
			Polygon: [][2]float64{{x0, 0}, {x1, 0}, {x1, 100}, {x0, 100}},
		}
	}
	want := &plan.Trace{Rooms: []plan.Room{room(0, 100)}}
	got := &plan.Trace{Rooms: []plan.Room{room(0, 50), room(50, 100)}}
	if result := SpaceIdentity(got, want); result.Splits != 1 || result.Matched != 0 {
		t.Fatalf("split presented as a matched room: %+v", result)
	}
	if result := SpaceIdentity(want, got); result.Merges != 1 {
		t.Fatalf("merge not reported: %+v", result)
	}
	shifted := &plan.Trace{Rooms: []plan.Room{room(1, 101)}}
	result := SpaceIdentity(shifted, want)
	if result.Matched != 1 || result.ZoneMatched != 1 || result.MeanBoundaryErrorPx <= 0 {
		t.Fatalf("missing geometry diagnostics: %+v", result)
	}
	duplicateTruth := &plan.Trace{Rooms: []plan.Room{room(0, 100), room(0, 100)}}
	if result := SpaceIdentity(want, duplicateTruth); result.Matched != 1 {
		t.Fatalf("one prediction matched duplicate references: %+v", result)
	}
}

func TestSpaceConflicts(t *testing.T) {
	box := func(x0, y0, x1, y1 float64) plan.Room {
		return plan.Room{Polygon: [][2]float64{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}}
	}
	for _, tc := range []struct {
		name  string
		rooms []plan.Room
		pairs int
	}{
		{"empty", nil, 0},
		{"adjacent", []plan.Room{box(0, 0, 100, 100), box(100, 0, 200, 100)}, 0},
		{"rounding overlap", []plan.Room{box(0, 0, 100, 100), box(99.5, 0, 200, 100)}, 0},
		{"subpixel", []plan.Room{box(0, 0, .5, 1), box(0, 0, .5, 1)}, 0},
		{"empty outline", []plan.Room{{}, box(0, 0, 100, 100)}, 0},
		{"enclosed", []plan.Room{box(0, 0, 100, 100), box(20, 20, 60, 60)}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conflicts := SpaceConflicts(&plan.Trace{Rooms: tc.rooms})
			if len(conflicts) != tc.pairs {
				t.Fatalf("got %v, want %d overlap pairs", conflicts, tc.pairs)
			}
			if tc.pairs > 0 && (conflicts[0].First != 0 || conflicts[0].Second != 1 ||
				math.Abs(conflicts[0].FirstFraction-.16) > 1e-9 || conflicts[0].SecondFraction != 1) {
				t.Fatalf("incorrect overlap fractions: %+v", conflicts)
			}
		})
	}
}
