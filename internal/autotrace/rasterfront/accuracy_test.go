package rasterfront

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/autotrace/score"
	"github.com/weaveplatform/spaceweave/internal/autotrace/synth"
	"github.com/weaveplatform/spaceweave/internal/floorgraph"
	"github.com/weaveplatform/spaceweave/internal/plan"
	"github.com/weaveplatform/spaceweave/internal/raster"
)

func fixture(t *testing.T, name string) *plan.Trace {
	t.Helper()
	b, err := os.ReadFile("../../plan/testdata/" + name + ".trace.json")
	if err != nil {
		t.Fatal(err)
	}
	var tr plan.Trace
	if err := json.Unmarshal(b, &tr); err != nil {
		t.Fatal(err)
	}
	return &tr
}

func readRender(t *testing.T, tr *plan.Trace, so synth.Options, o Options) floorgraph.Input {
	t.Helper()
	in, err := Read(raster.NewSource(synth.Render(tr, so)), o)
	if err != nil {
		t.Fatal(err)
	}
	return in
}

// asTrace is the evidence as a trace score can read: every wall segment,
// and every opening as a wall of its own carrying it, since a hand trace's
// wall runs through its openings.
func asTrace(in floorgraph.Input, openings bool) *plan.Trace {
	tr := &plan.Trace{}
	for _, s := range in.Walls {
		tr.Walls = append(
			tr.Walls,
			plan.Wall{From: [2]float64{s.A.X, s.A.Y}, To: [2]float64{s.B.X, s.B.Y}},
		)
	}
	if !openings {
		return tr
	}
	for _, o := range in.Openings {
		a, b := o.A.X, o.B.X
		if o.A.X == o.B.X {
			a, b = o.A.Y, o.B.Y
		}
		tr.Walls = append(tr.Walls, plan.Wall{
			From:     [2]float64{o.A.X, o.A.Y},
			To:       [2]float64{o.B.X, o.B.Y},
			Openings: []plan.Opening{{A: a, B: b, Kind: o.Kind}},
		})
	}
	return tr
}

const tolPx = 3

type result struct {
	segRecall, segPrecision, recall, precision float64
	openings                                   map[string]score.OpeningMatch
}

func measure(t *testing.T, tr *plan.Trace, so synth.Options, o Options) result {
	t.Helper()
	in := readRender(t, tr, so, o)
	var r result
	r.segRecall, r.segPrecision = score.Walls(asTrace(in, false), tr, tolPx)
	r.recall, r.precision = score.Walls(asTrace(in, true), tr, tolPx)
	// Openings are matched on their middles, which a door's jamb drawn a
	// pixel off moves by less than the tolerance a quarter of a door gives.
	r.openings = score.Openings(asTrace(in, true), tr, 8)
	return r
}

func (r result) String() string {
	kinds := make([]string, 0, len(r.openings))
	for k := range r.openings {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	var b strings.Builder
	for _, k := range kinds {
		m := r.openings[k]
		fmt.Fprintf(&b, " %s %d/%d (found %d)", k, m.Matched, m.Want, m.Got)
	}
	return fmt.Sprintf(
		"segments recall %.3f precision %.3f; with openings recall %.3f precision %.3f; openings:%s",
		r.segRecall,
		r.segPrecision,
		r.recall,
		r.precision,
		b.String(),
	)
}

var everything = synth.Options{Text: true, Furniture: true, AntiAlias: true}

// The office fixture is all axis-aligned walls at least 3 px thick, except
// one glass partition, which reads as a window: everything is findable.
func TestAccuracyOffice(t *testing.T) {
	tr := fixture(t, "office")
	for _, c := range []struct {
		name string
		so   synth.Options
	}{
		{"text and furniture", everything},
		{"jpeg and noise", synth.Options{Text: true, Furniture: true, AntiAlias: true, Noise: 24, Seed: 7, JPEG: 70}},
		{"walls only", synth.Options{}},
	} {
		r := measure(t, tr, c.so, Options{})
		t.Logf("office, %s: %v", c.name, r)
		if r.recall < 0.95 || r.precision < 0.9 {
			t.Errorf(
				"office, %s: recall %.3f precision %.3f, want ≥ 0.95, ≥ 0.9",
				c.name,
				r.recall,
				r.precision,
			)
		}
		if d := r.openings["door"]; d.Matched < d.Want-1 {
			t.Errorf("office, %s: %d of %d doors", c.name, d.Matched, d.Want)
		}
		if d := r.openings["double-door"]; d.Matched < d.Want {
			t.Errorf("office, %s: %d of %d double doors", c.name, d.Matched, d.Want)
		}
	}
}

// The corporate fixture has walls this front end can't see: about 12% of
// the wall length is lift shafts and risers drawn as free-standing hairline
// boxes, which meet no wall, and the 2 px partitions slanted at 163° are
// too thin to band once turned. The hairline partitions that run wall to
// wall and the thick slanted walls are read. The floor guards against
// regressions, two points under what this version measures. It is
// read once, without a scale: a given scale only sizes the door search,
// which TestReadRoom covers, and the image is large enough to dominate the
// suite's time under the race detector.
func TestAccuracyCorporate(t *testing.T) {
	tr := fixture(t, "corporate")
	r := measure(t, tr, everything, Options{})
	t.Logf("corporate: %v", r)
	if r.recall < 0.84 || r.precision < 0.76 {
		t.Errorf(
			"corporate: recall %.3f precision %.3f, want ≥ 0.84, ≥ 0.76",
			r.recall,
			r.precision,
		)
	}
}
