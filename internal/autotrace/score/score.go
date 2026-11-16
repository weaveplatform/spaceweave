// Package score measures an automatic trace against a hand trace of the same
// image, in source pixels. It is the Go side of
// web/floorplan-iso/tools/compare.mjs, which scores in metres after
// conversion; scoring in pixels keeps a calibration error out of the
// geometry numbers, which is what a tracer's front end is judged on.
package score

import (
	"math"
	"strings"
	"unicode"

	"github.com/weaveplatform/spaceweave/internal/plan"
)

// step is the sampling interval along a wall, px. Half the usual 3–4 px
// tolerance keeps a short miss from falling between two samples.
const step = 2.0

// Walls is the share of want's wall length lying within tolPx of some wall
// in got (recall), and of got's within tolPx of some wall in want
// (precision). Openings count as wall: a centreline runs through them.
func Walls(got, want *plan.Trace, tolPx float64) (recall, precision float64) {
	g, w := segments(got), segments(want)
	return Covered(w, g, tolPx), Covered(g, w, tolPx)
}

// Segment is a straight line between two points in pixels.
type Segment [2][2]float64

func segments(t *plan.Trace) []Segment {
	out := make([]Segment, 0, len(t.Walls))
	for _, w := range t.Walls {
		out = append(out, Segment{w.From, w.To})
	}
	return out
}

// Covered is the share of the length of segs within tol of some segment in
// other; 1 when segs has no length, since nothing was missed.
func Covered(segs, other []Segment, tol float64) float64 {
	// Each sample is tested against every segment; a box test first keeps
	// a 200-wall plan at 15,000 samples cheap enough to score in tests.
	boxes := make([][4]float64, len(other))
	for i, o := range other {
		boxes[i] = [4]float64{
			math.Min(o[0][0], o[1][0]) - tol, math.Min(o[0][1], o[1][1]) - tol,
			math.Max(o[0][0], o[1][0]) + tol, math.Max(o[0][1], o[1][1]) + tol,
		}
	}
	isNear := func(p [2]float64) bool {
		for i, b := range boxes {
			if p[0] >= b[0] && p[1] >= b[1] && p[0] <= b[2] && p[1] <= b[3] &&
				dist(p, other[i]) <= tol {
				return true
			}
		}
		return false
	}
	var total, near float64
	for _, s := range segs {
		l := math.Hypot(s[1][0]-s[0][0], s[1][1]-s[0][1])
		n := max(1, int(math.Ceil(l/step)))
		for i := range n {
			t := (float64(i) + 0.5) / float64(n)
			p := [2]float64{s[0][0] + t*(s[1][0]-s[0][0]), s[0][1] + t*(s[1][1]-s[0][1])}
			total += l / float64(n)
			if isNear(p) {
				near += l / float64(n)
			}
		}
	}
	if total == 0 {
		return 1
	}
	return near / total
}

func dist(p [2]float64, s Segment) float64 {
	dx, dy := s[1][0]-s[0][0], s[1][1]-s[0][1]
	l2 := dx*dx + dy*dy
	t := 0.0
	if l2 > 0 {
		t = math.Max(0, math.Min(1, ((p[0]-s[0][0])*dx+(p[1]-s[0][1])*dy)/l2))
	}
	return math.Hypot(p[0]-(s[0][0]+t*dx), p[1]-(s[0][1]+t*dy))
}

// RoomMatch is how many of want's rooms got found by name.
type RoomMatch struct {
	Matched, Want, Got int
	// Missing are want's names got lacks; Extra are got's names left over.
	Missing, Extra []string
}

// Rooms matches rooms by name, ignoring case and punctuation, each got room
// standing for at most one want room so two "Office"s need two.
func Rooms(got, want *plan.Trace) RoomMatch {
	pool := make([]string, 0, len(got.Rooms))
	for _, r := range got.Rooms {
		pool = append(pool, normName(r.Name))
	}
	extra := make([]string, 0, len(got.Rooms))
	m := RoomMatch{Want: len(want.Rooms), Got: len(got.Rooms)}
	used := make([]bool, len(pool))
	for _, r := range want.Rooms {
		n := normName(r.Name)
		i := -1
		for j, p := range pool {
			if !used[j] && p == n {
				i = j
				break
			}
		}
		if i < 0 {
			m.Missing = append(m.Missing, r.Name)
			continue
		}
		used[i] = true
		m.Matched++
	}
	for i, r := range got.Rooms {
		if !used[i] {
			extra = append(extra, r.Name)
		}
	}
	if len(extra) > 0 {
		m.Extra = extra
	}
	return m
}

