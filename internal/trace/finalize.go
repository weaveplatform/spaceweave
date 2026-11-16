package trace

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"math"
	"strings"

	"github.com/weaveplatform/spaceweave/internal/plan"
	"github.com/weaveplatform/spaceweave/internal/raster"
)

// Finalized is a trace that has been through every check a published plan
// must pass, with the overlay drawn from it.
type Finalized struct {
	Trace    *plan.Trace
	Plan     *plan.Plan // nil when the trace has structural problems
	PlanJSON []byte
	Overlay  *image.RGBA
	// Problems are plan.Check's and then the site validator's; empty when
	// the trace can be published.
	Problems []string
	// Pinned says the drawing's exact scale replaced the trace's own, and
	// by how much it differed; empty when nothing changed noticeably.
	Pinned string
}

// Valid reports whether f can be published.
func (f *Finalized) Valid() bool { return f != nil && f.Plan != nil && len(f.Problems) == 0 }

// Finalize pins the drawing's scale, checks t, draws its overlay and, when
// it is structurally sound, converts it and runs the site's validator. Every
// tracer publishes through here, so a plan that reaches the site has passed
// the same checks whoever traced it.
//
// The error is only for failures no change to t can fix: the plan couldn't
// be encoded, or the validator itself didn't run.
func Finalize(ctx context.Context, job Job, t *plan.Trace, v plan.Validator) (*Finalized, error) {
	f := &Finalized{Trace: t, Pinned: pin(job.Drawing, t)}
	f.Problems = plan.Check(t)
	f.Overlay = raster.Overlay(job.Image.Image, plan.Shapes(t))
	if len(f.Problems) > 0 {
		return f, nil
	}
	p, err := plan.Convert(t, job.Meta)
	if err != nil {
		f.Problems = []string{err.Error()}
		return f, nil
	}
	f.PlanJSON, err = json.Marshal(p)
	if err != nil {
		return f, fmt.Errorf("encode plan: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, validateTimeout)
	defer cancel()
	verdict, err := v.Validate(ctx, f.PlanJSON)
	if err != nil {
		return f, fmt.Errorf("validate plan: %w", err)
	}
	f.Plan, f.Problems = p, verdict
	return f, nil
}

// pinTolerance is how far a submitted scale may differ from a drawing's
// before the tracer is told it was replaced.
const pinTolerance = 0.01

// pin fixes the trace's scale to the drawing's when the drawing states its
// units, and says so when the trace's own value was off, so a tracer isn't
// left wondering why the overlay moved.
func pin(d *Drawing, t *plan.Trace) string {
	if !d.exact() {
		return ""
	}
	was := t.Calibration.PxPerMetre
	t.Calibration.PxPerMetre = [2]float64{d.PxPerMetre, d.PxPerMetre}
	if !strings.Contains(strings.ToLower(t.Calibration.Evidence), "drawing's units") {
		t.Calibration.Evidence = strings.TrimSpace(
			"From the drawing's units. " + t.Calibration.Evidence,
		)
	}
	for _, v := range was {
		if math.Abs(v-d.PxPerMetre) > pinTolerance*d.PxPerMetre {
			return fmt.Sprintf(
				"calibration.pxPerMetre was %v; it is fixed to the drawing's %.4f.",
				was,
				d.PxPerMetre,
			)
		}
	}
	return ""
}
