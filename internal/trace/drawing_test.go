package trace

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/fakeclaude"
	"github.com/weaveplatform/spaceweave/internal/plan"
)

func TestDrawingDescribesItsScale(t *testing.T) {
	cases := []struct {
		d    Drawing
		want []string
	}{
		{
			Drawing{Format: "DXF", PxPerMetre: 25},
			[]string{"states its units", "25.0000 px per metre", "fixed"},
		},
		{
			Drawing{Format: "DXF", PxPerMetre: 25, Assumed: true},
			[]string{"doesn't state its units", "Check that"},
		},
		{Drawing{Format: "SVG", PxPerPaperMM: 3}, []string{"on paper", "3.0000 px per paper"}},
		{
			Drawing{Format: "SVG", Notes: []string{"Not drawn: 1 LEADER."}},
			[]string{"says nothing about its scale", "Conversion note: Not drawn: 1 LEADER."},
		},
	}
	for _, c := range cases {
		got := c.d.describe()
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				t.Errorf("%+v: %q lacks %q", c.d, got, w)
			}
		}
	}
}

func TestPinFixesAnExactScale(t *testing.T) {
	r := &run{job: Job{Drawing: &Drawing{Format: "DXF", PxPerMetre: 25}}}
	tr := &plan.Trace{
		Calibration: plan.Calibration{PxPerMetre: [2]float64{20, 25}, Evidence: "door 0.9 m"},
	}
	msg := pin(r.job.Drawing, tr)
	if tr.Calibration.PxPerMetre != [2]float64{25, 25} ||
		tr.Calibration.Evidence != "From the drawing's units. door 0.9 m" {
		t.Fatalf("%+v", tr.Calibration)
	}
	if !strings.Contains(msg, "fixed to the drawing's 25.0000") {
		t.Fatalf("a scale that was off is reported: %q", msg)
	}
	// Within tolerance, and already citing the units: pinned quietly, and
	// the evidence isn't prefixed twice.
	if msg := pin(r.job.Drawing,
		tr,
	); msg != "" ||
		strings.Count(tr.Calibration.Evidence, "drawing's units") != 1 {
		t.Fatalf("%q %q", msg, tr.Calibration.Evidence)
	}
	for _, d := range []*Drawing{nil, {PxPerMetre: 25, Assumed: true}, {PxPerPaperMM: 3}} {
		r := &run{job: Job{Drawing: d}}
		tr := &plan.Trace{Calibration: plan.Calibration{PxPerMetre: [2]float64{20, 20}}}
		if msg := pin(r.job.Drawing, tr); msg != "" || tr.Calibration.PxPerMetre != [2]float64{20, 20} {
			t.Errorf("%+v isn't pinned: %q %+v", d, msg, tr.Calibration)
		}
	}
}

func TestOpeningSaysTheDrawingsScale(t *testing.T) {
	srv := fakeclaude.New(func(int, fakeclaude.Request) fakeclaude.Reply {
		return fakeclaude.Reply{StopReason: "refusal"}
	})
	defer srv.Close()
	j := job(t)
	j.Drawing = &Drawing{Format: "DXF", PxPerMetre: 20}
	if _, err := Run(
		context.Background(),
		options(srv, &fakeValidator{}),
		j,
	); !errors.Is(
		err,
		ErrNoTrace,
	) {
		t.Fatal(err)
	}
	if opening := string(
		srv.Requests()[0].Messages[0],
	); !strings.Contains(
		opening,
		"20.0000 px per metre",
	) {
		t.Fatalf("the opening message doesn't give the scale: %.300s", opening)
	}
}
