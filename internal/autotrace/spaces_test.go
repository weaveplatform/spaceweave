package autotrace

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
	"github.com/weaveplatform/spaceweave/internal/plan"
	"github.com/weaveplatform/spaceweave/internal/trace"
)

func spacesFile(t *testing.T, name string, data []byte) trace.Attachment {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return trace.Attachment{Path: p, Format: "dxf"}
}

func same(x, y float64) (float64, float64) { return x, y }

// TestSpacePlanIsReadAndPlaced is a floor package: the office's drawing
// and its space plan. The space plan is read and placed, report.json's
// account of it is filled in, and its names and numbers become the traced spaces.
func TestSpacePlanIsReadAndPlaced(t *testing.T) {
	ctx := context.Background()
	opts := trace.Options{Validator: validator(t)}
	job := drawingJob(t, dxftest.Office(4).Bytes())
	alone, err := Run(ctx, opts, job)
	if err != nil {
		t.Fatal(err)
	}
	job.Spaces = []trace.Attachment{
		spacesFile(t, "Office space.dxf", dxftest.OfficeSpaces(4, same).Layer("Metadata", 7, 0).
			Add(dxftest.Text("Metadata", 1000, 1000, 200, "False"), dxftest.Text("Metadata", 1100, 1100, 200, "Desks")).
			Bytes()),
	}
	res, err := Run(ctx, opts, job)
	if err != nil {
		t.Fatal(err)
	}
	s := res.Spaces
	if s == nil || s.File != "Office space.dxf" || s.Spaces != len(dxftest.OfficeSpaceList) ||
		s.Method != "shared" ||
		!s.Used ||
		s.Matched < 0.99 ||
		s.Transform == nil ||
		s.Layer != "EC1 Space Polygons" {
		t.Fatalf("%+v", s)
	}
	if slices.Equal(res.PlanJSON, alone.PlanJSON) {
		t.Fatal("registered spaces did not change the plan")
	}
	for _, sp := range dxftest.OfficeSpaceList {
		if !slices.ContainsFunc(
			res.Trace.Rooms,
			func(r plan.Room) bool { return r.Name == sp.Name && r.Number == sp.Number },
		) {
			t.Fatalf("space %q (%s) not preserved", sp.Name, sp.Number)
		}
	}
}

func TestPartialSpacePlan(t *testing.T) {
	job := drawingJob(t, dxftest.Office(4).Bytes())
	job.Spaces = []trace.Attachment{
		spacesFile(
			t,
			"partial.dxf",
			dxftest.SpacePlan(4, same, dxftest.OfficeSpaceList[:2]).Bytes(),
		),
	}
	res, err := Run(context.Background(), trace.Options{Validator: validator(t)}, job)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Team area", "Meeting", "Kitchen", "Director"} {
		if !slices.ContainsFunc(res.Trace.Rooms, func(r plan.Room) bool { return r.Name == name }) {
			t.Fatalf("partial package lost %q: %+v", name, res.Trace.Rooms)
		}
	}
}

func TestSpacePlanProblemsAreCaveats(t *testing.T) {
	ctx := context.Background()
	opts := trace.Options{Validator: validator(t)}
	job := drawingJob(t, dxftest.Office(4).Bytes())

	// Another floor's space plan: placed poorly, so not handed over.
	other := func(x, y float64) (float64, float64) { return 3*y + 40000, 2*x - 90000 }
	job.Spaces = []trace.Attachment{
		spacesFile(t, "other.dxf", dxftest.OfficeSpaces(4, other).Bytes()),
	}
	res, err := Run(ctx, opts, job)
	if err != nil {
		t.Fatal(err)
	}
	if res.Spaces == nil || res.Spaces.Used ||
		!strings.Contains(strings.Join(res.Caveats, " "), "doesn't line up") {
		t.Fatalf("%+v %q", res.Spaces, res.Caveats)
	}

	// A space plan that isn't one: the trace goes on without it.
	job.Spaces = []trace.Attachment{spacesFile(t, "office.dxf", dxftest.Office(4).Bytes())}
	if res, err = Run(ctx, opts, job); err != nil {
		t.Fatal(err)
	}
	if res.Spaces == nil || res.Spaces.Error == "" ||
		!strings.Contains(strings.Join(res.Caveats, " "), "couldn't be read") {
		t.Fatalf("%+v %q", res.Spaces, res.Caveats)
	}

	// An image upload has no drawing to place a space plan on.
	img, _ := imageJob(t, "office")
	img.Spaces = job.Spaces
	if res, err = Run(ctx, opts, img); err != nil {
		t.Fatal(err)
	}
	if res.Spaces == nil || res.Spaces.Error == "" ||
		!strings.Contains(strings.Join(res.Caveats, " "), "was not used") {
		t.Fatalf("%+v %q", res.Spaces, res.Caveats)
	}
}
