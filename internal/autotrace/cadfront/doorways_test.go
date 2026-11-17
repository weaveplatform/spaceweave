package cadfront

import (
	"math"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
	"github.com/weaveplatform/spaceweave/internal/floorgraph"
)

// splitBox is an 8 x 5 m box of 200 mm walls split at x = 4000 by a
// 100 mm partition with a gap from y = 1000 to y = 1000+gap; more adds
// whatever lies in it.
func splitBox(gap float64, more ...dxftest.Ent) *dxftest.Doc {
	const wall = "AR_EF_25_10-X_Walls"
	d := dxftest.New(4).Layer(wall, 7, 0).Layer("AR_EF_25_30-M_Internal Door", 3, 0)
	d.Add(dxftest.Line(wall, 0, 0, 8000, 0), dxftest.Line(wall, 200, 200, 7800, 200))
	d.Add(dxftest.Line(wall, 0, 5000, 8000, 5000), dxftest.Line(wall, 200, 4800, 7800, 4800))
	d.Add(dxftest.Line(wall, 0, 0, 0, 5000), dxftest.Line(wall, 200, 200, 200, 4800))
	d.Add(dxftest.Line(wall, 8000, 0, 8000, 5000), dxftest.Line(wall, 7800, 200, 7800, 4800))
	for _, x := range []float64{3950, 4050} {
		d.Add(dxftest.Line(wall, x, 200, x, 1000), dxftest.Line(wall, x, 1000+gap, x, 4800))
	}
	return d.Add(more...)
}

func rooms(t *testing.T, d *dxftest.Doc) (floorgraph.Input, Options, int) {
	t.Helper()
	in, o := read(t, d)
	res, err := floorgraph.Build(in)
	if err != nil {
		t.Fatal(err)
	}
	return in, o, len(res.Trace.Rooms)
}

// A door drawn ajar, or one with no swing at all, is door ink in a
// doorway; the doorway closes the rooms either side.
func TestDoorwaysWithoutASwing(t *testing.T) {
	const door = "AR_EF_25_30-M_Internal Door"
	ajar := []dxftest.Ent{
		// Frames each side, a leaf open 20 degrees, its 20 degree arc.
		dxftest.Rect(door, 3950, 950, 4050, 1000), dxftest.Rect(door, 3950, 1900, 4050, 1950),
		dxftest.Line(door, 4000, 1000, 4000+900*math.Sin(math.Pi/9), 1000+900*math.Cos(math.Pi/9)),
		dxftest.Arc(door, 4000, 1000, 900, 70, 90),
	}
	for name, c := range map[string]struct {
		gap   float64
		ink   []dxftest.Ent
		rooms int
	}{
		"ajar":                {900, ajar, 2},
		"empty door-wide gap": {900, nil, 2},
		"empty walkway":       {1500, nil, 1},
		"sliding door, 1.5 m": {1500, []dxftest.Ent{dxftest.Rect(door, 3980, 1000, 4020, 1800)}, 2},
		"wider than any door": {3500, []dxftest.Ent{dxftest.Rect(door, 3980, 1000, 4020, 1800)}, 1},
	} {
		t.Run(name, func(t *testing.T) {
			in, o, n := rooms(t, splitBox(c.gap, c.ink...))
			if n != c.rooms {
				t.Fatalf("%d rooms, want %d; openings %v", n, c.rooms, in.Openings)
			}
			if c.rooms != 2 {
				return
			}
			op, ok := findOpening(in, mm(o, 4000, 1000), mm(o, 4000, 1000+c.gap), 0.03*o.PxPerMetre)
			if !ok {
				op, ok = findOpening(in, mm(o, 4000, 1000+c.gap), mm(o, 4000, 1000), 0.03*o.PxPerMetre)
			}
			if !ok || op.Kind != floorgraph.Gap {
				t.Errorf("no plain opening across the gap: %v", in.Openings)
			}
		})
	}
}

// A partition stopping short of the wall ahead leaves a walkway unless a
// door's ink is in it: open plan keeps its gaps.
func TestGapToTheWallAhead(t *testing.T) {
	const wall, door = "AR_EF_25_10-X_Walls", "AR_EF_25_30-M_Internal Door"
	build := func(ink ...dxftest.Ent) *dxftest.Doc {
		d := dxftest.New(4).Layer(wall, 7, 0).Layer(door, 3, 0)
		d.Add(dxftest.Line(wall, 0, 0, 8000, 0), dxftest.Line(wall, 200, 200, 7800, 200))
		d.Add(dxftest.Line(wall, 0, 5000, 8000, 5000), dxftest.Line(wall, 200, 4800, 7800, 4800))
		d.Add(dxftest.Line(wall, 0, 0, 0, 5000), dxftest.Line(wall, 200, 200, 200, 4800))
		d.Add(dxftest.Line(wall, 8000, 0, 8000, 5000), dxftest.Line(wall, 7800, 200, 7800, 4800))
		d.Add(dxftest.Line(wall, 3950, 200, 3950, 3900), dxftest.Line(wall, 4050, 200, 4050, 3900))
		return d.Add(ink...)
	}
	if _, _, n := rooms(t, build()); n != 1 {
		t.Errorf("%d rooms across an empty 900 mm walkway, want 1", n)
	}
	if in, _, n := rooms(t, build(dxftest.Rect(door, 3980, 3900, 4020, 4700))); n != 2 {
		t.Errorf("%d rooms with a door in the gap, want 2; openings %v", n, in.Openings)
	}
}

// A doorway a door's swing already spans gets no second opening.
func TestDoorwayLeftToItsDoor(t *testing.T) {
	const door = "AR_EF_25_30-M_Internal Door"
	in, o, n := rooms(t, splitBox(900,
		dxftest.Line(door, 4050, 1000, 4950, 1000), dxftest.Arc(door, 4050, 1000, 900, 0, 90)))
	if n != 2 {
		t.Fatalf("%d rooms, want 2", n)
	}
	k := 0
	for _, op := range in.Openings {
		if m := op.A.Add(op.B).Mul(0.5); close2(m, mm(o, 4000, 1450), 0.2*o.PxPerMetre) {
			k++
		}
	}
	if k != 1 {
		t.Errorf("%d openings in the doorway, want its door alone: %v", k, in.Openings)
	}
}

// A wall drawn as a bundle whose outer finish lines stop short of its
// core keeps the core's length: the finish lines falling short is no
// corner overshoot to split.
func TestBundleWithShortFinishLines(t *testing.T) {
	const layer = "AR_EF_25_10-X_Walls"
	d := dxftest.New(4).Layer(layer, 7, 0)
	d.Add(dxftest.Line(layer, 150, 0, 5850, 0), dxftest.Line(layer, 150, 130, 5850, 130))
	for _, y := range []float64{15, 35, 95, 115} {
		d.Add(dxftest.Line(layer, 0, y, 6000, y))
	}
	d.Add(dxftest.Line(layer, 0, 3000, 6000, 3000), dxftest.Line(layer, 0, 3100, 6000, 3100))
	in, o := read(t, d)
	var long float64
	for _, w := range in.Walls {
		if m := w.A.Add(w.B).Mul(0.5); math.Abs(m.Y-mm(o, 0, 65).Y) < 0.1*o.PxPerMetre {
			long = math.Max(long, w.B.Sub(w.A).Len()/o.PxPerMetre)
		}
	}
	if long < 5.95 {
		t.Errorf("the bundle's wall is %.2f m long, want its core's 6 m; walls %v", long, in.Walls)
	}
}
