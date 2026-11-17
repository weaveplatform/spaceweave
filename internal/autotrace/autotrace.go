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
	"math"
	"path/filepath"
	"strings"

	"github.com/weaveplatform/spaceweave/internal/autotrace/cadfront"
	"github.com/weaveplatform/spaceweave/internal/autotrace/profile"
	"github.com/weaveplatform/spaceweave/internal/autotrace/rasterfront"
	"github.com/weaveplatform/spaceweave/internal/floorgraph"
	"github.com/weaveplatform/spaceweave/internal/raster"
	"github.com/weaveplatform/spaceweave/internal/trace"
)

// Version identifies the tracer's rules in report.json. Bump it when a
// change can alter the plan traced from the same file, so a reviewer can
// tell an old draft from a new one.
const Version = "6"

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
	in, layers, err := read(job)
	// The layers go in the report even when the trace fails: a drawing
	// whose walls are on a layer no profile names fails for want of walls,
	// and the layers list is where a reviewer says which layer holds them.
	res.Layers = layers
	if err != nil {
		return res, err
	}
	var spacesCaveat string
	in, res.Spaces, spacesCaveat = readSpaces(job, in)
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
	if spacesCaveat != "" {
		res.Caveats = append(res.Caveats, spacesCaveat)
	}
	if strings.TrimSpace(job.Feedback) != "" {
		res.Caveats = append(res.Caveats, feedbackCaveat)
	}
	return res, nil
}

// cadOptions are the options cadfront reads the job's drawing with; ok is
// false when it is read from its image instead. A drawing is read as
// entities only when the worker kept the transform onto its image; without
// it the entities can't be placed on the overlay, and the image is the
// next best thing.
func cadOptions(job trace.Job) (cadfront.Options, bool) {
	ext := strings.ToLower(filepath.Ext(job.Original))
	if job.View == nil || (ext != ".dxf" && ext != ".dwg") {
		return cadfront.Options{}, false
	}
	w, h := job.Image.Size()
	o := cadfront.Options{
		View:    *job.View,
		Size:    [2]int{w, h},
		Title:   job.Title,
		Profile: job.Profile,
	}
	if d := job.Drawing; d != nil {
		o.PxPerMetre, o.Exact = d.PxPerMetre, d.PxPerMetre > 0 && !d.Assumed
	}
	return o, true
}

// read picks the front end.
func read(job trace.Job) (floorgraph.Input, []profile.Layer, error) {
	if o, ok := cadOptions(job); ok {
		in, layers, err := cadfront.AnalyseFile(job.Original, o)
		if err != nil {
			return in, layers, fmt.Errorf("read drawing: %w", err)
		}
		return in, layers, nil
	}
	var o rasterfront.Options
	if d := job.Drawing; d != nil && d.PxPerMetre > 0 && !d.Assumed {
		o.Scale = floorgraph.Scale{
			PxPerMetre: [2]float64{d.PxPerMetre, d.PxPerMetre},
			Exact:      true,
			Evidence:   "From the drawing's units.",
		}
	}
	in, err := rasterfront.Read(job.Image, o)
	if err != nil {
		return in, nil, fmt.Errorf("read image: %w", err)
	}
	if in.Title == "" {
		in.Title = job.Title
	}
	return in, nil, nil
}

// readSpaces reads the job's space plan, places it on the drawing read into
// in, and hands its spaces over unless placing them was too poor to trust.
// A space plan that can't be read or placed costs the trace nothing but
// the spaces: the plan is traced from the drawing alone, and the caveat
// says why.
func readSpaces(job trace.Job, in floorgraph.Input) (floorgraph.Input, *trace.Spaces, string) {
	if len(job.Spaces) == 0 {
		return in, nil, ""
	}
	// The catalogue keeps one space plan per plan.
	file := job.Spaces[0].Path
	rep := &trace.Spaces{File: filepath.Base(file)}
	o, ok := cadOptions(job)
	if !ok {
		rep.Error = "the plan was traced from an image, which a space plan can't be placed on"
		return in, rep, "The space plan was not used: " + rep.Error + "."
	}
	spaces, reg, err := cadfront.ReadSpaces(file, o, in)
	if err != nil {
		rep.Error = err.Error()
		return in, rep, fmt.Sprintf(
			"The space plan %s couldn't be read (%v); rooms were found from the drawing alone.",
			rep.File,
			err,
		)
	}
	t := reg.Transform
	rep.Layer, rep.Spaces, rep.Method, rep.Notes = reg.Layer, len(spaces), reg.Method, reg.Notes
	rep.Transform = &[6]float64{t.A, t.B, t.C, t.D, t.E, t.F}
	rep.ResidualM, rep.Matched = round3(reg.ResidualM), round3(reg.Matched)
	if reg.Poor {
		return in, rep, fmt.Sprintf(
			"The space plan %s doesn't line up with the drawing: %.0f%% of the corners of its %d outlines lie "+
				"on a wall once placed, where at least half should. Its spaces were not used, and rooms were "+
				"found from the drawing alone; check the two files are the same floor.",
			rep.File,
			100*math.Max(reg.Matched, 0),
			len(spaces),
		)
	}
	rep.Used = true
	in.Spaces = spaces
	return in, rep, ""
}

// round3 keeps report.json's measures to the millimetre and the tenth of
// a per cent: finer is noise from the fit.
func round3(v float64) float64 { return math.Round(v*1000) / 1000 }
