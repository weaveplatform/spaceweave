// Package autotrace traces a plan without a model: the same image or drawing
// gives the same plan.json every time, and nothing leaves the machine.
//
// A DXF is read as CAD entities (cadfront), so layer names, door blocks and
// room text survive; anything else is read from its image (rasterfront).
// Both feed floorgraph, which builds the trace, and trace.Finalize, which
// puts it through the same checks a Claude trace gets.
package autotrace

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/weaveplatform/spaceweave/internal/autotrace/cadfront"
	"github.com/weaveplatform/spaceweave/internal/autotrace/rasterfront"
	"github.com/weaveplatform/spaceweave/internal/floorgraph"
	"github.com/weaveplatform/spaceweave/internal/raster"
	"github.com/weaveplatform/spaceweave/internal/trace"
)

// Version identifies the tracer's rules in report.json. Bump it when a
// change can alter the plan traced from the same file, so a reviewer can
// tell an old draft from a new one.
const Version = "5"

// ErrNoImage is returned for a job without an image to trace.
var ErrNoImage = errors.New("autotrace: no image")

// feedbackCaveat is added when a reviewer sent the plan back with a note.
// The automatic tracer can't act on it, and a reviewer who sees the same
// draft again should know why.
const feedbackCaveat = "Your note was kept, but the automatic tracer doesn't read notes: " +
	"this is the same trace as before. Edit the plan, or trace it with `tracer: claude`."

// Run traces job. It matches worker.TraceFunc; only opts.Validator is used.
// The result is never nil.
func Run(ctx context.Context, opts trace.Options, job trace.Job) (*trace.Result, error) {
	res := &trace.Result{}
	if job.Image == nil {
		return res, ErrNoImage
	}
	in, err := read(job)
	if err != nil {
		return res, err
	}
	if err := ctx.Err(); err != nil {
		return res, err //nolint:wrapcheck // the caller checks ctx itself
	}
	built, err := floorgraph.Build(in)
	if err != nil {
		return res, fmt.Errorf("%w: %w", trace.ErrNoTrace, err)
	}
	f, err := trace.Finalize(ctx, job, built.Trace, opts.Validator)
	if err != nil {
		return res, err //nolint:wrapcheck // Finalize says what failed
	}
	if !f.Valid() {
		res.Problems = f.Problems
		return res, fmt.Errorf(
			"%w: the trace failed %d checks, first: %s",
			trace.ErrNoTrace,
			len(f.Problems),
			f.Problems[0],
		)
	}
	png, err := raster.EncodePNG(f.Overlay)
	if err != nil {
		return res, fmt.Errorf("encode overlay: %w", err)
	}
	res.Trace, res.Plan, res.PlanJSON, res.Overlay = f.Trace, f.Plan, f.PlanJSON, png
	res.Confidence, res.Summary, res.Caveats = built.Confidence, built.Summary, built.Caveats
	if strings.TrimSpace(job.Feedback) != "" {
		res.Caveats = append(res.Caveats, feedbackCaveat)
	}
	return res, nil
}

// read picks the front end. A drawing is read as entities only when the
// worker kept the transform onto its image; without it the entities can't
// be placed on the overlay, and the image is the next best thing.
func read(job trace.Job) (floorgraph.Input, error) {
	w, h := job.Image.Size()
	d := job.Drawing
	exact := d != nil && d.PxPerMetre > 0 && !d.Assumed
	ext := strings.ToLower(filepath.Ext(job.Original))
	if job.View != nil && (ext == ".dxf" || ext == ".dwg") {
		o := cadfront.Options{View: *job.View, Size: [2]int{w, h}, Title: job.Title}
		if d != nil {
			o.PxPerMetre, o.Exact = d.PxPerMetre, exact
		}
		in, err := cadfront.ReadFile(job.Original, o)
		if err != nil {
			return in, fmt.Errorf("read drawing: %w", err)
		}
		return in, nil
	}
	var o rasterfront.Options
	if exact {
		o.Scale = floorgraph.Scale{
			PxPerMetre: [2]float64{d.PxPerMetre, d.PxPerMetre},
			Exact:      true,
			Evidence:   "From the drawing's units.",
		}
	}
	in, err := rasterfront.Read(job.Image, o)
	if err != nil {
		return in, fmt.Errorf("read image: %w", err)
	}
	if in.Title == "" {
		in.Title = job.Title
	}
	return in, nil
}
