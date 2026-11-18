package floorgraph

import "testing"

// A thickness change is a junction, even when the two wall runs overlap a
// little. Averaging their widths erases the thicker run's measured faces.
func TestMergePreservesThicknessChange(t *testing.T) {
	for _, overlap := range []float64{0, 1, 10} {
		thin := newSeg(P(0, 0), P(100, 0), P(1, 0), 0, 10, .8, false)
		thick := newSeg(P(100-overlap, 0), P(200, 0), P(1, 0), 0, 30, .8, false)
		got := mergeCollinear([]seg{thin, thick})
		if len(got) != 2 {
			t.Errorf("overlap %g: got %d walls, want separate thicknesses", overlap, len(got))
		}
		for _, sample := range []struct{ x, thickness float64 }{{50, 10}, {150, 30}} {
			found := false
			for _, s := range got {
				if s.a.X <= sample.x && s.b.X >= sample.x {
					found = true
					if s.t != sample.thickness {
						t.Errorf("width at %g = %g, want %g", sample.x, s.t, sample.thickness)
					}
				}
			}
			if !found {
				t.Errorf("wall evidence missing at %g", sample.x)
			}
		}
	}
}

func TestTracePreservesMeasuredThickness(t *testing.T) {
	in := partitioned()
	in.Walls[5].Thickness = 7
	r := mustBuild(t, in)
	found := false
	for _, w := range r.Trace.Walls {
		if w.Kind != interior || w.From[1] != w.To[1] {
			continue
		}
		found = true
		if w.Thickness != .14 {
			t.Errorf(
				"measured 140 mm partition has override %g, default %g",
				w.Thickness,
				r.Trace.Thickness.Interior,
			)
		}
	}
	if !found {
		t.Fatal("measured partition is missing")
	}
}

// Thick façade faces can be over 1.2 m apart at their centrelines while
// leaving less than 1.2 m clear. Keeping their true thickness must not turn
// the construction's repeated bays into rooms.
func TestThickFacadeClearWidth(t *testing.T) {
	in := Input{
		Walls: box(0, 0, 1000, 800), Scale: scale50, Coverage: -1,
		Labels: []Label{{At: P(500, 400), Text: "Workspace", Height: 10}},
	}
	in.Walls[0].Thickness = 20
	in.Walls = append(in.Walls, thick(wall(0, 70, 1000, 70), 20))
	for x := 100.0; x < 1000; x += 100 {
		in.Walls = append(in.Walls, wall(x, 0, x, 70))
	}
	r := mustBuild(t, in)
	if len(r.Trace.Rooms) != 1 {
		t.Errorf("got %d rooms, want one floor and no facade bays", len(r.Trace.Rooms))
	}
	in.Labels = append(in.Labels, Label{At: P(50, 35), Text: "Riser", Height: 5})
	if got := mustBuild(t, in); len(got.Trace.Rooms) != 2 {
		t.Errorf("labelled bay lost: got %d rooms", len(got.Trace.Rooms))
	}
}

func TestFacadeGridWidthUncertainty(t *testing.T) {
	b := &builder{sc: scaleInfo{sx: 50, sy: 100}}
	var spaces []space
	for i := range 3 {
		x := float64(i) * 100
		outer := []Point{P(x, 0), P(x+100, 0), P(x+100, 150), P(x, 150)}
		raw := []Point{P(x, 14), P(x+100, 14), P(x+100, 136), P(x, 136)}
		spaces = append(
			spaces,
			space{poly: outer, outer: outer, rawOuter: raw, width: 1.5, areaM: 3},
		)
	}
	for _, slack := range []float64{0, .05} {
		got := b.spaceStrips(spaces, nil, slack, func(space) bool { return false })
		want := 0
		if slack > 0 {
			want = 3
		}
		if len(got) != want {
			t.Errorf("slack %g: got %d facade bays, want %d", slack, len(got), want)
		}
	}
}
