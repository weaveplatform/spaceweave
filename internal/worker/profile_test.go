package worker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/autotrace/profile"
	"github.com/weaveplatform/spaceweave/internal/catalog"
	"github.com/weaveplatform/spaceweave/internal/trace"
)

// TestRetraceReadsTheProfile is a layer's role corrected from the page:
// the profile is read afresh for the retrace, and the report says how
// each layer was read, failure or not.
func TestRetraceReadsTheProfile(t *testing.T) {
	store, e := setup(t)
	lib := &profile.Library{Dir: filepath.Join(store.Dir, "profiles")}
	layers := []profile.Layer{{Name: "Q-TRENNWAND", Role: profile.Unknown, Entities: 8, Source: profile.Guess}}
	var got trace.Job
	w := &Worker{Store: store, Profiles: lib, Trace: func(_ context.Context, _ trace.Options, j trace.Job) (*trace.Result, error) {
		got = j
		return &trace.Result{Layers: layers}, errors.New("no walls")
	}}
	if err := w.Process(context.Background(), claim(t, store)); err != nil {
		t.Fatal(err)
	}
	if m, _ := got.Profile.Resolve("Q-TRENNWAND"); m.Role != "" {
		t.Fatalf("no client rule yet: %+v", m)
	}
	var r Report
	readJSON(t, filepath.Join(store.PlanDir(e.ID), ReportFile), &r)
	if r.Status != "failed" || len(r.Layers) != 1 || r.Layers[0].Source != profile.Guess || r.Profile != "" {
		t.Fatalf("a failed trace still lists its layers: %+v", r)
	}

	if err := lib.Put("acme", &profile.Profile{Rules: []profile.Rule{{Role: profile.Wall, Layer: "Q-TRENNWAND"}}}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Review(e.ID, catalog.Retrace, ""); err != nil {
		t.Fatal(err)
	}
	layers = []profile.Layer{{Name: "Q-TRENNWAND", Role: profile.Wall, Entities: 8, Source: "profile:acme"}}
	w.Trace = func(_ context.Context, _ trace.Options, j trace.Job) (*trace.Result, error) {
		got = j
		res := good()
		res.Layers = layers
		return res, nil
	}
	if err := w.Process(context.Background(), claim(t, store)); err != nil {
		t.Fatal(err)
	}
	if m, _ := got.Profile.Resolve("q-trennwand"); m.Role != profile.Wall || m.Source != "profile:acme" {
		t.Fatalf("the retrace reads the saved profile: %+v", m)
	}
	if got.Previous != nil || got.Feedback != "" {
		t.Fatalf("a retrace starts afresh: %+v", got)
	}
	readJSON(t, filepath.Join(store.PlanDir(e.ID), ReportFile), &r)
	if r.Status != "draft" || r.Profile != "acme" || r.Layers[0].Role != profile.Wall {
		t.Fatalf("%+v", r)
	}
}

func TestBrokenProfileFailsTheTrace(t *testing.T) {
	store, e := setup(t)
	dir := filepath.Join(store.Dir, "profiles")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "acme.yaml"), []byte("rules: [{role: wal, layer: x}]"), 0o600); err != nil {
		t.Fatal(err)
	}
	traced := false
	w := &Worker{
		Store: store, Profiles: &profile.Library{Dir: dir, Fixed: "acme"},
		Trace: func(context.Context, trace.Options, trace.Job) (*trace.Result, error) {
			traced = true
			return good(), nil
		},
	}
	if err := w.Process(context.Background(), claim(t, store)); err != nil {
		t.Fatal(err)
	}
	after, _ := store.Get(e.ID)
	if traced || after.Status != catalog.Failed || !strings.Contains(after.Error, `unknown role "wal"`) {
		t.Fatalf("a profile that doesn't parse says so, rather than being ignored: %+v", after)
	}
}
