package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/weaveplatform/spaceweave/internal/catalog"
	"github.com/weaveplatform/spaceweave/internal/plan"
	"github.com/weaveplatform/spaceweave/internal/trace"
)

func setup(t *testing.T) (*catalog.Store, catalog.Entry) {
	t.Helper()
	store, err := catalog.Open(filepath.Join(t.TempDir(), "generated"))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 40, 30))); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "Office Plan.png")
	if err := os.WriteFile(src, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	e, _, err := store.Ingest(src)
	if err != nil {
		t.Fatal(err)
	}
	return store, e
}

func claim(t *testing.T, s *catalog.Store) catalog.Entry {
	t.Helper()
	e, ok, err := s.Claim()
	if err != nil || !ok {
		t.Fatalf("claim: %v %v", ok, err)
	}
	return e
}

func good() *trace.Result {
	return &trace.Result{
		Trace:      &plan.Trace{Title: "Office", Calibration: plan.Calibration{PxPerMetre: [2]float64{20, 20}, Evidence: "door leaves"}},
		Plan:       &plan.Plan{Width: 2, Depth: 1.5},
		PlanJSON:   []byte(`{"WIDTH":2,"DEPTH":1.5}`),
		Overlay:    []byte("\x89PNG overlay"),
		Confidence: 0.8, Summary: "A small office", Caveats: []string{"chairs guessed"},
		Turns: 7, Submits: 2, CostUSD: 1.25,
	}
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}

func TestDraftIsPublished(t *testing.T) {
	store, ingested := setup(t)
	var got trace.Job
	var opts trace.Options
	w := &Worker{Store: store, Options: trace.Options{Model: "claude-opus-5-5"},
		Trace: func(_ context.Context, o trace.Options, j trace.Job) (*trace.Result, error) {
			got, opts = j, o
			_, _ = o.Transcript.Write([]byte("{}\n"))
			return good(), nil
		}}
	worked, err := w.Next(context.Background())
	if err != nil || !worked {
		t.Fatal(worked, err)
	}
	e, err := store.Get(ingested.ID)
	if err != nil {
		t.Fatal(err)
	}
	if e.Status != catalog.Draft || *e.Confidence != 0.8 || e.Summary != "A small office" || e.CostUSD != 1.25 {
		t.Fatalf("%+v", e)
	}
	if got.Title != "Office Plan" || got.Meta.URL != "generated/"+e.ID+"/source.png" ||
		got.Meta.Credit != "Uploaded Office Plan.png" || got.Meta.SizePx != [2]int{40, 30} || got.Previous != nil {
		t.Fatalf("job %+v", got)
	}
	if opts.Model != "claude-opus-5-5" || opts.Transcript == nil {
		t.Fatal("options not passed through")
	}
	dir := store.PlanDir(e.ID)
	var p map[string]float64
	readJSON(t, filepath.Join(dir, PlanFile), &p)
	var tr plan.Trace
	readJSON(t, filepath.Join(dir, TraceFile), &tr)
	var r Report
	readJSON(t, filepath.Join(dir, ReportFile), &r)
	if p["WIDTH"] != 2 || tr.Title != "Office" || r.Status != "draft" || r.Calibration != "door leaves" ||
		!strings.HasPrefix(r.Size, "2.00 × 1.50 m") || r.Turns != 7 || r.Model != "claude-opus-5-5" || r.Attempt != 1 {
		t.Fatalf("plan %v trace %+v report %+v", p, tr, r)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, TranscriptFile)); string(b) != "{}\n" {
		t.Fatalf("transcript %q", b)
	}
	if worked, err := w.Next(context.Background()); worked || err != nil {
		t.Fatal("nothing left to do", worked, err)
	}
}

