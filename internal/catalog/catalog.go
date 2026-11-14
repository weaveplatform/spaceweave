// Package catalog owns the tracer's output directory: catalog.json, which
// the microsite polls to fill its plan picker, and one directory per plan
// holding the source image, the trace, the plan and the tracer's evidence.
//
// The daemon and the `spaceweave plans` commands are separate processes that
// both edit catalog.json, so every change is a read-modify-write under a lock
// file, written to a temporary file and renamed into place so the site never
// reads half a file.
package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// Status is where a plan is in its life.
type Status string

// The statuses. Their string values are read by js/plans/index.js.
const (
	Queued   Status = "queued"
	Tracing  Status = "tracing"
	Draft    Status = "draft"
	Approved Status = "approved"
	Failed   Status = "failed"
)

// transitions lists the allowed moves. Rejecting a draft (or an approved plan)
// and retrying a failure both send it back to the queue.
var transitions = map[Status][]Status{
	Queued:   {Tracing},
	Tracing:  {Draft, Failed, Queued},
	Draft:    {Approved, Queued},
	Approved: {Queued},
	Failed:   {Queued},
}

// Errors.
var (
	ErrNotFound   = errors.New("no such plan")
	ErrTransition = errors.New("status change not allowed")
	ErrLocked     = errors.New("catalog is locked by another process")
)

// Entry is one plan in catalog.json.
type Entry struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Status     Status    `json:"status"`
	Source     string    `json:"source"`   // file name inside the plan's directory
	Original   string    `json:"original"` // the inbox file it came from
	SHA256     string    `json:"sha256"`
	Confidence *float64  `json:"confidence,omitempty"`
	Summary    string    `json:"summary,omitempty"`
	CostUSD    float64   `json:"costUsd"`
	Attempts   int       `json:"attempts"`
	Note       string    `json:"note,omitempty"` // reviewer feedback for the next attempt
	Error      string    `json:"error,omitempty"`
	Progress   *Progress `json:"progress,omitempty"` // set only while tracing
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Progress is how far a trace has got, for the site's pipeline view.
type Progress struct {
	Turn    int     `json:"turn"`
	Submits int     `json:"submits"`
	CostUSD float64 `json:"costUsd"`
}

// Catalog is catalog.json.
type Catalog struct {
	Version int     `json:"version"`
	Plans   []Entry `json:"plans"`
}

func (c *Catalog) find(id string) (*Entry, error) {
	for i := range c.Plans {
		if c.Plans[i].ID == id {
			return &c.Plans[i], nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
}

// Store is the output directory.
type Store struct {
	Dir string
	Now func() time.Time
	// LockWait bounds how long an update waits for another process; a lock
	// file older than LockStale is assumed to belong to a crashed process.
	LockWait, LockStale time.Duration

	mu sync.Mutex
}

// Open prepares the output directory.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create output directory: %w", err)
	}
	return &Store{
		Dir:       dir,
		Now:       time.Now,
		LockWait:  10 * time.Second,
		LockStale: 30 * time.Second,
	}, nil
}

func (s *Store) path() string { return filepath.Join(s.Dir, "catalog.json") }

// PlanDir is the directory holding one plan's files.
func (s *Store) PlanDir(id string) string { return filepath.Join(s.Dir, id) }

