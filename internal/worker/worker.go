// Package worker takes queued plans from the catalogue, traces each one, and
// writes the results into the plan's directory for the microsite.
package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/weaveplatform/spaceweave/internal/catalog"
	"github.com/weaveplatform/spaceweave/internal/convert"
	"github.com/weaveplatform/spaceweave/internal/plan"
	"github.com/weaveplatform/spaceweave/internal/raster"
	"github.com/weaveplatform/spaceweave/internal/trace"
)

// SitePrefix is where the microsite finds the output directory, relative to
// its own root. It must match GENERATED in web/floorplan-iso/js/plans/index.js,
// or the "Source" link on a generated plan 404s.
const SitePrefix = "generated/"

// Files written into each plan's directory, besides the source image.
const (
	TraceFile      = "trace.json"
	PlanFile       = "plan.json"
	OverlayFile    = "overlay.png"
	ReportFile     = "report.json"
	TranscriptFile = "transcript.jsonl"
)

// DefaultPoll is how often an idle worker looks for queued plans. The inbox
// wakes workers directly; polling catches plans re-queued by `plans reject`
// or `plans retry`, which run in another process.
const DefaultPoll = 2 * time.Second

// TraceFunc runs the tracer; trace.Run outside tests.
type TraceFunc func(context.Context, trace.Options, trace.Job) (*trace.Result, error)

// Worker processes plans.
type Worker struct {
	Store   *catalog.Store
	Options trace.Options
	Trace   TraceFunc // trace.Run when nil
	Convert convert.Options
	Log     *slog.Logger
	Poll    time.Duration // DefaultPoll when 0
}

// Conversion is report.json's account of turning a drawing into an image.
type Conversion struct {
	Format string   `json:"format"`
	Width  int      `json:"width"`
	Height int      `json:"height"`
	Scale  string   `json:"scale"`
	Notes  []string `json:"notes,omitempty"`
}

// Report is report.json: what the reviewer needs to judge a draft, or to see
// why a run failed.
type Report struct {
	ID          string      `json:"id"`
	Status      string      `json:"status"`
	Model       string      `json:"model"`
	Attempt     int         `json:"attempt"`
	Feedback    string      `json:"feedback,omitempty"`
	Conversion  *Conversion `json:"conversion,omitempty"`
	Confidence  float64     `json:"confidence"`
	Summary     string      `json:"summary,omitempty"`
	Caveats     []string    `json:"caveats,omitempty"`
	Calibration string      `json:"calibration,omitempty"`
	Size        string      `json:"size,omitempty"`
	Stopped     string      `json:"stopped,omitempty"`
	Problems    []string    `json:"problems,omitempty"`
	Error       string      `json:"error,omitempty"`
	Turns       int         `json:"turns"`
	Submits     int         `json:"submits"`
	Usage       trace.Usage `json:"usage"`
	CostUSD     float64     `json:"costUsd"`
	Seconds     float64     `json:"seconds"`
}

func (w *Worker) log() *slog.Logger {
	if w.Log == nil {
		return slog.New(slog.DiscardHandler)
	}
	return w.Log
}

// Run requeues plans a previous daemon left mid-trace, then runs n workers
// until ctx is done. A send on wake (from the inbox) starts one looking now.
func (w *Worker) Run(ctx context.Context, n int, wake <-chan struct{}) error {
	recovered, err := w.Store.Recover()
	if err != nil {
		return fmt.Errorf("recover interrupted plans: %w", err)
	}
	if recovered > 0 {
		w.log().Info("requeued plans interrupted by the last shutdown", "count", recovered)
	}
	poll := w.Poll
	if poll <= 0 {
		poll = DefaultPoll
	}
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			tick := time.NewTicker(poll)
			defer tick.Stop()
			for ctx.Err() == nil {
				worked, err := w.Next(ctx)
				if err != nil {
					w.log().Error("worker", "err", err)
				}
				if worked {
					continue
				}
				select {
				case <-ctx.Done():
				case <-wake:
				case <-tick.C:
				}
			}
		})
	}
	wg.Wait()
	return nil
}

// Next processes the oldest queued plan, if there is one.
func (w *Worker) Next(ctx context.Context) (bool, error) {
	// A read first, so an idle poll doesn't rewrite catalog.json (which the
	// site is polling too) every couple of seconds.
	c, err := w.Store.Load()
	if err != nil {
		return false, fmt.Errorf("read catalog: %w", err)
	}
	queued := false
	for _, e := range c.Plans {
		queued = queued || e.Status == catalog.Queued
	}
	if !queued {
		return false, nil
	}
	e, ok, err := w.Store.Claim()
	if err != nil || !ok {
		return false, err //nolint:wrapcheck // the catalogue's errors name themselves
	}
	return true, w.Process(ctx, e)
}

