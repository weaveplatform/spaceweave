// Package inbox watches a folder for floor-plan images and hands each one,
// once it has finished arriving, to an ingest function.
//
// Files of one floor are grouped by name. A file named as a space plan,
// its drawing's stem then "space", "spaces" or "space plan" ("Floor 02
// space.dwg", "Floor 02 - space plan.dxf", "Floor 02.spaces.dwg" for
// "Floor 02.dwg"), is attached to the plan traced from that drawing rather
// than traced on its own: as its space plan when it is a DXF or DWG, as a
// reference otherwise. One that arrives first waits for its drawing, for
// Hold; with no drawing of its stem by then it is a plan of its own, so a
// drawing that is merely called "Open space.dwg" is still traced.
package inbox

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/weaveplatform/spaceweave/internal/convert"
)

// DefaultRescan is how often the whole folder is listed again. fsnotify
// misses events on network and some virtual filesystems, and from editors
// that write through a temporary file, so a slow full listing backs it up.
const DefaultRescan = 30 * time.Second

// DefaultHold is how long a space plan waits for its drawing: long enough
// for the rest of a folder copied in together to arrive.
const DefaultHold = 2 * time.Minute

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
	// Attach adds the file at path, in the given role, to the plan
	// ingested from drawing. Nil ingests every file as a plan of its own.
	Attach func(drawing, path, role string) error
	Hold   time.Duration // DefaultHold when 0
	Log    *slog.Logger
}

// The roles Attach is asked for. They must match catalog.RoleSpaces and
// catalog.RoleReference, which this package doesn't import.
const (
	RoleSpaces    = "spaces"
	RoleReference = "reference"
)

// spaceName is a space plan's file stem: its drawing's stem, a separator,
// and "space", "spaces" or "space plan" in any case and spacing.
var spaceName = regexp.MustCompile(`(?i)^(.*?)[\s._-]+spaces?(?:[\s._-]*plans?)?$`)

// SpacePlanOf says whether name is named as a space plan, the stem of the
// drawing it belongs to, and the role it is attached in: RoleSpaces for a
// DXF or DWG, whose outlines are read, and RoleReference for anything else.
func SpacePlanOf(name string) (stem, role string, ok bool) {
	ext := filepath.Ext(name)
	m := spaceName.FindStringSubmatch(strings.TrimSuffix(name, ext))
	if m == nil || strings.TrimSpace(m[1]) == "" {
		return "", "", false
	}
	role = RoleReference
	if f, ok := convert.ByExt(name); ok && (f.ID == convert.DXF.ID || f.ID == convert.DWG.ID) {
		role = RoleSpaces
	}
	return m[1], role, true
}

// ErrBusy is what Attach returns when the plan can't take a file yet, being
// converted or traced: the file is offered again at the next poll rather
// than the next rescan, so it joins the plan as soon as the run ends.
var ErrBusy = errors.New("inbox: the plan is busy")

// stemOf is a file's name without its extension.
func stemOf(name string) string { return strings.TrimSuffix(name, filepath.Ext(name)) }

type stamp struct {
	size int64
	mod  time.Time
}

// Eligible reports whether name looks like a file the watcher takes: one of
// convert.Formats, not a dotfile.
func Eligible(name string) bool { return convert.Eligible(name) }

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

// watch is one Run's state: what the folder holds and what has been done
// with it.
type watch struct {
	*Watcher
	hold    time.Duration
	pending map[string]*stamp    // last observation; nil until first polled
	done    map[string]stamp     // what was handed over, so rescans skip it
	held    map[string]time.Time // when each space plan was first seen
}

func (w *Watcher) loop(ctx context.Context, events <-chan fsnotify.Event, errs <-chan error) {
	rescan := w.Rescan
	if rescan <= 0 {
		rescan = DefaultRescan
	}
	rescans := time.NewTicker(rescan)
	defer rescans.Stop()
	polls := time.NewTicker(max(w.Settle, minPoll))
	defer polls.Stop()

	s := &watch{
		Watcher: w, hold: w.Hold,
		pending: map[string]*stamp{}, done: map[string]stamp{}, held: map[string]time.Time{},
	}
	if s.hold <= 0 {
		s.hold = DefaultHold
	}
	s.scan()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			s.event(ev)
		case err, ok := <-errs:
			if !ok {
				errs = nil
				continue
			}
			w.Log.Warn("inbox: file events", "err", err)
		case <-rescans.C:
			s.scan()
		case <-polls.C:
			s.poll()
		}
	}
}

func (s *watch) event(ev fsnotify.Event) {
	name := filepath.Base(ev.Name)
	switch {
	case ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename):
		delete(s.done, name)
		// A rename's new name arrives as its own Create.
		if _, err := os.Stat(ev.Name); errors.Is(err, fs.ErrNotExist) {
			delete(s.pending, name)
		}
	case Eligible(name):
		if _, ok := s.pending[name]; !ok {
			s.pending[name] = nil
		}
	}
}

