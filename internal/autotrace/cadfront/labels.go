package cadfront

import (
	"cmp"
	"math"
	"slices"
	"strings"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/floorgraph"
	"github.com/weaveplatform/spaceweave/internal/plan"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// Room outlines: closed polylines on area layers (A-AREA, ROOM, SPACE,
// A-FLOR-IDEN) would say where rooms are outright, but floorgraph.Input
// carries no polygons; floorgraph finds rooms from the walls, named by the
// labels below. Area layers are excluded from walls (notWallWords), so
// their outlines don't turn into walls meanwhile.

// findLabels reads every piece of text that isn't part of a door, window,
// piece of furniture or dimension. MTEXT gives its first line, the name in
// a room tag whose later lines are its number or area.
func (r *reader) findLabels() {
	for i := range r.leaves {
		l := &r.leaves[i]
		if r.consumed(l) {
			continue
		}
		lines := cad.TextLines(l.e, l.m)
		if len(lines) == 0 {
			continue
		}
		// TextLines leaves out blank lines, so the first has words.
		text := strings.TrimSpace(lines[0].Content)
		var box vector.Rect
		for _, t := range lines {
			box = box.Union(t.Bounds())
		}
		l.used = true
		r.in.Labels = append(r.in.Labels, floorgraph.Label{
			At: r.toPx(box.Center()), Text: text, Height: lines[0].Height * r.px,
		})
	}
}

// Furniture sizes outside this range, in metres, are a block drawn at
// the wrong scale or a symbol that isn't furniture.
const minItem, maxItem = 0.1, 12.0

// findFurniture places furniture blocks, by the type their name gives and
// the size they are drawn. It needs real sizes, so it does nothing when
// the drawing's units are unknown.
func (r *reader) findFurniture() {
	var tops []*node
	for _, n := range r.nodes {
		if n.top == n && n.role == roleFurniture {
			tops = append(tops, n)
		}
	}
	if len(tops) == 0 {
		return
	}
	if !r.known {
		r.note(
			"%d furniture blocks were left out: without the drawing's units their sizes are unknown.",
			len(tops),
		)
		return
	}
	unknown, odd := 0, 0
	for _, n := range tops {
		if n.furn == "" {
			unknown++
			continue
		}
		it, ok := r.item(n)
		if !ok {
			odd++
			continue
		}
		r.use(n)
		r.in.Furniture = append(r.in.Furniture, it)
	}
	if unknown > 0 {
		r.note(
			"%d furniture blocks had names that don't say what they are and were left out.",
			unknown,
		)
	}
	if odd > 0 {
		r.note("%d furniture blocks were left out for implausible sizes.", odd)
	}
}

func ptr[T any](v T) *T { return &v }

// round3 keeps sizes to the millimetre: finer is noise from the block's
// curves, and it keeps traces readable.
func round3(v float64) float64 { return math.Round(v*1000) / 1000 }

// item makes a furniture item from a block reference: its extent in the
// block's own axes gives width (along x) and depth, the insert's turn
// gives rot, and a seat or fitting faces the block's -y, the way symbol
// libraries draw chairs, toilets and basins (back to the wall at the top).
func (r *reader) item(n *node) (plan.Item, bool) {
	inv, ok := inverse(n.m)
	if !ok {
		return plan.Item{}, false
	}
	var box vector.Rect
	round := false
	for _, li := range r.subtree(n, nil) {
		if _, c := r.leaves[li].e.(*cad.Circle); c {
			round = true
		}
		for _, p := range r.points(li) {
			box = box.Extend(inv.Apply(p))
		}
	}
	if box.Empty() {
		return plan.Item{}, false
	}
	xAxis, yAxis := point{X: n.m.A, Y: n.m.B}, point{X: n.m.C, Y: n.m.D}
	w := round3(box.Size().X * xAxis.Len() * r.unit)
	d := round3(box.Size().Y * yAxis.Len() * r.unit)
	if w < minItem || d < minItem || w > maxItem || d > maxItem {
		return plan.Item{}, false
	}
	at := r.toPx(n.m.Apply(box.Center()))
	it := plan.Item{Type: n.furn, X: at.X, Y: at.Y}
	if it.Type == "table" {
		it.Type = "desk"
		if round && math.Abs(w-d) <= 0.15*math.Max(w, d) {
			it.Type = "roundTable"
		}
	}
	spec := plan.Types[it.Type]
	if spec.Oriented == "" {
		// Without an angle field the item is laid out along the image's
		// axes, so a block turned nearer upright than flat swaps sides.
		if a := imageAngle(r.o.View, xAxis); math.Abs(math.Sin(a*math.Pi/180)) > math.Sqrt2/2 {
			w, d = d, w
		}
	}
	for _, f := range spec.Required {
		switch f {
		case "w":
			it.W = ptr(w)
		case "d":
			it.D = ptr(d)
		case "r":
			it.R = ptr(round3((w + d) / 4))
		}
	}
	switch it.Type {
	case "chair":
		it.Size = ptr(round3((w + d) / 2))
	case "plant":
		it.R = ptr(round3((w + d) / 4))
	}
	switch spec.Oriented {
	case "rot":
		it.Rot = ptr(imageAngle(r.o.View, xAxis))
	case "facing":
		it.Facing = ptr(imageAngle(r.o.View, yAxis.Mul(-1)))
	}
	return it, true
}

func comparePoints(a, b point) int {
	return cmp.Or(cmp.Compare(a.X, b.X), cmp.Compare(a.Y, b.Y))
}

// sortOutput puts everything in a fixed order, and each wall's ends too,
// so equal drawings give equal inputs whatever order their entities were
// in.
func (r *reader) sortOutput() {
	for i := range r.in.Walls {
		if w := &r.in.Walls[i]; comparePoints(w.B, w.A) < 0 {
			w.A, w.B = w.B, w.A
		}
	}
	slices.SortStableFunc(r.in.Walls, func(a, b floorgraph.Segment) int {
		return cmp.Or(
			comparePoints(a.A, b.A),
			comparePoints(a.B, b.B),
			cmp.Compare(a.Thickness, b.Thickness),
		)
	})
	slices.SortStableFunc(r.in.Openings, func(a, b floorgraph.Opening) int {
		return cmp.Or(comparePoints(a.A, b.A), comparePoints(a.B, b.B), cmp.Compare(a.Kind, b.Kind))
	})
	slices.SortStableFunc(r.in.Labels, func(a, b floorgraph.Label) int {
		return cmp.Or(
			cmp.Compare(a.At.Y, b.At.Y),
			cmp.Compare(a.At.X, b.At.X),
			cmp.Compare(a.Text, b.Text),
		)
	})
	slices.SortStableFunc(r.in.Furniture, func(a, b plan.Item) int {
		return cmp.Or(cmp.Compare(a.Y, b.Y), cmp.Compare(a.X, b.X), cmp.Compare(a.Type, b.Type))
	})
}
