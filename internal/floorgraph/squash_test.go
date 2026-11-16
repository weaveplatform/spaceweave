package floorgraph

import (
	"strings"
	"testing"
)

// Doors across reading wider than doors down, with enough of each, give
// each axis its own scale; a slanted door counts towards neither axis.
func TestScaleFromSquashedDoors(t *testing.T) {
	in := Input{Walls: append(box(0, 0, 500, 400), wall(0, 400, 400, 0)), Coverage: -1}
	for k := range 8 {
		x, y := 10+60*float64(k), 5+48*float64(k)
		in.Openings = append(in.Openings,
			Opening{A: P(x, 0), B: P(x+44, 0), Kind: Door, Leafed: true, Leaf: P(x, 44)},
			Opening{A: P(0, y), B: P(0, y+40), Kind: Door, Leafed: true, Leaf: P(40, y)},
		)
	}
	in.Openings = append(in.Openings, Opening{A: P(150, 250), B: P(178, 222), Kind: Door})
	c := mustBuild(t, in).Trace.Calibration
	if c.PxPerMetre != [2]float64{51.163, 46.512} {
		t.Errorf("scale %v, want 51.163 across, 46.512 down", c.PxPerMetre)
	}
	if !strings.Contains(c.Evidence, "8 door widths across and 8 down") ||
		!strings.Contains(c.Evidence, "differ by 10%") {
		t.Errorf("evidence %q", c.Evidence)
	}

	// The same doors, a little apart: door-to-door variation, one scale.
	for i := range in.Openings {
		if o := &in.Openings[i]; o.A.X == 0 {
			o.B.Y = o.A.Y + 42
		}
	}
	if c := mustBuild(t, in).Trace.Calibration; c.PxPerMetre[0] != c.PxPerMetre[1] {
		t.Errorf("scale %v, want one for both axes", c.PxPerMetre)
	}
}
