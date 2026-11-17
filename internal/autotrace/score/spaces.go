package score

import (
	"math"
	"slices"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"

	"github.com/weaveplatform/spaceweave/internal/plan"
)

// Spaces measures individual space identity. Group coverage remains a
// separate legacy measure: a merged room cannot stand for two correct rooms.
type Spaces struct {
	Want                int     `json:"want"`
	Got                 int     `json:"got"`
	Matched             int     `json:"matched"`
	Named               int     `json:"named"`
	NameWant            int     `json:"nameWant"`
	Kind                int     `json:"kind"`
	MeanIoU             float64 `json:"meanIoU"`
	RoomWant            int     `json:"roomWant"`
	RoomMatched         int     `json:"roomMatched"`
	ZoneWant            int     `json:"zoneWant"`
	ZoneMatched         int     `json:"zoneMatched"`
	Splits              int     `json:"splits"`
	Merges              int     `json:"merges"`
	MeanAreaError       float64 `json:"meanAreaError"`
	MeanBoundaryErrorPx float64 `json:"meanBoundaryErrorPx"`
}

// SpaceConflict identifies materially overlapping reference outlines. An
// enclosing gross outline and its smaller spaces cannot all be preserved
// unchanged in a disjoint output. Report these separately without relaxing
// the identity threshold or silently editing the reference polygons.
type SpaceConflict struct {
	First          int     `json:"first"`
	Second         int     `json:"second"`
	FirstFraction  float64 `json:"firstFraction"`
	SecondFraction float64 `json:"secondFraction"`
}

// SpaceConflicts reports intersections exceeding both one pixel squared
// and 1% of the smaller reference. Tiny drafting overlaps remain below
// this diagnostic's threshold; it is not a geometry-validity test.
func SpaceConflicts(reference *plan.Trace) []SpaceConflict {
	inter, areas, _ := overlaps(reference.Rooms, reference.Rooms)
	var out []SpaceConflict
	for i := range areas {
		for j := i + 1; j < len(areas); j++ {
			shared := inter[i][j]
			if shared <= math.Max(1, 0.01*math.Min(areas[i], areas[j])) {
				continue
			}
			out = append(out, SpaceConflict{
				First: i, Second: j,
				FirstFraction: shared / areas[i], SecondFraction: shared / areas[j],
			})
		}
	}
	return out
}

// SpaceIdentity matches at IoU >= 0.9, independently of name and kind.
// Augmenting paths maximize cardinality so a duplicate prediction cannot
// claim the same reference twice. Higher IoU breaks otherwise equal choices.
func SpaceIdentity(got, want *plan.Trace) Spaces {
	result := Spaces{Want: len(want.Rooms), Got: len(got.Rooms)}
	graph := spaceCandidates(got, want, &result)
	edges, ious, aw, ag := graph.edges, graph.ious, graph.aw, graph.ag
	owner := make([]int, len(ag))
	for j := range owner {
		owner[j] = -1
	}
	var match func(int, []bool) bool
	match = func(i int, seen []bool) bool {
		for _, j := range edges[i] {
			if seen[j] {
				continue
			}
			seen[j] = true
			if owner[j] < 0 || match(owner[j], seen) {
				owner[j] = i
				return true
			}
		}
		return false
	}
	for i := range aw {
		match(i, make([]bool, len(ag)))
	}
	for j, i := range owner {
		if i < 0 {
			continue
		}
		result.Matched++
		result.MeanIoU += ious[i][j]
		if aw[i] > 0 {
			result.MeanAreaError += math.Abs(ag[j]-aw[i]) / aw[i]
		}
		result.MeanBoundaryErrorPx += (boundaryError(want.Rooms[i].Polygon, got.Rooms[j].Polygon) + boundaryError(got.Rooms[j].Polygon, want.Rooms[i].Polygon)) / 2
		a, b := want.Rooms[i], got.Rooms[j]
		if normalized(a.Name) != "" && normalized(a.Name) == normalized(b.Name) &&
			(a.Number == "" || normalized(a.Number) == normalized(b.Number)) {
			result.Named++
		}
		kind := func(r plan.Room) string {
			if r.Kind == "" {
				return plan.KindRoom
			}
			return r.Kind
		}
		if kind(a) == kind(b) {
			result.Kind++
			if a.Kind == plan.KindZone {
				result.ZoneMatched++
			} else {
				result.RoomMatched++
			}
		}
	}
	if result.Matched > 0 {
		result.MeanIoU /= float64(result.Matched)
		result.MeanAreaError /= float64(result.Matched)
		result.MeanBoundaryErrorPx /= float64(result.Matched)
	}
	return result
}

type spaceGraph struct {
	edges  [][]int
	ious   [][]float64
	aw, ag []float64
}

func spaceCandidates(got, want *plan.Trace, result *Spaces) spaceGraph {
	inter, aw, ag := overlaps(want.Rooms, got.Rooms)
	edges := make([][]int, len(aw))
	ious := make([][]float64, len(aw))
	for i := range aw {
		if want.Rooms[i].Kind == plan.KindZone {
			result.ZoneWant++
		} else {
			result.RoomWant++
		}
		pieces := 0
		for j := range ag {
			if ag[j] > 0 && inter[i][j]/ag[j] > .5 {
				pieces++
			}
		}
		if pieces > 1 {
			result.Splits++
		}
		if normalized(want.Rooms[i].Name) != "" {
			result.NameWant++
		}
		ious[i] = make([]float64, len(ag))
		for j := range ag {
			if union := aw[i] + ag[j] - inter[i][j]; union > 0 {
				ious[i][j] = inter[i][j] / union
			}
			if ious[i][j] >= MinIoU {
				edges[i] = append(edges[i], j)
			}
		}
		slices.SortStableFunc(edges[i], func(a, b int) int {
			if ious[i][a] > ious[i][b] {
				return -1
			}
			if ious[i][a] < ious[i][b] {
				return 1
			}
			return a - b
		})
	}
	for j := range ag {
		parts := 0
		for i := range aw {
			if aw[i] > 0 && inter[i][j]/aw[i] > .5 {
				parts++
			}
		}
		if parts > 1 {
			result.Merges++
		}
	}
	return spaceGraph{edges: edges, ious: ious, aw: aw, ag: ag}
}

func normalized(s string) string {
	return strings.Join(strings.Fields(cases.Fold().String(norm.NFKC.String(s))), " ")
}

// Sample vertices and edge midpoints in both directions; this is a
// boundary diagnostic in source pixels, not an exact Hausdorff distance.
func boundaryError(a, b [][2]float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	sum := 0.0
	for i, p := range a {
		q := a[(i+1)%len(a)]
		for _, f := range []float64{0, 0.5} {
			x, y := p[0]+f*(q[0]-p[0]), p[1]+f*(q[1]-p[1])
			best := math.Inf(1)
			for j, c := range b {
				d := b[(j+1)%len(b)]
				dx, dy := d[0]-c[0], d[1]-c[1]
				t := 0.0
				if ll := dx*dx + dy*dy; ll > 0 {
					t = math.Max(0, math.Min(1, ((x-c[0])*dx+(y-c[1])*dy)/ll))
				}
				best = math.Min(best, math.Hypot(x-c[0]-t*dx, y-c[1]-t*dy))
			}
			sum += best
		}
	}
	return sum / float64(2*len(a))
}
