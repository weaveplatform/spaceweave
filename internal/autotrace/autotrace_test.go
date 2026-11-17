package autotrace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/autotrace/profile"
	"github.com/weaveplatform/spaceweave/internal/autotrace/synth"
	"github.com/weaveplatform/spaceweave/internal/convert"
	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
	"github.com/weaveplatform/spaceweave/internal/plan"
	"github.com/weaveplatform/spaceweave/internal/raster"
	"github.com/weaveplatform/spaceweave/internal/trace"
)

type verdict struct {
	problems []string
	err      error
}

func (v verdict) Validate(context.Context, []byte) ([]string, error) { return v.problems, v.err }

// validator is the site's own when node is installed, so the plans built
// here are the ones the page would accept; otherwise every plan passes.
func validator(t *testing.T) plan.Validator {
	t.Helper()
	script, _ := filepath.Abs("../../web/floorplan-iso/tools/validate.mjs")
	if _, err := exec.LookPath("node"); err != nil {
		return verdict{}
	}
	if _, err := os.Stat(script); err != nil {
		return verdict{}
	}
	return plan.NodeValidator{Node: "node", Script: script}
}

// drawingJob converts a DXF the way the worker does and builds its job.
func drawingJob(t *testing.T, dxf []byte) trace.Job {
	t.Helper()
	dir := t.TempDir()
	original := filepath.Join(dir, "original.dxf")
	if err := os.WriteFile(original, dxf, 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := convert.File(context.Background(), original, dir, convert.Options{})
	if err != nil {
		t.Fatal(err)
	}
	src, err := raster.Load(res.Image)
	if err != nil {
		t.Fatal(err)
	}
	view := res.View
	return trace.Job{
		Image: src,
		Title: "Office",
		Meta:  plan.Meta{URL: "generated/x/source.png", Credit: "test", SizePx: [2]int{res.Width, res.Height}},
		Drawing: &trace.Drawing{
			Format: "DXF", PxPerMetre: res.PxPerMetre, Assumed: res.Assumed, Notes: res.Notes,
		},
		Original: original,
		View:     &view,
	}
}

// imageJob renders a hand trace as a plan image.
func imageJob(t *testing.T, name string) (trace.Job, *plan.Trace) {
	t.Helper()
	want := readTrace(t, name)
	var buf bytes.Buffer
	if err := png.Encode(&buf, synth.Render(want, synth.Options{Text: true, Furniture: true})); err != nil {
		t.Fatal(err)
	}
	src, err := raster.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	w, h := src.Size()
	return trace.Job{
		Image: src,
		Title: name,
		Meta:  plan.Meta{URL: "generated/x/source.png", Credit: "test", SizePx: [2]int{w, h}},
	}, want
}

// readTrace reads a hand trace: this package's testdata holds the ones
// internal/plan's fixtures lack (see testdata/gen.mjs).
func readTrace(t *testing.T, name string) *plan.Trace {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name+".trace.json"))
	if errors.Is(err, os.ErrNotExist) {
		b, err = os.ReadFile(filepath.Join("..", "plan", "testdata", name+".trace.json"))
	}
	if err != nil {
		t.Fatal(err)
	}
	var tr plan.Trace
	if err := decodeJSON(b, &tr); err != nil {
		t.Fatal(err)
	}
	return &tr
}

func TestDrawingIsReadAsEntities(t *testing.T) {
	job := drawingJob(t, dxftest.Office(4).Bytes())
	opts := trace.Options{Validator: validator(t)}
	res, err := Run(context.Background(), opts, job)
	if err != nil {
		t.Fatalf("%v; problems %q", err, res.Problems)
	}
	if res.Trace.Calibration.PxPerMetre != [2]float64{job.Drawing.PxPerMetre, job.Drawing.PxPerMetre} {
		t.Errorf("the drawing's exact scale is the plan's: %v, want %v",
			res.Trace.Calibration.PxPerMetre, job.Drawing.PxPerMetre)
	}
	names := map[string]bool{}
	for _, r := range res.Trace.Rooms {
		names[r.Name] = true
	}
	for name := range dxftest.OfficeRooms {
		if !names[name] {
			t.Errorf("room %q missing; got %v", name, names)
		}
	}
	if res.Confidence < 0.8 || res.Summary == "" || len(res.Overlay) == 0 || len(res.PlanJSON) == 0 {
		t.Errorf("confidence %.2f, summary %q, overlay %d bytes", res.Confidence, res.Summary, len(res.Overlay))
	}
	if res.CostUSD != 0 || res.Turns != 0 {
		t.Errorf("no model, no cost: %+v", res)
	}
	walls := slices.IndexFunc(res.Layers, func(l profile.Layer) bool { return l.Name == "A-WALL" })
	if walls < 0 || res.Layers[walls].Role != profile.Wall {
		t.Errorf("the layers are reported: %+v", res.Layers)
	}

	// The job's profile reaches the reader: a client who says A-WALL
	// holds nothing gets no walls, and so no trace.
	none, err := profile.NewSet("acme", &profile.Profile{Rules: []profile.Rule{{Role: profile.Ignore, Layer: "A-WALL"}}})
	if err != nil {
		t.Fatal(err)
	}
	ignored := job
	ignored.Profile = none
	failed, err := Run(context.Background(), opts, ignored)
	if err == nil || len(failed.Layers) == 0 {
		t.Errorf("no walls, and still the layers to say why: %v %+v", err, failed.Layers)
	}

	again, err := Run(context.Background(), opts, job)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(res.PlanJSON, again.PlanJSON) || !bytes.Equal(res.Overlay, again.Overlay) {
		t.Error("the same drawing must give the same plan and overlay")
	}
}

