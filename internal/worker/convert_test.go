package worker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/catalog"
	"github.com/weaveplatform/spaceweave/internal/convert"
	"github.com/weaveplatform/spaceweave/internal/dwg/dwgtest"
	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
	"github.com/weaveplatform/spaceweave/internal/trace"
)

func ingestDrawing(t *testing.T, name string, b []byte) (*catalog.Store, catalog.Entry) {
	t.Helper()
	store, err := catalog.Open(filepath.Join(t.TempDir(), "generated"))
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(src, b, 0o600); err != nil {
		t.Fatal(err)
	}
	e, _, err := store.Ingest(src)
	if err != nil {
		t.Fatal(err)
	}
	return store, e
}

func TestDrawingIsConvertedThenTracedWithItsScale(t *testing.T) {
	store, e := ingestDrawing(t, "Office.dxf", dxftest.Plan().Bytes())
	var jobs []trace.Job
	w := &Worker{
		Store: store, Convert: convert.Options{MaxSide: 800},
		Trace: func(_ context.Context, _ trace.Options, j trace.Job) (*trace.Result, error) {
			jobs = append(jobs, j)
			return good(), nil
		},
	}
	if worked, err := w.Next(context.Background()); !worked || err != nil {
		t.Fatal(worked, err)
	}
	got, _ := store.Get(e.ID)
	if got.Status != catalog.Draft || got.Source != convert.ImageFile ||
		got.Drawing != convert.DrawingFile ||
		!got.Scale.Exact() {
		t.Fatalf("%+v", got)
	}
	dir := store.PlanDir(e.ID)
	for _, f := range []string{"original.dxf", convert.ImageFile, convert.DrawingFile} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Error(err)
		}
	}
	j := jobs[0]
	if j.Drawing == nil || j.Drawing.Format != "DXF" ||
		j.Drawing.PxPerMetre != got.Scale.PxPerMetre ||
		j.Drawing.Assumed {
		t.Fatalf("job drawing %+v", j.Drawing)
	}
	if j.Meta.Credit != "Uploaded Office.dxf, rendered from DXF" ||
		j.Meta.URL != "generated/"+e.ID+"/source.png" {
		t.Fatalf("meta %+v", j.Meta)
	}
	var r Report
	readJSON(t, filepath.Join(dir, ReportFile), &r)
	if r.Conversion == nil || r.Conversion.Format != "DXF" || r.Conversion.Width != 800 ||
		!strings.Contains(r.Conversion.Scale, "drawing's units") {
		t.Fatalf("report conversion %+v", r.Conversion)
	}

	// Sent back: traced again from the same image, without converting, and
	// the job still knows the drawing's scale.
	if err := store.Transition(
		e.ID,
		catalog.Queued,
		func(x *catalog.Entry) { x.Note = "missing a door" },
	); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "original.dxf")); err != nil {
		t.Fatal(err)
	}
	if worked, err := w.Next(context.Background()); !worked || err != nil {
		t.Fatal(worked, err)
	}
	if j := jobs[1]; j.Drawing == nil || j.Drawing.PxPerMetre != got.Scale.PxPerMetre ||
		j.Feedback != "missing a door" {
		t.Fatalf("retrace %+v", j.Drawing)
	}
}

// A DWG goes the same way as a DXF: its original kept beside the image,
// and the job told the scale and handed the original and its view, which
// is what lets the CAD tracer read the entities.
func TestDWGIsConvertedThenTracedWithItsScale(t *testing.T) {
	store, e := ingestDrawing(t, "Office.dwg", dwgtest.Office("AC1018"))
	var job trace.Job
	w := &Worker{
		Store: store, Convert: convert.Options{MaxSide: 800},
		Trace: func(_ context.Context, _ trace.Options, j trace.Job) (*trace.Result, error) {
			job = j
			return good(), nil
		},
	}
	if worked, err := w.Next(context.Background()); !worked || err != nil {
		t.Fatal(worked, err)
	}
	got, _ := store.Get(e.ID)
	dir := store.PlanDir(e.ID)
	if got.Status != catalog.Draft || got.Format != convert.DWG.ID ||
		got.Upload != "original.dwg" || !got.Scale.Exact() {
		t.Fatalf("%+v", got)
	}
	if job.Drawing == nil || job.Drawing.Format != "DWG" ||
		job.Drawing.PxPerMetre != got.Scale.PxPerMetre ||
		job.Original != filepath.Join(dir, "original.dwg") || job.View == nil ||
		job.Meta.Credit != "Uploaded Office.dwg, rendered from DWG" {
		t.Fatalf("job %+v", job)
	}
}

func TestBadDrawingFailsWithoutTracing(t *testing.T) {
	store, e := ingestDrawing(t, "empty.dxf", dxftest.New(4).Bytes())
	w := &Worker{
		Store: store,
		Trace: func(context.Context, trace.Options, trace.Job) (*trace.Result, error) {
			t.Fatal("a drawing that didn't convert isn't traced")
			return nil, nil //nolint:nilnil // unreachable
		},
	}
	if _, err := w.Next(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, _ := store.Get(e.ID)
	if got.Status != catalog.Failed || !strings.HasPrefix(got.Error, "convert empty.dxf:") ||
		got.Source != "" {
		t.Fatalf("%+v", got)
	}
}

func TestShutdownDuringConversionRequeues(t *testing.T) {
	store, e := ingestDrawing(t, "Office.dxf", dxftest.Plan().Bytes())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w := &Worker{Store: store}
	if _, err := w.Next(ctx); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.Get(e.ID); got.Status != catalog.Queued {
		t.Fatalf("%+v", got)
	}
}

func TestReadConversion(t *testing.T) {
	dir := t.TempDir()
	if readConversion(filepath.Join(dir, "absent.json")) != nil {
		t.Fatal("no report, no conversion")
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if readConversion(bad) != nil {
		t.Fatal("a broken report is ignored")
	}
}
