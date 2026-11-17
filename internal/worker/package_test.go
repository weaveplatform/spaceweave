package worker

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/catalog"
	"github.com/weaveplatform/spaceweave/internal/trace"
)

// TestPackageFilesReachTheTracer is a plan with a space plan and a
// reference attached: the tracer is handed both where they lie in the
// plan's folder, and report.json says how the space plan was read.
func TestPackageFilesReachTheTracer(t *testing.T) {
	store, e := setup(t)
	traced := func(context.Context, trace.Options, trace.Job) (*trace.Result, error) { return good(), nil }
	w := &Worker{Store: store, Trace: traced}
	if err := w.Process(context.Background(), claim(t, store)); err != nil {
		t.Fatal(err)
	}
	for _, f := range []struct{ name, role string }{
		{"Office space.dwg", catalog.RoleSpaces},
		{"print.png", catalog.RoleReference},
	} {
		p := filepath.Join(t.TempDir(), f.name)
		if err := os.WriteFile(p, []byte(f.name), 0o600); err != nil {
			t.Fatal(err)
		}
		if f.role == catalog.RoleSpaces {
			// A space plan needs a drawing to go on; this plan's is an
			// image, so the catalogue is told it isn't, as a test.
			if err := store.Update(func(c *catalog.Catalog) error {
				c.Plans[0].Format = "dxf"
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		}
		if _, _, err := store.Attach(e.ID, p, f.role); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Update(func(c *catalog.Catalog) error {
		c.Plans[0].Format = "png"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	var got trace.Job
	read := &trace.Spaces{
		File:    "Office space.dwg",
		Spaces:  5,
		Method:  "shared",
		Matched: 0.9,
		Used:    true,
	}
	w.Trace = func(_ context.Context, _ trace.Options, j trace.Job) (*trace.Result, error) {
		got = j
		res := good()
		res.Spaces = read
		return res, nil
	}
	c := claim(t, store)
	if c.Status != catalog.Tracing {
		t.Fatalf("a retrace with its image already there isn't converted: %+v", c)
	}
	if err := w.Process(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	dir := store.PlanDir(e.ID)
	if len(got.Spaces) != 1 || len(got.References) != 1 || got.Spaces[0].Format != "dwg" ||
		got.References[0].Format != "png" {
		t.Fatalf("%+v %+v", got.Spaces, got.References)
	}
	for _, a := range append(got.Spaces, got.References...) {
		if filepath.Dir(filepath.Dir(a.Path)) != dir {
			t.Fatalf("%s is not in the plan's files folder", a.Path)
		}
		if _, err := os.Stat(a.Path); err != nil {
			t.Fatal(err)
		}
	}
	var r Report
	readJSON(t, filepath.Join(dir, ReportFile), &r)
	if r.Spaces == nil || r.Spaces.Spaces != 5 || !r.Spaces.Used {
		t.Fatalf("%+v", r.Spaces)
	}
}
