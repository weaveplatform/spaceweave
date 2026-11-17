package floorgraph

import "testing"

func TestObservedDoorWinsOverWeakOverlap(t *testing.T) {
	in := officeFloor()
	in.Openings[0].Leafed = true
	in.Openings[0].Leaf = P(220, 100)
	in.Openings = append(
		in.Openings,
		Opening{A: P(200, 75), B: P(200, 130), Kind: Gap, Weight: 0.2},
	)
	r := mustBuild(t, in)
	if stats := r.Openings; stats.Input != stats.Placed+stats.Unplaced+stats.Narrow+stats.Overlapped {
		t.Fatalf("lost hypotheses: %+v", stats)
	}
	doors := 0
	for _, w := range r.Trace.Walls {
		for _, o := range w.Openings {
			if o.Kind == Door {
				doors++
			}
		}
	}
	if doors != 1 {
		t.Fatalf("observed door erased by weak overlapping gap: %d doors", doors)
	}
}
