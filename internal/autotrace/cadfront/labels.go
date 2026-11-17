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

// Room outlines: closed polylines on area layers (A-AREA, ROOM, SPACE)
// would say where rooms are outright, but floorgraph.Input carries no
// polygons; floorgraph finds rooms from the walls, named by the labels
// below. An area layer's lines are never walls, so its outlines don't turn
// into walls meanwhile, except in a drawing with no walls at all, where
// they are the only evidence (see areaRooms).

// findLabels reads every piece of text that isn't part of a door, window,
// piece of furniture or dimension. MTEXT gives its first line, the name in
// a room tag whose later lines are its number or area. Which of it may name
// a room, and how, is labelKind's to say.
func (r *reader) findLabels() {
	named := slices.ContainsFunc(r.leaves, func(l leaf) bool {
		return !r.consumed(&l) && l.info.text == textName &&
			len(cad.TextLines(l.e, l.m)) > 0
	})
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
		// An area written out is never a room's name, and nor is what a
		// workplace is called.
		if _, ok := statedArea(text); ok || r.deskLabel(box.Center()) {
			continue
		}
		kind, ok := labelKind(l.info.text, text, named)
		if !ok {
			continue
		}
		l.labelled = true
		r.in.Labels = append(r.in.Labels, floorgraph.Label{
			At: r.toPx(box.Center()), Text: text, Height: lines[0].Height * r.px, Kind: kind,
		})
	}
}

// Furniture sizes outside this range, in metres, are a block drawn at
// the wrong scale or a symbol that isn't furniture.
const minItem, maxItem = 0.1, 12.0

func ptr[T any](v T) *T { return &v }

// round3 keeps sizes to the millimetre: finer is noise from the block's
// curves, and it keeps traces readable.
func round3(v float64) float64 { return math.Round(v*1000) / 1000 }

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