func TestImageIsReadFromPixels(t *testing.T) {
	job, want := imageJob(t, "office")
	res, err := Run(context.Background(), trace.Options{Validator: validator(t)}, job)
	if err != nil {
		t.Fatalf("%v; problems %q", err, res.Problems)
	}
	if len(res.Trace.Walls) < len(want.Walls)/2 {
		t.Errorf("%d walls from an image of %d", len(res.Trace.Walls), len(want.Walls))
	}
	again, err := Run(context.Background(), trace.Options{Validator: validator(t)}, job)
	if err != nil || !bytes.Equal(res.PlanJSON, again.PlanJSON) {
		t.Fatalf("the same image must give the same plan: %v", err)
	}
}

func TestExactScaleReachesTheImageReader(t *testing.T) {
	job, _ := imageJob(t, "office")
	job.Drawing = &trace.Drawing{Format: "SVG", PxPerMetre: 40}
	res, err := Run(context.Background(), trace.Options{Validator: verdict{}}, job)
	if err != nil {
		t.Fatal(err)
	}
	if res.Trace.Calibration.PxPerMetre != [2]float64{40, 40} {
		t.Errorf("scale %v", res.Trace.Calibration.PxPerMetre)
	}
}

func TestFeedbackIsAcknowledged(t *testing.T) {
	job, _ := imageJob(t, "office")
	job.Feedback = "the kitchen is missing"
	res, err := Run(context.Background(), trace.Options{Validator: verdict{}}, job)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(res.Caveats, "\n"), "doesn't read notes") {
		t.Errorf("caveats %q", res.Caveats)
	}
}

func TestFailures(t *testing.T) {
	ctx := context.Background()
	ok := trace.Options{Validator: verdict{}}
	if res, err := Run(ctx, ok, trace.Job{}); err == nil || res == nil {
		t.Error("no image")
	}

	job, _ := imageJob(t, "office")
	blank := job
	var buf bytes.Buffer
	if err := png.Encode(&buf, synth.Render(&plan.Trace{}, synth.Options{Size: imageSize(job)})); err != nil {
		t.Fatal(err)
	}
	var err error
	if blank.Image, err = raster.Decode(&buf); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(ctx, ok, blank); err == nil {
		t.Error("a blank image has no plan")
	}

	res, err := Run(ctx, trace.Options{Validator: verdict{problems: []string{"room r1 is outside"}}}, job)
	if !errors.Is(err, trace.ErrNoTrace) || len(res.Problems) != 1 {
		t.Errorf("a rejected plan isn't published: %v %q", err, res.Problems)
	}
	if _, err := Run(ctx, trace.Options{Validator: verdict{err: errors.New("node missing")}}, job); err == nil ||
		!strings.Contains(err.Error(), "node missing") {
		t.Errorf("a broken validator is reported: %v", err)
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := Run(cancelled, ok, job); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled: %v", err)
	}

	missing := drawingJob(t, dxftest.Office(4).Bytes())
	missing.Original = filepath.Join(t.TempDir(), "gone.dxf")
	if _, err := Run(ctx, ok, missing); err == nil || !strings.Contains(err.Error(), "read drawing") {
		t.Errorf("a drawing that has gone: %v", err)
	}
}

func decodeJSON(b []byte, v any) error { return json.Unmarshal(b, v) }

func imageSize(job trace.Job) image.Point {
	w, h := job.Image.Size()
	return image.Pt(w, h)
}
