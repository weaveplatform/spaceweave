// Package inbox watches a folder for floor-plan images and hands each one,
// once it has finished arriving, to an ingest function.
package inbox

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Extensions are the image types the tracer can read.
var Extensions = []string{".png", ".jpg", ".jpeg", ".webp"}

// DefaultRescan is how often the whole folder is listed again. fsnotify
// misses events on network and some virtual filesystems, and from editors
// that write through a temporary file, so a slow full listing backs it up.
const DefaultRescan = 30 * time.Second

// minPoll keeps a zero settle time from becoming a busy loop.
const minPoll = 50 * time.Millisecond

// Watcher watches Dir and calls Ingest for each image once its size and
// modification time are the same on two polls Settle apart, so a file still
// being copied in isn't read half-written. A file is offered again only if
// it changes; Ingest is expected to recognise content it has seen before.
type Watcher struct {
	Dir    string
	Settle time.Duration
	Rescan time.Duration // DefaultRescan when 0
	Ingest func(path string) error
	Log    *slog.Logger
}

type stamp struct {
	size int64
	mod  time.Time
}

// Eligible reports whether name looks like an image the watcher takes.
// Dotfiles are skipped: they are usually partial downloads or OS metadata.
func Eligible(name string) bool {
	base := filepath.Base(name)
	if strings.HasPrefix(base, ".") {
		return false
	}
	return slices.Contains(Extensions, strings.ToLower(filepath.Ext(base)))
}

// newWatcher is swapped in tests to exercise the fallback to rescans.
var newWatcher = fsnotify.NewWatcher

// Run watches until ctx is done.
func (w *Watcher) Run(ctx context.Context) error {
	if w.Log == nil {
		w.Log = slog.New(slog.DiscardHandler)
	}
	if err := os.MkdirAll(w.Dir, 0o750); err != nil {
		return fmt.Errorf("create inbox: %w", err)
	}
	var events <-chan fsnotify.Event
	var errs <-chan error
	fsw, err := newWatcher()
	if err == nil {
		err = fsw.Add(w.Dir)
	}
	if err != nil {
		w.Log.Warn("inbox: file events unavailable, relying on rescans", "err", err)
	} else {
		events, errs = fsw.Events, fsw.Errors
	}
	if fsw != nil {
		defer fsw.Close() //nolint:errcheck // nothing to do with a close error
	}
	w.loop(ctx, events, errs)
	return nil
}

func (w *Watcher) loop(ctx context.Context, events <-chan fsnotify.Event, errs <-chan error) {
	log := w.Log

	rescan := w.Rescan
	if rescan <= 0 {
		rescan = DefaultRescan
	}
	rescans := time.NewTicker(rescan)
	defer rescans.Stop()
	polls := time.NewTicker(max(w.Settle, minPoll))
	defer polls.Stop()

	pending := map[string]*stamp{} // last observation; nil until first polled
	done := map[string]stamp{}     // what was ingested, so rescans skip it
	scan := func() {
		entries, err := os.ReadDir(w.Dir)
		if err != nil {
			log.Error("inbox: list folder", "err", err)
			return
		}
		for _, e := range entries {
			if e.Type().IsRegular() && Eligible(e.Name()) {
				if _, ok := pending[e.Name()]; !ok {
					pending[e.Name()] = nil
				}
			}
		}
	}
	poll := func() {
		for name, prev := range pending {
			path := filepath.Join(w.Dir, name)
			info, err := os.Stat(path)
			if err != nil || !info.Mode().IsRegular() {
				delete(pending, name)
				continue
			}
			now := stamp{info.Size(), info.ModTime()}
			if d, ok := done[name]; ok && d == now {
				delete(pending, name)
				continue
			}
			if prev == nil || *prev != now || now.size == 0 {
				pending[name] = &now
				continue
			}
			delete(pending, name)
			if err := w.Ingest(path); err != nil {
				// Left out of done, so the next rescan offers it again.
				log.Error("inbox: ingest", "file", name, "err", err)
				continue
			}
			done[name] = now
		}
	}

	scan()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			name := filepath.Base(ev.Name)
			switch {
			case ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename):
				delete(done, name)
				// A rename's new name arrives as its own Create.
				if _, err := os.Stat(ev.Name); errors.Is(err, fs.ErrNotExist) {
					delete(pending, name)
				}
			case Eligible(name):
				if _, ok := pending[name]; !ok {
					pending[name] = nil
				}
			}
		case err, ok := <-errs:
			if !ok {
				errs = nil
				continue
			}
			log.Warn("inbox: file events", "err", err)
		case <-rescans.C:
			scan()
		case <-polls.C:
			poll()
		}
	}
}
