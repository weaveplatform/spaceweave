package inbox

import (
	"bytes"
	"context"
	"errors"
	"github.com/fsnotify/fsnotify"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"
)

type recorder struct {
	mu    sync.Mutex
	paths []string
	fail  map[string]int // fail this many times before succeeding
}

func (r *recorder) ingest(path string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := filepath.Base(path)
	if r.fail[name] > 0 {
		r.fail[name]--
		return errors.New("disk full")
	}
	r.paths = append(r.paths, name)
	return nil
}

func (r *recorder) got() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.paths)
}

// syncBuffer is a log sink the test can read while the watcher writes.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) has(s string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return bytes.Contains(b.buf.Bytes(), []byte(s))
}

func start(t *testing.T, w *Watcher) (stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	var once sync.Once
	return func() {
		once.Do(func() {
			cancel()
			if err := <-done; err != nil {
				t.Error(err)
			}
		})
	}
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestEligible(t *testing.T) {
	for name, want := range map[string]bool{
		"plan.png": true, "Plan.JPG": true, "a.jpeg": true, "b.webp": true,
		".plan.png": false, "notes.txt": false, "plan.png.part": false, "gif.gif": false,
	} {
		if Eligible(name) != want {
			t.Errorf("Eligible(%q) = %v", name, !want)
		}
	}
}

func TestIngestsSettledImagesOnce(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "before.png"), []byte("already here"))
	write(t, filepath.Join(dir, "notes.txt"), []byte("not an image"))
	write(t, filepath.Join(dir, ".hidden.png"), []byte("skipped"))
	if err := os.Mkdir(filepath.Join(dir, "folder.png"), 0o750); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{}
	var logs bytes.Buffer
	// Settle is generous next to the 10 ms between chunks so that a scheduler
	// stall under -race can't look like a finished upload.
	w := &Watcher{Dir: dir, Settle: 250 * time.Millisecond, Rescan: 60 * time.Millisecond, Ingest: rec.ingest,
		Log: slog.New(slog.NewTextHandler(&logs, nil))}
	stop := start(t, w)
	defer stop()

	eventually(t, "the existing image", func() bool { return slices.Contains(rec.got(), "before.png") })

	// An upload arriving in pieces is taken once, after it stops growing.
	path := filepath.Join(dir, "upload.webp")
	write(t, path, nil)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	for range 5 {
		if _, err := f.WriteString("chunk"); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
		if slices.Contains(rec.got(), "upload.webp") {
			t.Fatal("taken while still growing")
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the upload", func() bool { return slices.Contains(rec.got(), "upload.webp") })

	// Rescans don't offer it again; a change does.
	time.Sleep(600 * time.Millisecond)
	write(t, path, []byte("a different image"))
	eventually(t, "the changed upload", func() bool {
		n := 0
		for _, p := range rec.got() {
			if p == "upload.webp" {
				n++
			}
		}
		return n == 2
	})
	stop()
	if got := rec.got(); len(got) != 3 {
		t.Fatalf("ingested %v", got)
	}
}

func TestFailedIngestIsRetriedAndRemovalForgets(t *testing.T) {
	dir := t.TempDir()
	rec := &recorder{fail: map[string]int{"flaky.png": 1}}
	var logs bytes.Buffer
	w := &Watcher{Dir: filepath.Join(dir, "inbox"), Settle: 20 * time.Millisecond, Rescan: 50 * time.Millisecond,
		Ingest: rec.ingest, Log: slog.New(slog.NewTextHandler(&logs, nil))}
	stop := start(t, w)
	defer stop()
	eventually(t, "the inbox to be created", func() bool { _, err := os.Stat(w.Dir); return err == nil })

	write(t, filepath.Join(w.Dir, "flaky.png"), []byte("x"))
	eventually(t, "a retry", func() bool { return slices.Contains(rec.got(), "flaky.png") })

	// Removed then dropped in again with the same content: taken again, and
	// left to Ingest to recognise.
	gone := filepath.Join(w.Dir, "flaky.png")
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	write(t, gone, []byte("x"))
	eventually(t, "the re-added file", func() bool { return len(rec.got()) == 2 })

	// A file deleted before it settles is dropped quietly.
	brief := filepath.Join(w.Dir, "brief.png")
	write(t, brief, []byte("y"))
	if err := os.Remove(brief); err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)
	stop()
	if slices.Contains(rec.got(), "brief.png") || !bytes.Contains(logs.Bytes(), []byte("disk full")) {
		t.Fatalf("got %v, logs %s", rec.got(), logs.String())
	}
}

func TestZeroSettleAndDefaults(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.png"), []byte("x"))
	write(t, filepath.Join(dir, "empty.png"), nil)
	rec := &recorder{}
	w := &Watcher{Dir: dir, Ingest: rec.ingest}
	stop := start(t, w)
	defer stop()
	eventually(t, "the image", func() bool { return slices.Contains(rec.got(), "a.png") })
	time.Sleep(100 * time.Millisecond)
	if slices.Contains(rec.got(), "empty.png") {
		t.Fatal("an empty file is still arriving")
	}
}

func TestUnusableFolder(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	write(t, file, nil)
	w := &Watcher{Dir: filepath.Join(file, "inbox"), Ingest: func(string) error { return nil }}
	if err := w.Run(context.Background()); err == nil {
		t.Fatal("expected an error for an inbox under a file")
	}
}

func TestListingErrorsAreLogged(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "inbox")
	var logs syncBuffer
	w := &Watcher{Dir: dir, Settle: 10 * time.Millisecond, Rescan: 20 * time.Millisecond,
		Ingest: func(string) error { return nil }, Log: slog.New(slog.NewTextHandler(&logs, nil))}
	stop := start(t, w)
	eventually(t, "the inbox", func() bool { _, err := os.Stat(dir); return err == nil })
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	eventually(t, "a logged listing error", func() bool { return logs.has("list folder") })
	stop()
}

func TestWithoutFileEvents(t *testing.T) {
	newWatcher = func() (*fsnotify.Watcher, error) { return nil, errors.New("no inotify") }
	defer func() { newWatcher = fsnotify.NewWatcher }()
	dir := t.TempDir()
	rec := &recorder{}
	var logs syncBuffer
	w := &Watcher{Dir: dir, Settle: 10 * time.Millisecond, Rescan: 20 * time.Millisecond, Ingest: rec.ingest,
		Log: slog.New(slog.NewTextHandler(&logs, nil))}
	stop := start(t, w)
	defer stop()
	write(t, filepath.Join(dir, "a.png"), []byte("x"))
	eventually(t, "the image, found by a rescan", func() bool { return slices.Contains(rec.got(), "a.png") })
	if !logs.has("relying on rescans") {
		t.Fatal("the fallback isn't logged")
	}
}

func TestEventErrorsAndClosedChannels(t *testing.T) {
	var logs syncBuffer
	w := &Watcher{Dir: t.TempDir(), Settle: 10 * time.Millisecond, Rescan: time.Hour,
		Ingest: func(string) error { return nil }, Log: slog.New(slog.NewTextHandler(&logs, nil))}
	events, errs := make(chan fsnotify.Event), make(chan error, 1)
	errs <- errors.New("queue overflow")
	close(events)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.loop(ctx, events, errs); close(done) }()
	eventually(t, "the error to be logged", func() bool { return logs.has("queue overflow") })
	close(errs)
	time.Sleep(30 * time.Millisecond)
	cancel()
	<-done
}