// Process converts (if it is a drawing) and traces one claimed plan, and
// records the outcome.
func (w *Worker) Process(ctx context.Context, e catalog.Entry) error {
	started := time.Now()
	log := w.log().With("plan", e.ID, "attempt", e.Attempts)
	dir := w.Store.PlanDir(e.ID)
	report := Report{ID: e.ID, Model: w.Options.Model, Attempt: e.Attempts, Feedback: e.Note}

	if e.Status == catalog.Converting {
		log.Info("converting", "format", e.Format)
		converted, conv, err := w.convert(ctx, e)
		report.Conversion = conv
		switch {
		case ctx.Err() != nil:
			log.Info("interrupted; requeued")
			return w.transition(e.ID, catalog.Queued, nil)
		case err != nil:
			return w.fail(e, &report, nil, started, fmt.Errorf("convert %s: %w", e.Original, err))
		}
		e = converted
	} else if f, ok := convert.ByID(e.Format); ok && f.Drawing {
		report.Conversion = readConversion(filepath.Join(dir, ReportFile))
	}
	log.Info("tracing")

	src, err := raster.Load(filepath.Join(dir, e.Source))
	if err != nil {
		return w.fail(e, &report, nil, started, fmt.Errorf("read image: %w", err))
	}
	wd, ht := src.Size()
	job := trace.Job{
		Image:    src,
		Title:    e.Title,
		Feedback: e.Note,
		Meta: plan.Meta{
			URL:    path.Join(SitePrefix, e.ID, e.Source),
			Credit: "Uploaded " + e.Original,
			SizePx: [2]int{wd, ht},
		},
	}
	if e.Note != "" {
		// A reviewer sent this back: start from the trace they reviewed.
		job.Previous = readTrace(filepath.Join(dir, TraceFile))
	}
	if c := report.Conversion; c != nil {
		job.Drawing = &trace.Drawing{Format: c.Format, Notes: c.Notes}
		if sc := e.Scale; sc != nil {
			job.Drawing.PxPerMetre, job.Drawing.Assumed, job.Drawing.PxPerPaperMM = sc.PxPerMetre, sc.Assumed, sc.PxPerPaperMM
		}
		job.Meta.Credit = fmt.Sprintf("Uploaded %s, rendered from %s", e.Original, c.Format)
	}

	//nolint:gosec // inside the output dir
	transcript, err := os.Create(filepath.Join(dir, TranscriptFile))
	if err != nil {
		return w.fail(e, &report, nil, started, fmt.Errorf("create transcript: %w", err))
	}
	defer transcript.Close() //nolint:errcheck // best effort; the report is what matters
	opts := w.Options
	opts.Transcript = transcript
	opts.Progress = func(p trace.Progress) {
		// Best effort: a missed update only makes the site's view lag.
		err := w.Store.SetProgress(
			e.ID,
			catalog.Progress{Turn: p.Turn, Submits: p.Submits, CostUSD: p.CostUSD},
		)
		if err != nil {
			log.Warn("record progress", "err", err)
		}
	}
	run := w.Trace
	if run == nil {
		run = trace.Run
	}
	res, err := run(ctx, opts, job)
	if res == nil {
		res = &trace.Result{}
	}
	report.Turns, report.Submits, report.Usage, report.CostUSD = res.Turns, res.Submits, res.Usage, res.CostUSD
	report.Stopped, report.Problems = res.Stopped, res.Problems

	if ctx.Err() != nil {
		// Shutting down: put it back rather than publish or fail a run that
		// was cut short for reasons that have nothing to do with the plan.
		log.Info("interrupted; requeued")
		return w.transition(
			e.ID,
			catalog.Queued,
			func(x *catalog.Entry) {
				x.CostUSD += res.CostUSD
			},
		)
	}
	if err != nil {
		return w.fail(e, &report, res, started, err)
	}
	return w.publish(e, &report, res, started)
}