// Load reads the catalogue; a missing file is an empty catalogue.
func (s *Store) Load() (*Catalog, error) {
	b, err := os.ReadFile(s.path())
	if errors.Is(err, fs.ErrNotExist) {
		return &Catalog{Version: 1, Plans: []Entry{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read catalog: %w", err)
	}
	var c Catalog
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", s.path(), err)
	}
	if c.Plans == nil {
		c.Plans = []Entry{}
	}
	return &c, nil
}

// Get returns one entry.
func (s *Store) Get(id string) (Entry, error) {
	c, err := s.Load()
	if err != nil {
		return Entry{}, err
	}
	e, err := c.find(id)
	if err != nil {
		return Entry{}, err
	}
	return *e, nil
}

// Update applies fn to the catalogue and saves it, holding the lock
// throughout. Nothing is written if fn fails.
func (s *Store) Update(fn func(*Catalog) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()

	c, err := s.Load()
	if err != nil {
		return err
	}
	if err := fn(c); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", " ")
	if err != nil {
		return fmt.Errorf("encode catalog: %w", err)
	}
	return WriteFile(s.path(), append(b, '\n'))
}

// lock takes the cross-process lock file.
func (s *Store) lock() (func(), error) {
	path := filepath.Join(s.Dir, "catalog.lock")
	deadline := s.Now().Add(s.LockWait)
	for {
		f, err := os.OpenFile(
			path,
			os.O_CREATE|os.O_EXCL|os.O_WRONLY,
			0o600,
		) //nolint:gosec // fixed name in our own directory
		if err == nil {
			_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
			_ = f.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("lock catalog: %w", err)
		}
		if info, statErr := os.Stat(
			path,
		); statErr == nil &&
			s.Now().Sub(info.ModTime()) > s.LockStale {
			_ = os.Remove(path)
			continue
		}
		if s.Now().After(deadline) {
			return nil, ErrLocked
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// Transition moves a plan to a new status, applying mutate to the entry in
// the same locked update.
func (s *Store) Transition(id string, to Status, mutate func(*Entry)) error {
	return s.Update(func(c *Catalog) error {
		e, err := c.find(id)
		if err != nil {
			return err
		}
		if !slices.Contains(transitions[e.Status], to) {
			return fmt.Errorf("%w: %s is %s, can't become %s", ErrTransition, id, e.Status, to)
		}
		e.Status = to
		e.UpdatedAt = s.Now().UTC()
		e.Progress = nil
		if mutate != nil {
			mutate(e)
		}
		return nil
	})
}

// Claim takes the oldest queued plan and marks it tracing. ok is false when
// the queue is empty.
func (s *Store) Claim() (Entry, bool, error) {
	var claimed Entry
	found := false
	err := s.Update(func(c *Catalog) error {
		var oldest *Entry
		for i := range c.Plans {
			e := &c.Plans[i]
			if e.Status == Queued && (oldest == nil || e.UpdatedAt.Before(oldest.UpdatedAt)) {
				oldest = e
			}
		}
		if oldest == nil {
			return nil
		}
		oldest.Status = Tracing
		oldest.Attempts++
		oldest.UpdatedAt = s.Now().UTC()
		claimed, found = *oldest, true
		return nil
	})
	return claimed, found, err
}

// SetProgress records how far a trace has got. It does nothing once the plan
// has left tracing, so a late update can't resurrect a finished run's progress.
func (s *Store) SetProgress(id string, p Progress) error {
	return s.Update(func(c *Catalog) error {
		e, err := c.find(id)
		if err != nil {
			return err
		}
		if e.Status == Tracing {
			e.Progress = &p
		}
		return nil
	})
}

// Action is a reviewer's decision on a plan.
type Action string

// The review actions, shared by `spaceweave plans` and the site's buttons.
const (
	Approve Action = "approve"
	Reject  Action = "reject"
	Retry   Action = "retry"
)

// reviews narrows the transitions to what each action means: reject and
// retry both re-queue, but rejecting a failure or retrying a draft is a
// mistake worth saying so.
var reviews = map[Action]struct {
	from []Status
	to   Status
}{
	Approve: {[]Status{Draft}, Approved},
	Reject:  {[]Status{Draft, Approved}, Queued},
	Retry:   {[]Status{Failed}, Queued},
}

// ErrAction is an unknown review action.
var ErrAction = errors.New("unknown review action")

// Review applies a reviewer's action and returns the status the plan had and
// the entry as it is now. A rejection's note goes to the next attempt.
func (s *Store) Review(id string, a Action, note string) (Status, Entry, error) {
	r, ok := reviews[a]
	if !ok {
		return "", Entry{}, fmt.Errorf("%w: %q", ErrAction, a)
	}
	var was Status
	var now Entry
	err := s.Update(func(c *Catalog) error {
		e, err := c.find(id)
		if err != nil {
			return err
		}
		if !slices.Contains(r.from, e.Status) {
			return fmt.Errorf("%w: %s is %s, can't %s it", ErrTransition, id, e.Status, a)
		}
		was = e.Status
		e.Status = r.to
		e.UpdatedAt = s.Now().UTC()
		e.Progress = nil
		switch a {
		case Reject:
			e.Note = note
		case Retry:
			e.Error = ""
		case Approve:
		}
		now = *e
		return nil
	})
	return was, now, err
}

// Recover requeues plans left tracing by a worker that stopped mid-run, so
// they aren't stuck forever. Call it before starting workers.
func (s *Store) Recover() (int, error) {
	n := 0
	err := s.Update(func(c *Catalog) error {
		for i := range c.Plans {
			if c.Plans[i].Status == Tracing {
				c.Plans[i].Status = Queued
				c.Plans[i].UpdatedAt = s.Now().UTC()
				c.Plans[i].Progress = nil
				n++
			}
		}
		return nil
	})
	return n, err
}

// Ingest adds an image to the queue: it hashes the file, and unless an entry
// with the same content exists, copies it into a new plan directory. created
// is false for a duplicate, with the existing entry returned.
func (s *Store) Ingest(path string) (Entry, bool, error) {
	sum, err := hashFile(path)
	if err != nil {
		return Entry{}, false, err
	}
	var entry Entry
	created := false
	err = s.Update(func(c *Catalog) error {
		for _, e := range c.Plans {
			if e.SHA256 == sum {
				entry = e
				return nil
			}
		}
		base := filepath.Base(path)
		ext := strings.ToLower(filepath.Ext(base))
		id := Slug(strings.TrimSuffix(base, filepath.Ext(base))) + "-" + sum[:6]
		dir := s.PlanDir(id)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create plan directory: %w", err)
		}
		if err := copyFile(path, filepath.Join(dir, "source"+ext)); err != nil {
			return err
		}
		now := s.Now().UTC()
		entry = Entry{
			ID: id, Title: Title(base), Status: Queued, Source: "source" + ext, Original: base,
			SHA256: sum, CreatedAt: now, UpdatedAt: now,
		}
		c.Plans = append(c.Plans, entry)
		created = true
		return nil
	})
	return entry, created, err
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // inbox files are the input
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close() //nolint:errcheck // read-only
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFile(from, to string) error {
	b, err := os.ReadFile(from) //nolint:gosec // inbox files are the input
	if err != nil {
		return fmt.Errorf("read %s: %w", from, err)
	}
	return WriteFile(to, b)
}

// WriteFile writes data to path via a temporary file and a rename, so a
// reader sees the old content or the new, never a partial file.
func WriteFile(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck // gone after a successful rename
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// Slug makes a file name safe for a URL path segment.
func Slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		switch {
		case ok:
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	s := strings.TrimRight(b.String(), "-")
	if len(s) > 40 {
		s = strings.TrimRight(s[:40], "-")
	}
	if s == "" {
		return "plan"
	}
	return s
}

// Title is a readable first guess at a plan's name from its file name; the
// tracer replaces it with what the drawing says.
func Title(file string) string {
	base := strings.TrimSuffix(file, filepath.Ext(file))
	words := strings.FieldsFunc(
		base,
		func(r rune) bool { return r == '-' || r == '_' || r == ' ' || r == '.' },
	)
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	if len(words) == 0 {
		return "Untitled plan"
	}
	return strings.Join(words, " ")
}
