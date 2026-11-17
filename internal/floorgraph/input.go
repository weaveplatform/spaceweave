// Package floorgraph turns wall, opening and label evidence into a plan.Trace
// without a model: same input, same trace.
//
// Front ends (CAD entities, raster images) find the evidence, in source-image
// pixels: x right, y down, origin top-left. This package owns everything
// that doesn't depend on where the evidence came from: joining walls into a
// planar graph, finding the rooms it encloses, telling exterior walls from
// interior ones, naming rooms from labels, and the scale when the front end
// couldn't find one.
package floorgraph

import (
	"github.com/weaveplatform/spaceweave/internal/plan"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// Point is a position in source-image pixels.
type Point = vector.Point

// Input is everything a front end found.
type Input struct {
	Title string
	// Size is the source image's width and height in pixels.
	Size [2]int
	// Walls are wall centrelines. A wall may be broken at its openings; the
	// openings join the pieces back up.
	Walls     []Segment
	Openings  []Opening
	Labels    []Label
	Furniture []plan.Item // already in trace units: px for positions, metres for sizes
	Scale     Scale       // zero when the front end found none
	// Coverage is the share (0–1) of the drawing's ink the front end
	// explained as walls, openings, labels or furniture; negative when it
	// can't tell.
	Coverage float64
	// Notes are caveats the front end wants passed on, in its own words.
	Notes []string
}

// Segment is a stretch of wall centreline.
type Segment struct {
	A, B      Point
	Thickness float64 // px, face to face
	Glass     bool    // a glazed partition or curtain wall
	// Weight is how sure the front end is that this is a wall, 0–1.
	Weight float64
}

// Opening kinds, as plan.Opening.Kind.
const (
	Door       = "door"
	DoubleDoor = "double-door"
	Window     = "window"
	Gap        = "opening"
)

// Opening is a door, window or plain gap in a wall. A and B are its two
// ends on (or within half a wall thickness of) the wall's centreline.
type Opening struct {
	A, B Point
	Kind string
	// HingeA says the single leaf hangs at A rather than B. Only for Door
	// with Leaf set.
	HingeA bool
	// Leaf is a point on the side the leaf opens into, such as the middle of
	// its swing arc; Leafed says it was found. Doors without it get a swing
	// guessed from the room sizes and a caveat.
	Leaf   Point
	Leafed bool
	// Weight is how sure the front end is, 0–1.
	Weight float64
}

// Label is a piece of text in the drawing.
type Label struct {
	At     Point // the middle of the text
	Text   string
	Height float64 // px
}

// Scale is the image's scale on the building.
type Scale struct {
	PxPerMetre [2]float64 // across, down
	// Exact means it comes from the drawing's stated units; otherwise it was
	// measured or assumed, and Evidence says how.
	Exact    bool
	Evidence string
}

// Known reports whether the scale was found.
func (s Scale) Known() bool { return s.PxPerMetre[0] > 0 && s.PxPerMetre[1] > 0 }

// Result is the trace and how far to trust it.
type Result struct {
	Trace *plan.Trace
	// Confidence is 0–1, computed from the evidence (see Build).
	Confidence float64
	Summary    string
	Caveats    []string
}