func normName(s string) string {
	f := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return (r < 'a' || r > 'z') && !unicode.IsDigit(r)
	})
	return strings.Join(f, " ")
}

// OpeningMatch counts one kind of opening.
type OpeningMatch struct {
	// Matched is how many of want's openings of this kind have one of got's,
	// of the same kind, whose middle is within the tolerance on a parallel
	// wall.
	Matched, Want, Got int
	// Located is how many of want's openings of this kind have one of got's
	// of any kind there: found, if not always told apart.
	Located int
	// Swung is how many matched doors and double doors open to the same
	// side; Hinged how many matched single doors hang from the same end.
	Swung, Hinged int
}

// maxSkew is how far, in radians, two walls may disagree in direction and
// still be the same wall line; a door on a crossing wall is a different door.
const maxSkew = 15 * math.Pi / 180

// Openings counts and matches openings per kind (door, double-door, window,
// opening), each got opening matching at most one want opening.
func Openings(got, want *plan.Trace, tolPx float64) map[string]OpeningMatch {
	g, w := placed(got), placed(want)
	out := map[string]OpeningMatch{}
	for _, o := range g {
		m := out[o.kind]
		m.Got++
		out[o.kind] = m
	}
	used, seen := make([]bool, len(g)), make([]bool, len(g))
	for _, o := range w {
		m := out[o.kind]
		m.Want++
		if best := nearest(g, used, o, tolPx, true); best >= 0 {
			used[best] = true
			m.Matched++
			c := g[best]
			if c.swing == o.swing && o.swing != "" {
				m.Swung++
			}
			if o.kind == "door" && c.hinge == o.hinge {
				m.Hinged++
			}
		}
		if best := nearest(g, seen, o, tolPx, false); best >= 0 {
			seen[best] = true
			m.Located++
		}
		out[o.kind] = m
	}
	return out
}

// nearest is the unused opening in g closest to o within tol on a parallel
// wall, of o's kind when sameKind; -1 when there is none.
func nearest(g []placedOpening, used []bool, o placedOpening, tol float64, sameKind bool) int {
	best, bestD := -1, tol
	for i, c := range g {
		if used[i] || (sameKind && c.kind != o.kind) || skew(c.dir, o.dir) > maxSkew {
			continue
		}
		if d := math.Hypot(c.mid[0]-o.mid[0], c.mid[1]-o.mid[1]); d <= bestD {
			best, bestD = i, d
		}
	}
	return best
}

type placedOpening struct {
	kind, swing, hinge string
	mid                [2]float64
	dir                float64 // the wall's direction, radians
}

func placed(t *plan.Trace) []placedOpening {
	var out []placedOpening
	for _, w := range t.Walls {
		dx, dy := w.To[0]-w.From[0], w.To[1]-w.From[1]
		if dx == 0 && dy == 0 {
			continue
		}
		for _, o := range w.Openings {
			hinge := o.Hinge
			if o.Kind == "door" && hinge == "" {
				hinge = "a" // the trace format's default
			}
			out = append(out, placedOpening{
				kind:  o.Kind,
				swing: o.Swing,
				hinge: hinge,
				mid:   OnWall(w, (o.A+o.B)/2),
				dir:   math.Atan2(dy, dx),
			})
		}
	}
	return out
}

// OnWall is the point on w's centreline at v along its dominant axis, the
// convention plan.Opening's A and B use.
func OnWall(w plan.Wall, v float64) [2]float64 {
	dx, dy := w.To[0]-w.From[0], w.To[1]-w.From[1]
	if math.Abs(dx) >= math.Abs(dy) {
		return [2]float64{v, w.From[1] + (v-w.From[0])*dy/dx}
	}
	return [2]float64{w.From[0] + (v-w.From[1])*dx/dy, v}
}

// skew is the angle between two undirected lines.
func skew(a, b float64) float64 {
	d := math.Mod(math.Abs(a-b), math.Pi)
	return math.Min(d, math.Pi-d)
}
