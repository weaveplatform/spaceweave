package floorgraph

import "testing"

// A thin wall that stops inside a thick one, off its centreline and
// running alongside it, meets it on the drawing; the rooms either side
// close. The geometry is made up: a 20 px wall and a 4 px one end to end
// with their lower faces in line, and a 4 px partition stopping against a
// 30 px column's face.
func TestEndsInsideAThickWallAnchor(t *testing.T) {
	thick := func(x0, y0, x1, y1, t float64) Segment {
		return Segment{A: P(x0, y0), B: P(x1, y1), Thickness: t, Weight: 1}
	}
	in := Input{
		Walls: []Segment{
			thick(0, 0, 200, 0, 20),
			thick(200, 8, 400, 8, 4), // ends 8 px off the thick wall's end, inside it
			wall(400, 8, 400, 300),
			wall(400, 300, 0, 300),
			wall(0, 300, 0, 0),
			thick(185, 150, 215, 150, 30), // a column
			thick(200, 162, 200, 300, 4),  // stops against its face
			thick(200, 138, 200, 0, 4),
		},
		Scale:    scale50,
		Coverage: -1,
	}
	r := mustBuild(t, in)
	if len(r.Trace.Rooms) != 2 {
		t.Fatalf("rooms %v walls %+v", roomNames(r), r.Trace.Walls)
	}
}

func TestAnchor(t *testing.T) {
	segs := []seg{
		newSeg(P(0, 0), P(100, 0), P(1, 0), 0, 20, 1, false),
		newSeg(P(50, 0.2), P(50, 50), P(0, 1), 1, 4, 1, false),
		newSeg(P(150, 5), P(200, 5), P(1, 0), 2, 4, 1, false),
		newSeg(P(100, 9), P(100, 60), P(0, 1), 3, 4, 1, false),
	}
	if _, ok := anchor(segs, 1, segs[1].a); ok {
		t.Error("an end on the centreline already touches it")
	}
	if _, ok := anchor(segs, 2, segs[2].a); ok {
		t.Error("an end clear of every wall's thickness was anchored")
	}
	if p, ok := anchor(segs, 3, segs[3].a); !ok || p != P(100, 0) {
		t.Errorf("anchor %v %v, want the thick wall's end", p, ok)
	}
}
