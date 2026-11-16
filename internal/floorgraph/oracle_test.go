package floorgraph

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/plan"
)

func loadTrace(t *testing.T, name string) *plan.Trace {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "plan", "testdata", name+".trace.json"))
	if err != nil {
		t.Fatal(err)
	}
	var tr plan.Trace
	if err := json.Unmarshal(b, &tr); err != nil {
		t.Fatal(err)
	}
	return &tr
}

func pp(v [2]float64) Point { return P(v[0], v[1]) }

// evidence is what a perfect front end would find in the image a hand
// trace was made from: each wall's centreline broken at its openings, each
// opening with its leaf on its swing side, each room's name at its label.
// Every other door has its ends given b-first, so the hinge has to be mapped
// back.
func evidence(tr *plan.Trace) Input {
	sx, sy := tr.Calibration.PxPerMetre[0], tr.Calibration.PxPerMetre[1]
	def := map[string]float64{
		"exterior": tr.Thickness.Exterior,
		"interior": tr.Thickness.Interior,
		"glass":    tr.Thickness.Glass,
	}
	in := Input{
		Title: tr.Title,
		Scale: Scale{
			PxPerMetre: tr.Calibration.PxPerMetre,
			Exact:      true,
			Evidence:   tr.Calibration.Evidence,
		},
		Furniture: tr.Furniture,
		Coverage:  -1,
	}
	flip := false
	for _, w := range tr.Walls {
		from, to := pp(w.From), pp(w.To)
		ax := axis(from, to)
		d := unit(sub(to, from))
		tm := w.Thickness
		if tm == 0 {
			tm = def[w.Kind]
		}
		tpx := tm / metresPerPx(P(-d.Y, d.X), sx, sy)
		at := func(c float64) float64 { return (c - coord(from, ax)) / (coord(to, ax) - coord(from, ax)) }
		piece := func(t0, t1 float64) {
			in.Walls = append(in.Walls, Segment{
				A: lerp(
					from,
					to,
					t0,
				),
				B:         lerp(from, to, t1),
				Thickness: tpx,
				Glass:     w.Kind == "glass",
				Weight:    1,
			})
		}
		ops := slices.Clone(w.Openings)
		slices.SortFunc(
			ops,
			func(x, y plan.Opening) int { return cmpFloat(math.Min(at(x.A), at(x.B)), math.Min(at(y.A), at(y.B))) },
		)
		cur := 0.0
		for _, o := range ops {
			ta, tb := at(o.A), at(o.B)
			t0, t1 := math.Min(ta, tb), math.Max(ta, tb)
			if t0 > cur {
				piece(cur, t0)
			}
			cur = math.Max(cur, t1)
			op := Opening{
				A:      lerp(from, to, ta),
				B:      lerp(from, to, tb),
				Kind:   o.Kind,
				HingeA: o.Hinge == "a",
				Weight: 1,
			}
			if o.Swing != "" {
				m, half := mid(op.A, op.B), dist(op.A, op.B)/2
				off := map[string]Point{"N": P(0, -half), "S": P(0, half), "E": P(half, 0), "W": P(-half, 0)}[o.Swing]
				op.Leaf, op.Leafed = P(m.X+off.X, m.Y+off.Y), true
			}
			if o.Kind == Door {
				if flip {
					op.A, op.B, op.HingeA = op.B, op.A, !op.HingeA
				}
				flip = !flip
			}
			in.Openings = append(in.Openings, op)
		}
		if cur < 1 {
			piece(cur, 1)
		}
	}
	for _, r := range tr.Rooms {
		at := P(0, 0)
		if r.Label != nil {
			at = pp(*r.Label)
		} else {
			poly := make([]Point, len(r.Polygon))
			for i, v := range r.Polygon {
				poly[i] = pp(v)
			}
			at = centroid(poly)
		}
		in.Labels = append(in.Labels, Label{At: at, Text: r.Name, Height: 12})
	}
	return in
}

// coverage is the share of the length of walls a lying within tol px of
// some wall in b, sampled every px.
func coverage(a, b []plan.Wall, tol float64) float64 {
	total, hit := 0.0, 0.0
	for _, w := range a {
		from, to := pp(w.From), pp(w.To)
		n := max(int(dist(from, to)), 1)
		for i := 0; i <= n; i++ {
			p := lerp(from, to, float64(i)/float64(n))
			total++
			if slices.ContainsFunc(
				b,
				func(o plan.Wall) bool { return segDist(p, pp(o.From), pp(o.To)) <= tol },
			) {
				hit++
			}
		}
	}
	return hit / total
}

type oracleStats struct {
	recall, precision    float64
	rooms, wantRooms     int
	named, wantNamed     int
	ops, wantOps         map[string]int
	kindAgree, swingGood float64
}

