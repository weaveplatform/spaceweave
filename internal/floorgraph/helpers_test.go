package floorgraph

import (
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/plan"
)

func P(x, y float64) Point { return Point{X: x, Y: y} }

// wall is a test segment with a thickness of 6 px and full weight.
func wall(x0, y0, x1, y1 float64) Segment {
	return Segment{A: P(x0, y0), B: P(x1, y1), Thickness: 6, Weight: 1}
}

// box is the four walls of an axis-aligned rectangle.
func box(x0, y0, x1, y1 float64) []Segment {
	return []Segment{
		wall(x0, y0, x1, y0),
		wall(x1, y0, x1, y1),
		wall(x1, y1, x0, y1),
		wall(x0, y1, x0, y0),
	}
}

// scale50 is an exact 50 px/m scale.
var scale50 = Scale{PxPerMetre: [2]float64{50, 50}, Exact: true, Evidence: "test"}

// mustBuild builds and holds the result to everything a published plan
// must pass: plan.Check, plan.Convert and the site validator's geometry.
func mustBuild(t *testing.T, in Input) *Result {
	t.Helper()
	r, err := Build(in)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	checkResult(t, r)
	return r
}

func checkResult(t *testing.T, r *Result) {
	t.Helper()
	if p := plan.Check(r.Trace); len(p) > 0 {
		t.Fatalf("plan.Check: %v", p)
	}
	p, err := plan.Convert(
		r.Trace,
		plan.Meta{URL: "x.png", Credit: "test", SizePx: [2]int{100, 100}},
	)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if errs := siteCheck(p); len(errs) > 0 {
		t.Fatalf("site validator: %v", errs)
	}
	if !(r.Confidence >= 0 && r.Confidence <= 1) || r.Summary == "" {
		t.Fatalf("confidence %v, summary %q", r.Confidence, r.Summary)
	}
}

// siteCheck is the geometry of web/floorplan-iso/test/invariants.mjs's
// checkPlan, in Go so the fuzz test can run it: walls over 5 cm with their
// ends in the outline (0.2 m slack), openings on their wall, rooms and voids
// over 0.5 m² with their corners in the outline (0.15 m slack), a known kind
// for every room, no room labelled inside a void, and an exterior wall; and
// no room or zone sharing more than overlapSlack with another, with a void
// or with the world outside the outline.
func siteCheck(p *plan.Plan) []string {
	var errs []string
	outline := p.Outline
	if outline == nil {
		outline = [][2]float64{{0, 0}, {p.Width, 0}, {p.Width, p.Depth}, {0, p.Depth}}
	}
	poly := points(outline)
	in := func(o [2]float64, slack float64) bool {
		for _, i := range []float64{0, -1, 1} {
			for _, j := range []float64{0, -1, 1} {
				if inside(P(o[0]+i*slack, o[1]+j*slack), poly) {
					return true
				}
			}
		}
		return false
	}
	ext := false
	for _, w := range p.Walls {
		ext = ext || w.Kind == "exterior"
		dx, dy := w.To[0]-w.From[0], w.To[1]-w.From[1]
		if !(math.Hypot(dx, dy) > 0.05) {
			errs = append(errs, "wall "+w.ID+" under 5 cm")
		}
		ax := 0
		if math.Abs(dy) > math.Abs(dx) {
			ax = 1
		}
		lo, hi := math.Min(w.From[ax], w.To[ax]), math.Max(w.From[ax], w.To[ax])
		for _, o := range w.Openings {
			if !(o.A < o.B) || !(o.A >= lo-1e-6 && o.B <= hi+1e-6) {
				errs = append(errs, "wall "+w.ID+": opening off the wall")
			}
		}
		if !in(w.From, 0.2) || !in(w.To, 0.2) {
			errs = append(errs, "wall "+w.ID+" outside the outline")
		}
	}
	if !ext {
		errs = append(errs, "no exterior walls")
	}
	var voids [][][2]float64
	for _, v := range p.Voids {
		for _, o := range v {
			if !in(o, 0.15) {
				errs = append(errs, "void outside the outline")
			}
		}
		if !(len(v) >= 3 && polyArea(v) >= 0.5) {
			errs = append(errs, "void under 0.5 m²")
		}
		voids = append(voids, v)
	}
	var spaces []plan.MRoom
	for _, r := range p.Rooms {
		if r.Kind != "" && !slices.Contains(plan.Kinds, r.Kind) {
			errs = append(errs, "room "+r.ID+" kind "+r.Kind)
		}
		if !(len(r.Polygon) >= 3 && polyArea(r.Polygon) > 0.5) {
			errs = append(errs, "room "+r.ID+" under 0.5 m²")
			continue
		}
		for _, o := range r.Polygon {
			if !in(o, 0.15) {
				errs = append(errs, "room "+r.ID+" outside the outline")
				break
			}
		}
		for _, v := range voids {
			if r.Label != nil && inside(P(r.Label[0], r.Label[1]), points(v)) {
				errs = append(errs, "room "+r.ID+" labelled in a void")
			}
		}
		spaces = append(spaces, r)
	}
	for i, r := range spaces {
		a := polyArea(r.Polygon)
		if out := a - overlapArea(r.Polygon, outline); out > overlapSlack(a, polyArea(outline)) {
			errs = append(errs, fmt.Sprintf("room %s has %.2f m² outside the outline", r.ID, out))
		}
		for _, v := range voids {
			if o := overlapArea(r.Polygon, v); o > overlapSlack(a, polyArea(v)) {
				errs = append(errs, fmt.Sprintf("room %s covers %.2f m² of a void", r.ID, o))
			}
		}
		for _, q := range spaces[i+1:] {
			if o := overlapArea(r.Polygon, q.Polygon); o > overlapSlack(a, polyArea(q.Polygon)) {
				errs = append(errs, fmt.Sprintf("rooms %s and %s overlap by %.2f m²", r.ID, q.ID, o))
			}
		}
	}
	return errs
}

// overlapSlack is how much floor two polygons may share before invariants.mjs
// calls it an overlap: its OVERLAP_SLACK, a tracing slip.
func overlapSlack(a, b float64) float64 { return math.Max(0.25, 0.005*math.Min(a, b)) }

func points(poly [][2]float64) []Point {
	out := make([]Point, len(poly))
	for i, o := range poly {
		out[i] = P(o[0], o[1])
	}
	return out
}

func polyArea(poly [][2]float64) float64 { return math.Abs(signedArea(points(poly))) }

func roomNames(r *Result) []string {
	var out []string
	for _, rm := range r.Trace.Rooms {
		out = append(out, rm.Name)
	}
	return out
}

func openingKinds(r *Result) map[string]int {
	out := map[string]int{}
	for _, w := range r.Trace.Walls {
		for _, o := range w.Openings {
			out[o.Kind]++
		}
	}
	return out
}

func kindCount(r *Result, kind string) int {
	n := 0
	for _, w := range r.Trace.Walls {
		if w.Kind == kind {
			n++
		}
	}
	return n
}