func TestRejectedPlanStartsFromItsTrace(t *testing.T) {
	store, e := setup(t)
	w := &Worker{Store: store, Trace: func(context.Context, trace.Options, trace.Job) (*trace.Result, error) { return good(), nil }}
	if err := w.Process(context.Background(), claim(t, store)); err != nil {
		t.Fatal(err)
	}
	if err := store.Transition(e.ID, catalog.Queued, func(x *catalog.Entry) { x.Note = "Missing the kitchen" }); err != nil {
		t.Fatal(err)
	}
	var got trace.Job
	w.Trace = func(_ context.Context, _ trace.Options, j trace.Job) (*trace.Result, error) {
		got = j
		return good(), nil
	}
	if err := w.Process(context.Background(), claim(t, store)); err != nil {
		t.Fatal(err)
	}
	if got.Feedback != "Missing the kitchen" || got.Previous == nil || got.Previous.Title != "Office" {
		t.Fatalf("%+v", got)
	}
	after, _ := store.Get(e.ID)
	if after.Note != "" || after.Attempts != 2 || after.CostUSD != 2.5 {
		t.Fatalf("%+v", after)
	}
	var r Report
	readJSON(t, filepath.Join(store.PlanDir(e.ID), ReportFile), &r)
	if r.Feedback != "Missing the kitchen" || r.Attempt != 2 {
		t.Fatalf("%+v", r)
	}

	// A note with no earlier trace to build on starts fresh.
	if err := os.Remove(filepath.Join(store.PlanDir(e.ID), TraceFile)); err != nil {
		t.Fatal(err)
	}
	if readTrace(filepath.Join(store.PlanDir(e.ID), TraceFile)) != nil {
		t.Fatal("no trace")
	}
	if err := os.WriteFile(filepath.Join(store.PlanDir(e.ID), TraceFile), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if readTrace(filepath.Join(store.PlanDir(e.ID), TraceFile)) != nil {
		t.Fatal("unreadable trace")
	}
}

func TestFailures(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, s *catalog.Store, e catalog.Entry)
		trace TraceFunc
		want  string
		cost  float64
	}{
		{
			name: "tracer",
			trace: func(context.Context, trace.Options, trace.Job) (*trace.Result, error) {
				return &trace.Result{CostUSD: 3, Stopped: trace.StopSpend, Problems: []string{"room overlaps"}}, trace.ErrNoTrace
			},
			want: trace.ErrNoTrace.Error(), cost: 3,
		},
		{
			name:  "no result",
			trace: func(context.Context, trace.Options, trace.Job) (*trace.Result, error) { return nil, errors.New("boom") },
			want:  "boom",
		},
		{
			name: "bad plan json",
			trace: func(context.Context, trace.Options, trace.Job) (*trace.Result, error) {
				r := good()
				r.PlanJSON = []byte("{")
				return r, nil
			},
			want: "encode plan", cost: 1.25,
		},
		{
			name: "image",
			setup: func(t *testing.T, s *catalog.Store, e catalog.Entry) {
				if err := os.WriteFile(filepath.Join(s.PlanDir(e.ID), e.Source), []byte("not a png"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			want: "read image",
		},
		{
			name: "transcript",
			setup: func(t *testing.T, s *catalog.Store, e catalog.Entry) {
				if err := os.Mkdir(filepath.Join(s.PlanDir(e.ID), TranscriptFile), 0o750); err != nil {
					t.Fatal(err)
				}
			},
			want: "create transcript",
		},
		{
			name: "output file",
			setup: func(t *testing.T, s *catalog.Store, e catalog.Entry) {
				if err := os.Mkdir(filepath.Join(s.PlanDir(e.ID), OverlayFile), 0o750); err != nil {
					t.Fatal(err)
				}
			},
			want: "overlay.png", cost: 1.25,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			store, e := setup(t)
			if c.setup != nil {
				c.setup(t, store, e)
			}
			tr := c.trace
			if tr == nil {
				tr = func(context.Context, trace.Options, trace.Job) (*trace.Result, error) { return good(), nil }
			}
			w := &Worker{Store: store, Trace: tr}
			if err := w.Process(context.Background(), claim(t, store)); err != nil {
				t.Fatal("a failed trace is recorded, not returned:", err)
			}
			after, _ := store.Get(e.ID)
			if after.Status != catalog.Failed || !strings.Contains(after.Error, c.want) || after.CostUSD != c.cost {
				t.Fatalf("%+v", after)
			}
			var r Report
			readJSON(t, filepath.Join(store.PlanDir(e.ID), ReportFile), &r)
			if r.Status != "failed" || !strings.Contains(r.Error, c.want) {
				t.Fatalf("%+v", r)
			}
		})
	}
}

func TestShutdownRequeues(t *testing.T) {
	store, e := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	w := &Worker{Store: store, Trace: func(context.Context, trace.Options, trace.Job) (*trace.Result, error) {
		cancel()
		return &trace.Result{CostUSD: 0.5, Stopped: trace.StopTimeout}, nil
	}}
	if err := w.Process(ctx, claim(t, store)); err != nil {
		t.Fatal(err)
	}
	after, _ := store.Get(e.ID)
	if after.Status != catalog.Queued || after.CostUSD != 0.5 {
		t.Fatalf("%+v", after)
	}
}

func TestRunRecoversAndWakes(t *testing.T) {
	store, first := setup(t)
	claim(t, store) // left tracing, as if the last daemon died mid-run

	var mu sync.Mutex
	done := map[string]bool{}
	var logs bytes.Buffer
	var logMu sync.Mutex
	w := &Worker{
		Store: store, Poll: time.Hour,
		Log: slog.New(slog.NewTextHandler(writerFunc(func(p []byte) (int, error) {
			logMu.Lock()
			defer logMu.Unlock()
			return logs.Write(p)
		}), nil)),
		Trace: func(_ context.Context, _ trace.Options, j trace.Job) (*trace.Result, error) {
			mu.Lock()
			defer mu.Unlock()
			done[j.Meta.URL] = true
			return good(), nil
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	wake := make(chan struct{}, 1)
	exited := make(chan error, 1)
	go func() { exited <- w.Run(ctx, 2, wake) }()

	waitFor(t, func() bool { e, _ := store.Get(first.ID); return e.Status == catalog.Draft })

	// A new image, then a wake: picked up without waiting for the hour-long poll.
	var buf bytes.Buffer
	_ = png.Encode(&buf, image.NewGray(image.Rect(0, 0, 8, 8)))
	src := filepath.Join(t.TempDir(), "second.png")
	if err := os.WriteFile(src, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	second, _, err := store.Ingest(src)
	if err != nil {
		t.Fatal(err)
	}
	wake <- struct{}{}
	waitFor(t, func() bool { e, _ := store.Get(second.ID); return e.Status == catalog.Draft })

	cancel()
	if err := <-exited; err != nil {
		t.Fatal(err)
	}
	logMu.Lock()
	defer logMu.Unlock()
	if !strings.Contains(logs.String(), "requeued plans interrupted") {
		t.Fatal(logs.String())
	}
}

func TestCatalogErrors(t *testing.T) {
	store, _ := setup(t)
	if err := os.WriteFile(filepath.Join(store.Dir, "catalog.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	w := &Worker{Store: store}
	if _, err := w.Next(context.Background()); err == nil {
		t.Fatal("expected a catalogue error")
	}
	if err := w.Run(context.Background(), 1, nil); err == nil {
		t.Fatal("expected Run to fail on recovery")
	}
}

func TestRunLogsAndCarriesOn(t *testing.T) {
	store, e := setup(t)
	var logs bytes.Buffer
	var mu sync.Mutex
	has := func(s string) bool {
		mu.Lock()
		defer mu.Unlock()
		return strings.Contains(logs.String(), s)
	}
	w := &Worker{Store: store, Poll: 10 * time.Millisecond,
		Trace: func(context.Context, trace.Options, trace.Job) (*trace.Result, error) { return good(), nil },
		Log: slog.New(slog.NewTextHandler(writerFunc(func(p []byte) (int, error) {
			mu.Lock()
			defer mu.Unlock()
			return logs.Write(p)
		}), nil))}
	ctx, cancel := context.WithCancel(context.Background())
	exited := make(chan error, 1)
	go func() { exited <- w.Run(ctx, 1, nil) }()
	// The catalogue, not the log: "draft ready" is logged just before the
	// write that would replace the corrupt file planted below.
	waitFor(t, func() bool { x, _ := store.Get(e.ID); return x.Status == catalog.Draft })
	if err := os.WriteFile(filepath.Join(store.Dir, "catalog.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return has("read catalog") })
	cancel()
	if err := <-exited; err != nil {
		t.Fatal(err)
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
