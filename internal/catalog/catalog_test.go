package catalog

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { c.t = c.t.Add(time.Second); return c.t }

func open(t *testing.T) (*Store, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "generated")
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	c := &clock{t: time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)}
	s.Now = c.now
	return s, dir
}

func image(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestIngestDedupesByContent(t *testing.T) {
	s, dir := open(t)
	e, created, err := s.Ingest(image(t, "Corporate Floor_3.PNG", "pixels"))
	if err != nil || !created {
		t.Fatalf("ingest: %v %v", created, err)
	}
	if e.ID != "corporate-floor-3-"+e.SHA256[:6] || e.Status != Queued || e.Source != "source.png" || e.Title != "Corporate Floor 3" || e.Original != "Corporate Floor_3.PNG" {
		t.Fatalf("entry %+v", e)
	}
	if b, err := os.ReadFile(filepath.Join(dir, e.ID, "source.png")); err != nil || string(b) != "pixels" {
		t.Fatalf("source not copied: %v", err)
	}

	again, created, err := s.Ingest(image(t, "renamed.png", "pixels"))
	if err != nil || created || again.ID != e.ID {
		t.Fatalf("same content under another name is a duplicate: %+v %v %v", again, created, err)
	}
	if _, _, err := s.Ingest(filepath.Join(t.TempDir(), "missing.png")); err == nil {
		t.Fatal("want error for a missing file")
	}
}

func TestCatalogFileIsWhatTheSiteReads(t *testing.T) {
	s, dir := open(t)
	e, _, _ := s.Ingest(image(t, "a.jpg", "x"))
	conf := 0.75
	_ = s.Transition(e.ID, Tracing, nil)
	_ = s.Transition(e.ID, Draft, func(e *Entry) { e.Confidence = &conf; e.Summary = "Small office" })

	b, err := os.ReadFile(filepath.Join(dir, "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Version int              `json:"version"`
		Plans   []map[string]any `json:"plans"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	p := raw.Plans[0]
	// js/plans/index.js reads exactly these keys.
	if raw.Version != 1 || p["id"] != e.ID || p["status"] != "draft" || p["title"] != "A" || p["confidence"] != 0.75 || p["summary"] != "Small office" {
		t.Fatalf("catalog.json: %s", b)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 2 {
		t.Fatalf("no temp or lock files left behind: %v", entries)
	}
}

func TestTransitions(t *testing.T) {
	s, _ := open(t)
	e, _, _ := s.Ingest(image(t, "a.png", "a"))
	steps := []struct {
		to Status
		ok bool
	}{
		{Draft, false}, {Tracing, true}, {Approved, false}, {Draft, true}, {Approved, true},
		{Queued, true}, {Tracing, true}, {Failed, true}, {Draft, false}, {Queued, true},
	}
	for i, st := range steps {
		err := s.Transition(e.ID, st.to, nil)
		if st.ok != (err == nil) {
			t.Fatalf("step %d to %s: %v", i, st.to, err)
		}
		if !st.ok && !errors.Is(err, ErrTransition) {
			t.Fatalf("step %d: want ErrTransition, got %v", i, err)
		}
	}
	if err := s.Transition("nope", Queued, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if _, err := s.Get("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatal("Get of a missing id")
	}
	got, err := s.Get(e.ID)
	if err != nil || got.Status != Queued {
		t.Fatalf("get: %+v %v", got, err)
	}
}

func TestClaimTakesTheOldestAndRecoverRequeues(t *testing.T) {
	s, _ := open(t)
	first, _, _ := s.Ingest(image(t, "first.png", "1"))
	second, _, _ := s.Ingest(image(t, "second.png", "2"))

	got, ok, err := s.Claim()
	if err != nil || !ok || got.ID != first.ID || got.Status != Tracing || got.Attempts != 1 {
		t.Fatalf("claim: %+v %v %v", got, ok, err)
	}
	got, ok, _ = s.Claim()
	if !ok || got.ID != second.ID {
		t.Fatalf("second claim: %+v", got)
	}
	if _, ok, _ := s.Claim(); ok {
		t.Fatal("queue should be empty")
	}

	n, err := s.Recover()
	if err != nil || n != 2 {
		t.Fatalf("recover: %d %v", n, err)
	}
	if got, ok, _ := s.Claim(); !ok || got.Attempts != 2 {
		t.Fatalf("requeued plan counts its attempts: %+v", got)
	}
}

func TestUpdateWritesNothingWhenFnFails(t *testing.T) {
	s, dir := open(t)
	boom := errors.New("boom")
	if err := s.Update(func(*Catalog) error { return boom }); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "catalog.json")); !os.IsNotExist(err) {
		t.Fatal("catalog.json written despite the error")
	}
}

func TestLoadErrors(t *testing.T) {
	s, dir := open(t)
	if err := os.WriteFile(filepath.Join(dir, "catalog.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); err == nil {
		t.Fatal("want parse error")
	}
	if err := s.Update(func(*Catalog) error { return nil }); err == nil {
		t.Fatal("update must not overwrite an unreadable catalogue")
	}
	if _, err := s.Get("x"); err == nil {
		t.Fatal("get on an unreadable catalogue")
	}
	if _, _, err := s.Claim(); err == nil {
		t.Fatal("claim on an unreadable catalogue")
	}
	if err := os.WriteFile(filepath.Join(dir, "catalog.json"), []byte(`{"version":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if c, err := s.Load(); err != nil || c.Plans == nil {
		t.Fatal("a catalogue with no plans key has an empty, non-nil list")
	}

	if err := os.Remove(filepath.Join(dir, "catalog.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "catalog.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); err == nil {
		t.Fatal("want read error when catalog.json is a directory")
	}
}

func TestLocking(t *testing.T) {
	s, dir := open(t)
	s.Now = time.Now
	s.LockWait = 100 * time.Millisecond
	lock := filepath.Join(dir, "catalog.lock")

	if err := os.WriteFile(lock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.Update(func(*Catalog) error { return nil }); !errors.Is(err, ErrLocked) {
		t.Fatalf("want ErrLocked while another process holds it, got %v", err)
	}

	old := time.Now().Add(-time.Minute)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}
	if err := s.Update(func(*Catalog) error { return nil }); err != nil {
		t.Fatalf("a stale lock is taken over: %v", err)
	}

	// Many writers in one process: every append must survive.
	s.LockWait = 5 * time.Second
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Go(func() {
			if _, _, err := s.Ingest(image(t, "p.png", string(rune('a'+i)))); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if c, _ := s.Load(); len(c.Plans) != 20 {
		t.Fatalf("lost updates: %d plans", len(c.Plans))
	}

	ro, _ := open(t)
	if err := os.Chmod(ro.Dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro.Dir, 0o700) })
	if os.Getuid() != 0 {
		if err := ro.Update(func(*Catalog) error { return nil }); err == nil || errors.Is(err, ErrLocked) {
			t.Fatalf("an unwritable directory is an error, not a wait: %v", err)
		}
	}
}

func TestWriteFileErrors(t *testing.T) {
	if err := WriteFile(filepath.Join(t.TempDir(), "missing", "x.json"), nil); err == nil {
		t.Fatal("want error writing into a missing directory")
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "target"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "target", "keep"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(filepath.Join(dir, "target"), []byte("x")); err == nil {
		t.Fatal("want error renaming over a non-empty directory")
	}
	if _, err := Open(filepath.Join(dir, "target", "keep", "sub")); err == nil {
		t.Fatal("want error creating a directory under a file")
	}
}

func TestIngestCannotCreatePlanDir(t *testing.T) {
	s, dir := open(t)
	src := image(t, "x.png", "content")
	sum, _ := hashFile(src)
	// A file where the plan directory should go.
	if err := os.WriteFile(filepath.Join(dir, "x-"+sum[:6]), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Ingest(src); err == nil {
		t.Fatal("want error")
	}
}

func TestSlugAndTitle(t *testing.T) {
	cases := map[string]string{
		"Corporate Floor (v2)": "corporate-floor-v2",
		"--__--":               "plan",
		"Ünïcode plan":         "n-code-plan",
		"a-very-long-name-that-keeps-going-and-going-forever": "a-very-long-name-that-keeps-going-and-go",
		"abcdefghij-abcdefghij-abcdefghij-abcdef-xyz":         "abcdefghij-abcdefghij-abcdefghij-abcdef",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
	if Title("medical-office_layout.png") != "Medical Office Layout" || Title(".png") != "Untitled plan" {
		t.Fatal("Title")
	}
}

func TestReviewAndProgress(t *testing.T) {
	s, _ := open(t)
	e, _, _ := s.Ingest(image(t, "a.png", "a"))
	if _, _, err := s.Review(e.ID, "delete", ""); !errors.Is(err, ErrAction) {
		t.Fatalf("unknown action: %v", err)
	}
	if _, _, err := s.Review(
		e.ID,
		Approve,
		"",
	); !errors.Is(err, ErrTransition) ||
		!strings.Contains(err.Error(), "can't approve") {
		t.Fatalf("approving a queued plan: %v", err)
	}
	if _, _, err := s.Review("nope", Approve, ""); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}

	if err := s.SetProgress(e.ID, Progress{Turn: 1}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(e.ID); got.Progress != nil {
		t.Fatal("progress recorded for a plan that isn't tracing")
	}
	_ = s.Transition(e.ID, Tracing, nil)
	if err := s.SetProgress(e.ID, Progress{Turn: 3, Submits: 1, CostUSD: 0.4}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(e.ID); got.Progress == nil || got.Progress.Turn != 3 {
		t.Fatalf("%+v", got.Progress)
	}
	if err := s.SetProgress("nope", Progress{}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.Recover(); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(e.ID); got.Progress != nil {
		t.Fatal("a requeued plan keeps its old progress")
	}
	_ = s.Transition(e.ID, Tracing, nil)
	_ = s.SetProgress(e.ID, Progress{Turn: 5})
	_ = s.Transition(e.ID, Failed, func(x *Entry) { x.Error = "boom" })
	if got, _ := s.Get(e.ID); got.Progress != nil {
		t.Fatal("a finished plan keeps its progress")
	}

	was, now, err := s.Review(e.ID, Retry, "ignored")
	if err != nil || was != Failed || now.Status != Queued || now.Error != "" || now.Note != "" {
		t.Fatalf("retry: %s %+v %v", was, now, err)
	}
	_ = s.Transition(e.ID, Tracing, nil)
	_ = s.Transition(e.ID, Draft, nil)
	if was, now, err := s.Review(
		e.ID,
		Approve,
		"",
	); err != nil || was != Draft ||
		now.Status != Approved {
		t.Fatalf("approve: %s %+v %v", was, now, err)
	}
	if _, now, err := s.Review(
		e.ID,
		Reject,
		"Add the kitchen",
	); err != nil || now.Status != Queued ||
		now.Note != "Add the kitchen" {
		t.Fatalf("reject: %+v %v", now, err)
	}
}