func (w *Worker) publish(
	e catalog.Entry,
	report *Report,
	res *trace.Result,
	started time.Time,
) error {
	dir := w.Store.PlanDir(e.ID)
	traceJSON, err := json.MarshalIndent(res.Trace, "", " ")
	if err != nil {
		return w.fail(e, report, res, started, fmt.Errorf("encode trace: %w", err))
	}
	var planJSON bytes.Buffer
	if err := json.Indent(&planJSON, res.PlanJSON, "", " "); err != nil {
		return w.fail(e, report, res, started, fmt.Errorf("encode plan: %w", err))
	}
	report.Status = string(catalog.Draft)
	report.Confidence, report.Summary, report.Caveats = res.Confidence, res.Summary, res.Caveats
	report.Calibration = res.Trace.Calibration.Evidence
	report.Size = plan.Describe(res.Plan)
	report.Seconds = time.Since(started).Seconds()
	reportJSON, err := json.MarshalIndent(report, "", " ")
	if err != nil {
		return w.fail(e, report, res, started, fmt.Errorf("encode report: %w", err))
	}
	// The catalogue changes only after every file is in place, so the site
	// never lists a draft whose plan.json is missing or stale.
	for _, f := range []struct {
		name string
		data []byte
	}{
		{TraceFile, append(traceJSON, '\n')},
		{OverlayFile, res.Overlay},
		{ReportFile, append(reportJSON, '\n')},
		{PlanFile, append(planJSON.Bytes(), '\n')},
	} {
		if err := catalog.WriteFile(filepath.Join(dir, f.name), f.data); err != nil {
			return w.fail(e, report, res, started, err)
		}
	}
	confidence := res.Confidence
	w.log().
		Info("draft ready", "plan", e.ID, "confidence", confidence, "costUsd", res.CostUSD, "stopped", res.Stopped)
	return w.transition(
		e.ID,
		catalog.Draft,
		func(x *catalog.Entry) {
			x.Confidence = &confidence
			x.Summary = res.Summary
			x.CostUSD += res.CostUSD
			x.Note, x.Error = "", ""
		},
	)
}

// fail records a failed run. The returned error is only ever the catalogue's:
// the run's own failure is recorded, not propagated, so one bad image
// doesn't stop the worker.
func (w *Worker) fail(
	e catalog.Entry,
	report *Report,
	res *trace.Result,
	started time.Time,
	cause error,
) error {
	w.log().Warn("trace failed", "plan", e.ID, "err", cause)
	report.Status = string(catalog.Failed)
	report.Error = cause.Error()
	report.Seconds = time.Since(started).Seconds()
	cost := 0.0
	if res != nil {
		cost = res.CostUSD
	}
	if b, err := json.MarshalIndent(report, "", " "); err == nil {
		if err := catalog.WriteFile(
			filepath.Join(w.Store.PlanDir(e.ID), ReportFile),
			append(b, '\n'),
		); err != nil {
			w.log().Warn("write report", "plan", e.ID, "err", err)
		}
	}
	return w.transition(
		e.ID,
		catalog.Failed,
		func(x *catalog.Entry) {
			x.Error = cause.Error()
			x.CostUSD += cost
		},
	)
}

// convert renders a drawing into the plan's directory and records it,
// moving the plan on to tracing.
func (w *Worker) convert(ctx context.Context, e catalog.Entry) (catalog.Entry, *Conversion, error) {
	dir := w.Store.PlanDir(e.ID)
	res, err := convert.File(ctx, filepath.Join(dir, e.Upload), dir, w.Convert)
	if err != nil {
		return e, nil, err //nolint:wrapcheck // Process names the file
	}
	conv := &Conversion{
		Format: res.Format.Label,
		Width:  res.Width,
		Height: res.Height,
		Scale:  res.Scale(),
		Notes:  res.Notes,
	}
	var scale *catalog.Scale
	if res.PxPerMetre > 0 || res.PxPerPaperMM > 0 {
		scale = &catalog.Scale{
			PxPerMetre:   res.PxPerMetre,
			Assumed:      res.Assumed,
			PxPerPaperMM: res.PxPerPaperMM,
		}
	}
	w.log().
		Info("converted", "plan", e.ID, "width", res.Width, "height", res.Height, "scale", res.Scale())
	now, err := w.Store.Converted(e.ID, convert.ImageFile, convert.DrawingFile, scale)
	if err != nil {
		return e, conv, fmt.Errorf("update catalog: %w", err)
	}
	return now, conv, nil
}

// readConversion recovers a drawing's conversion from its last report, for
// a retrace that doesn't convert again.
func readConversion(file string) *Conversion {
	b, err := os.ReadFile(file) //nolint:gosec // inside the output dir
	if err != nil {
		return nil
	}
	var r Report
	if json.Unmarshal(b, &r) != nil {
		return nil
	}
	return r.Conversion
}

// readTrace loads a previous trace, or nil when there's none to build on.
func readTrace(file string) *plan.Trace {
	b, err := os.ReadFile(file) //nolint:gosec // inside the output dir
	if err != nil {
		return nil
	}
	var t plan.Trace
	if err := json.Unmarshal(b, &t); err != nil {
		return nil
	}
	return &t
}

func (w *Worker) transition(id string, to catalog.Status, mutate func(*catalog.Entry)) error {
	if err := w.Store.Transition(id, to, mutate); err != nil {
		return fmt.Errorf("update catalog: %w", err)
	}
	return nil
}
