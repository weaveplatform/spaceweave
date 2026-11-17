package synth

import (
	"fmt"

	"github.com/weaveplatform/spaceweave/internal/plan"
)

// Builder assembles a small trace in code, for tests that want one wall or
// one door rather than a whole fixture.
type Builder struct {
	t plan.Trace
}

// New starts a trace at pxPerMetre on both axes, with walls 0.2 m outside,
// 0.1 m inside and 0.05 m of glass.
func New(pxPerMetre float64) *Builder {
	return &Builder{t: plan.Trace{
		Title: "synthetic",
		Calibration: plan.Calibration{
			PxPerMetre: [2]float64{pxPerMetre, pxPerMetre},
			Evidence:   "synthetic",
		},
		Thickness: plan.Thickness{Exterior: 0.2, Interior: 0.1, Glass: 0.05},
	}}
}

// Wall adds a wall of kind from from to to, with its openings.
func (b *Builder) Wall(kind string, from, to [2]float64, openings ...plan.Opening) *Builder {
	b.t.Walls = append(b.t.Walls, plan.Wall{
		ID:       fmt.Sprintf("w%d", len(b.t.Walls)+1),
		Kind:     kind,
		From:     from,
		To:       to,
		Openings: openings,
	})
	return b
}

// Box adds four exterior walls around the rectangle (x0, y0)–(x1, y1).
func (b *Builder) Box(x0, y0, x1, y1 float64) *Builder {
	return b.Wall("exterior", [2]float64{x0, y0}, [2]float64{x1, y0}).
		Wall("exterior", [2]float64{x1, y0}, [2]float64{x1, y1}).
		Wall("exterior", [2]float64{x1, y1}, [2]float64{x0, y1}).
		Wall("exterior", [2]float64{x0, y1}, [2]float64{x0, y0})
}

// Room adds a named room with its polygon; the label goes at its centroid.
func (b *Builder) Room(name string, polygon ...[2]float64) *Builder {
	b.t.Rooms = append(b.t.Rooms, plan.Room{
		ID:      fmt.Sprintf("r%d", len(b.t.Rooms)+1),
		Name:    name,
		Zone:    "work",
		Floor:   "carpet",
		Polygon: polygon,
	})
	return b
}

// Item adds a piece of furniture.
func (b *Builder) Item(it plan.Item) *Builder {
	b.t.Furniture = append(b.t.Furniture, it)
	return b
}

// Trace is the trace built so far.
func (b *Builder) Trace() *plan.Trace {
	t := b.t
	return &t
}

// Door is a single door from a to b on its wall's dominant axis, opening to
// swing (N, S, E or W) and hinged at "a" or "b".
func Door(a, b float64, swing, hinge string) plan.Opening {
	return plan.Opening{A: a, B: b, Kind: "door", Swing: swing, Hinge: hinge}
}

// DoubleDoor is a pair of leaves from a to b opening to swing.
func DoubleDoor(a, b float64, swing string) plan.Opening {
	return plan.Opening{A: a, B: b, Kind: "double-door", Swing: swing}
}

// Window is a glazed opening from a to b.
func Window(a, b float64) plan.Opening { return plan.Opening{A: a, B: b, Kind: "window"} }

// Gap is an opening with no leaf from a to b.
func Gap(a, b float64) plan.Opening { return plan.Opening{A: a, B: b, Kind: "opening"} }
