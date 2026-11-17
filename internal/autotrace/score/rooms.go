package score

import (
	"math"
	"slices"

	"github.com/weaveplatform/spaceweave/internal/plan"
)

// MinIoU is the intersection over union two floor areas need to be the
// same room. Room polygons follow wall centrelines, so a trace whose walls
// are within a few pixels of the hand trace's still loses a sliver along
// every side: 0.9 allows 2 px on each side of a 1.5 m WC at 40 px/m.
const MinIoU = 0.9

// AreaMatch compares the rooms of two traces by the floor they cover, not by
// name: an automatic trace without OCR has no names to match.
type AreaMatch struct {
	Want, Got int
	// Exact is how many of want's rooms have one of got's with an IoU of at
	// least MinIoU: the same room.
	Exact int
	// Covered adds to Exact the want rooms that got covers as a group: a got
	// room that is the union of several want rooms (the hand trace split an
	// open area where the drawing has no wall, so the tracer can't), or a
	// want room that is the union of several got rooms (the tracer split
	// it), each group at MinIoU.
	Covered int
	// Named is how many Exact matches carry the same name.
	Named int
	// AreaRecall is the share of want's floor area inside some got room;
	// AreaPrecision the share of got's inside some want room.
	AreaRecall, AreaPrecision float64
}

// RoomAreas matches got's rooms to want's by area.
func RoomAreas(got, want *plan.Trace) AreaMatch {
	m := AreaMatch{Want: len(want.Rooms), Got: len(got.Rooms)}
	inter, aw, ag := overlaps(want.Rooms, got.Rooms)
	iou := func(i, a, b float64) float64 {
		if u := a + b - i; u > 0 {
			return i / u
		}
		return 0
	}
	covered := make([]bool, len(want.Rooms))
	for i := range want.Rooms {
		for j := range got.Rooms {
			if iou(inter[i][j], aw[i], ag[j]) >= MinIoU {
				m.Exact++
				covered[i] = true
				if normName(want.Rooms[i].Name) == normName(got.Rooms[j].Name) {
					m.Named++
				}
				break
			}
		}
	}
	// Merges: a got room holding most of each of several want rooms.
	for j := range got.Rooms {
		var members []int
		var in, area float64
		for i := range want.Rooms {
			if aw[i] > 0 && 2*inter[i][j] >= aw[i] {
				members = append(members, i)
				in += inter[i][j]
				area += aw[i]
			}
		}
		if len(members) > 1 && iou(in, area, ag[j]) >= MinIoU {
			for _, i := range members {
				covered[i] = true
			}
		}
	}
	// Splits: a want room holding most of each of several got rooms.
	for i := range want.Rooms {
		var n int
		var in, area float64
		for j := range got.Rooms {
			if ag[j] > 0 && 2*inter[i][j] >= ag[j] {
				n++
				in += inter[i][j]
				area += ag[j]
			}
		}
		if n > 1 && iou(in, aw[i], area) >= MinIoU {
			covered[i] = true
		}
	}
	for _, c := range covered {
		if c {
			m.Covered++
		}
	}
	var both, sw, sg float64
	for i := range aw {
		sw += aw[i]
		for j := range ag {
			both += inter[i][j]
		}
	}
	for _, a := range ag {
		sg += a
	}
	m.AreaRecall, m.AreaPrecision = 1, 1
	if sw > 0 {
		m.AreaRecall = math.Min(1, both/sw)
	}
	if sg > 0 {
		m.AreaPrecision = math.Min(1, both/sg)
	}
	return m
}

type span struct {
	x0, x1 float64
	room   int
}

// overlaps is the area each want room shares with each got room, and each
// room's own area, px². Polygons are cut into spans along every pixel row's
// middle and the spans intersected, which is exact to a pixel row and needs
// no polygon clipping.
func overlaps(want, got []plan.Room) (inter [][]float64, aw, ag []float64) {
	inter = make([][]float64, len(want))
	for i := range inter {
		inter[i] = make([]float64, len(got))
	}
	aw, ag = make([]float64, len(want)), make([]float64, len(got))
	y0, y1 := math.Inf(1), math.Inf(-1)
	for _, rs := range [2][]plan.Room{want, got} {
		for _, r := range rs {
			for _, p := range r.Polygon {
				y0, y1 = math.Min(y0, p[1]), math.Max(y1, p[1])
			}
		}
	}
	if math.IsInf(y0, 0) {
		return inter, aw, ag
	}
	for y := math.Floor(y0) + 0.5; y < y1; y++ {
		ws, gs := spans(want, y), spans(got, y)
		for _, s := range ws {
			aw[s.room] += s.x1 - s.x0
		}
		for _, s := range gs {
			ag[s.room] += s.x1 - s.x0
		}
		for _, a := range ws {
			for _, b := range gs {
				if d := math.Min(a.x1, b.x1) - math.Max(a.x0, b.x0); d > 0 {
					inter[a.room][b.room] += d
				}
			}
		}
	}
	return inter, aw, ag
}

// spans are the stretches of the row at y inside each room, by the even-odd
// rule.
func spans(rooms []plan.Room, y float64) []span {
	var out []span
	var xs []float64
	for ri, r := range rooms {
		xs = xs[:0]
		n := len(r.Polygon)
		for k := range n {
			p, q := r.Polygon[k], r.Polygon[(k+1)%n]
			if (p[1] <= y) != (q[1] <= y) {
				xs = append(xs, p[0]+(y-p[1])*(q[0]-p[0])/(q[1]-p[1]))
			}
		}
		slices.Sort(xs)
		for k := 0; k+1 < len(xs); k += 2 {
			out = append(out, span{xs[k], xs[k+1], ri})
		}
	}
	return out
}