func (s *watch) scan() {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		s.Log.Error("inbox: list folder", "err", err)
		return
	}
	for _, e := range entries {
		if e.Type().IsRegular() && Eligible(e.Name()) {
			if _, ok := s.pending[e.Name()]; !ok {
				s.pending[e.Name()] = nil
			}
		}
	}
}

// spaceFirst orders drawings before space plans, so one arriving with its
// drawing joins its plan in the same poll, then by name, so files arriving
// together are handed over in the same order every time.
func spaceFirst(a, b string) int {
	_, _, sa := SpacePlanOf(a)
	_, _, sb := SpacePlanOf(b)
	switch {
	case sa == sb:
		return strings.Compare(a, b)
	case sb:
		return -1
	default:
		return 1
	}
}

func (s *watch) poll() {
	names := make([]string, 0, len(s.pending))
	for name := range s.pending {
		names = append(names, name)
	}
	slices.SortFunc(names, spaceFirst)
	for _, name := range names {
		prev := s.pending[name]
		info, err := os.Stat(filepath.Join(s.Dir, name))
		if err != nil || !info.Mode().IsRegular() {
			delete(s.pending, name)
			delete(s.held, name)
			continue
		}
		now := stamp{info.Size(), info.ModTime()}
		if d, ok := s.done[name]; ok && d == now {
			delete(s.pending, name)
			continue
		}
		if prev == nil || *prev != now || now.size == 0 {
			s.pending[name] = &now
			continue
		}
		if s.offer(name, now) {
			delete(s.pending, name)
		}
	}
}

// drawingFirst orders a stem's files drawing before image, then by name,
// so the one a space plan joins doesn't depend on listing order.
func drawingFirst(a, b string) int {
	fa, _ := convert.ByExt(a)
	fb, _ := convert.ByExt(b)
	switch {
	case fa.Drawing == fb.Drawing:
		return strings.Compare(a, b)
	case fa.Drawing:
		return -1
	default:
		return 1
	}
}

// drawingOf is the file in the folder a space plan of stem belongs to: an
// eligible file of that stem that isn't a space plan itself.
func (s *watch) drawingOf(stem string) string {
	entries, _ := os.ReadDir(s.Dir)
	var found []string
	for _, e := range entries {
		n := e.Name()
		if _, _, space := SpacePlanOf(n); space || !e.Type().IsRegular() || !Eligible(n) {
			continue
		}
		if strings.EqualFold(stemOf(n), stem) {
			found = append(found, n)
		}
	}
	if len(found) == 0 {
		return ""
	}
	return slices.MinFunc(found, drawingFirst)
}

// offer hands a settled file over, and reports whether it is done with;
// false leaves it pending, to be offered again next poll.
func (s *watch) offer(name string, now stamp) bool {
	stem, role, space := SpacePlanOf(name)
	if space && s.Attach != nil {
		if handled, finished := s.attach(name, stem, role, now); handled {
			return finished
		}
		s.Log.Info("inbox: no drawing for this space plan; tracing it as a plan of its own",
			"file", name, "stem", stem)
	}
	delete(s.held, name)
	if err := s.Ingest(filepath.Join(s.Dir, name)); err != nil {
		s.Log.Error("inbox: ingest", "file", name, "err", err)
		return true
	}
	s.done[name] = now
	// Its space plans go again: a drawing changed is a new plan, which they
	// belong to now.
	for other := range s.done {
		if st, _, ok := SpacePlanOf(
			other,
		); ok && s.Attach != nil &&
			strings.EqualFold(st, stemOf(name)) {
			delete(s.done, other)
			s.pending[other] = nil
		}
	}
	return true
}

// attach offers a space plan to its drawing's plan. handled is false once
// it has waited Hold with no drawing, for it to be ingested on its own.
func (s *watch) attach(name, stem, role string, now stamp) (handled, finished bool) {
	if _, ok := s.held[name]; !ok {
		s.held[name] = time.Now()
	}
	drawing := s.drawingOf(stem)
	if drawing == "" {
		return time.Since(s.held[name]) < s.hold, false
	}
	if _, ingested := s.done[drawing]; !ingested {
		return true, false // its drawing is on its way in
	}
	delete(s.held, name)
	err := s.Attach(filepath.Join(s.Dir, drawing), filepath.Join(s.Dir, name), role)
	if errors.Is(err, ErrBusy) {
		return true, false
	}
	if err != nil {
		// Left out of done, so the next rescan offers it again.
		s.Log.Error("inbox: attach", "file", name, "drawing", drawing, "err", err)
		return true, true
	}
	s.done[name] = now
	return true, true
}