func oracle(t *testing.T, name string) (*Result, oracleStats) {
	t.Helper()
	tr := loadTrace(t, name)
	r := mustBuild(t, evidence(tr))
	got := r.Trace
	s := oracleStats{
		recall:    coverage(tr.Walls, got.Walls, 2),
		precision: coverage(got.Walls, tr.Walls, 2),
		rooms:     len(got.Rooms), wantRooms: len(tr.Rooms),
		ops: openingKinds(r), wantOps: map[string]int{},
	}
	names := roomNames(r)
	want := map[string]bool{}
	for _, rm := range tr.Rooms {
		want[rm.Name] = true
	}
	s.wantNamed = len(want)
	for n := range want {
		if slices.Contains(names, n) {
			s.named++
		}
	}
	for _, w := range tr.Walls {
		for _, o := range w.Openings {
			s.wantOps[o.Kind]++
		}
	}
	// Kind agreement: of each hand wall's length, the share on an output
	// wall of the same kind.
	agree, total := 0.0, 0.0
	for _, w := range tr.Walls {
		var same []plan.Wall
		for _, o := range got.Walls {
			if o.Kind == w.Kind {
				same = append(same, o)
			}
		}
		l := dist(pp(w.From), pp(w.To))
		total += l
		agree += l * coverage([]plan.Wall{w}, same, 2)
	}
	s.kindAgree = agree / total
	s.swingGood = swingAgreement(tr, got)
	return r, s
}

// swingAgreement is the share of hand-traced doors with an output door at the
// same place (within 2 px at both ends) swinging the same way, hinged on the
// same side.
func swingAgreement(want, got *plan.Trace) float64 {
	type door struct {
		a, b         Point
		swing, hinge string
	}
	collect := func(tr *plan.Trace) []door {
		var out []door
		for _, w := range tr.Walls {
			from, to := pp(w.From), pp(w.To)
			ax := axis(from, to)
			span := coord(to, ax) - coord(from, ax)
			for _, o := range w.Openings {
				if o.Kind != Door && o.Kind != DoubleDoor {
					continue
				}
				pa := lerp(from, to, (o.A-coord(from, ax))/span)
				pb := lerp(from, to, (o.B-coord(from, ax))/span)
				out = append(out, door{pa, pb, o.Swing, o.Hinge})
			}
		}
		return out
	}
	g := collect(got)
	good, n := 0, 0
	for _, w := range collect(want) {
		n++
		if slices.ContainsFunc(g, func(d door) bool {
			return dist(d.a, w.a) <= 2 && dist(d.b, w.b) <= 2 && d.swing == w.swing &&
				d.hinge == w.hinge
		}) {
			good++
		}
	}
	return float64(good) / float64(n)
}

// The golden hand traces, turned back into the evidence a perfect front end
// would find, must come back: the walls, the rooms the walls enclose, the
// names, the openings with their swings. Rooms a hand trace splits without a
// wall between them (office's coffee area and retreat; corporate's open
// plan) can't come back from walls alone, which bounds the room numbers.
func TestOracle(t *testing.T) {
	for _, tc := range []struct {
		name                                  string
		recall, precision, named, kind, swing float64
		minRooms, maxRooms                    int
	}{
		{"office", 0.99, 0.99, 0.8, 0.99, 0.99, 11, 13},
		{"corporate", 0.99, 0.99, 0.65, 0.99, 0.99, 52, 62},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, s := oracle(t, tc.name)
			t.Logf(
				"%s: wall recall %.3f precision %.3f; kinds agree %.3f; rooms %d (hand %d); names %d/%d; openings %v (hand %v); doors right %.3f",
				tc.name,
				s.recall,
				s.precision,
				s.kindAgree,
				s.rooms,
				s.wantRooms,
				s.named,
				s.wantNamed,
				s.ops,
				s.wantOps,
				s.swingGood,
			)
			if s.recall < tc.recall || s.precision < tc.precision {
				t.Errorf(
					"wall recall %.3f precision %.3f, want ≥ %.2f, %.2f",
					s.recall,
					s.precision,
					tc.recall,
					tc.precision,
				)
			}
			if s.kindAgree < tc.kind {
				t.Errorf("kinds agree on %.3f of the length, want ≥ %.2f", s.kindAgree, tc.kind)
			}
			if s.rooms < tc.minRooms || s.rooms > tc.maxRooms {
				t.Errorf("%d rooms, want %d–%d", s.rooms, tc.minRooms, tc.maxRooms)
			}
			if float64(s.named) < tc.named*float64(s.wantNamed) {
				t.Errorf("%d of %d names, want ≥ %.0f%%", s.named, s.wantNamed, 100*tc.named)
			}
			if s.swingGood < tc.swing {
				t.Errorf("doors right %.3f, want ≥ %.2f", s.swingGood, tc.swing)
			}
			for k, n := range s.wantOps {
				if s.ops[k] != n {
					t.Errorf("%d %s openings, hand trace has %d", s.ops[k], k, n)
				}
			}
		})
	}
}
