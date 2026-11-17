package inbox

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestSpacePlanOf(t *testing.T) {
	for name, want := range map[string][2]string{
		"Floor 02 space.dwg":          {"Floor 02", RoleSpaces},
		"Floor 02 - space plan.dxf":   {"Floor 02", RoleSpaces},
		"Floor 02.spaces.dwg":         {"Floor 02", RoleSpaces},
		"floor_02_SPACE_PLANS.DWG":    {"floor_02", RoleSpaces},
		"Floor 02-spaceplan.dxf":      {"Floor 02", RoleSpaces},
		"Floor 02 space.png":          {"Floor 02", RoleReference},
		"Floor 02.dwg":                {"", ""},
		"Spaces.dwg":                  {"", ""},
		" space.dwg":                  {"", ""},
		"Floor 02 spaceship.dwg":      {"", ""},
		"Floor 02 workspace.dwg":      {"", ""},
		"Floor 02 space plan v2.dwg":  {"", ""},
		"Floor 02 - Space Plan .dwg":  {"", ""},
		"Floor 02 open space plan.dw": {"Floor 02 open", RoleReference},
	} {
		stem, role, ok := SpacePlanOf(name)
		if ok != (want[0] != "") || stem != want[0] || role != want[1] {
			t.Errorf("SpacePlanOf(%q) = %q %q %v, want %q", name, stem, role, ok, want)
		}
	}
}

// packager records what the watcher hands over.
type packager struct {
	mu       sync.Mutex
	ingested []string
	attached [][3]string
	busy     int   // refuse this many attaches as busy first
	fail     error // refuse every attach with this
}

func (p *packager) ingest(path string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ingested = append(p.ingested, filepath.Base(path))
	return nil
}

func (p *packager) attach(drawing, path, role string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.busy > 0 {
		p.busy--
		return ErrBusy
	}
	if p.fail != nil {
		return p.fail
	}
	p.attached = append(p.attached, [3]string{filepath.Base(drawing), filepath.Base(path), role})
	return nil
}

func (p *packager) got() ([]string, [][3]string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.ingested), slices.Clone(p.attached)
}

func (p *packager) watch(t *testing.T, dir string, hold time.Duration) (*syncBuffer, func()) {
	t.Helper()
	logs := &syncBuffer{}
	w := &Watcher{
		Dir: dir, Settle: 20 * time.Millisecond, Rescan: 50 * time.Millisecond, Hold: hold,
		Ingest: p.ingest, Attach: p.attach, Log: slog.New(slog.NewTextHandler(logs, nil)),
	}
	return logs, start(t, w)
}

// TestSpacePlansJoinTheirDrawing is a floor's files dropped together: the
// drawing becomes a plan, and its space plan and a print of it are
// attached to that plan rather than traced on their own.
func TestSpacePlansJoinTheirDrawing(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "Floor 02 space.dwg"), []byte("spaces"))
	write(t, filepath.Join(dir, "Floor 02.spaces.png"), []byte("print"))
	write(t, filepath.Join(dir, "Floor 02.png"), []byte("a picture of it"))
	write(t, filepath.Join(dir, "Floor 02.dwg"), []byte("drawing"))
	p := &packager{busy: 2}
	_, stop := p.watch(t, dir, time.Hour)
	defer stop()
	eventually(t, "the attachments", func() bool { _, a := p.got(); return len(a) == 2 })
	ingested, attached := p.got()
	// Both drawing and picture are plans; the space plan goes with the
	// drawing, not the picture, and waited out the busy plan.
	if !slices.Equal(ingested, []string{"Floor 02.dwg", "Floor 02.png"}) {
		t.Fatalf("%q", ingested)
	}
	want := [][3]string{
		{"Floor 02.dwg", "Floor 02 space.dwg", RoleSpaces},
		{"Floor 02.dwg", "Floor 02.spaces.png", RoleReference},
	}
	if !slices.Equal(attached, want) {
		t.Fatalf("%q", attached)
	}

	// The drawing changes: a new plan, which its space plan joins.
	write(t, filepath.Join(dir, "Floor 02.dwg"), []byte("drawing, revised"))
	eventually(
		t,
		"the space plan joining the new drawing",
		func() bool { _, a := p.got(); return len(a) == 4 },
	)
	if ingested, _ := p.got(); len(ingested) != 3 {
		t.Fatalf("%q", ingested)
	}
}

// TestSpacePlanWaitsForItsDrawing is a space plan copied in before its
// drawing: it waits, and joins the drawing's plan once there is one.
func TestSpacePlanWaitsForItsDrawing(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "L3 - space plan.dxf"), []byte("spaces"))
	p := &packager{}
	_, stop := p.watch(t, dir, time.Hour)
	defer stop()
	time.Sleep(200 * time.Millisecond)
	if ingested, attached := p.got(); len(ingested)+len(attached) != 0 {
		t.Fatalf("held: %q %q", ingested, attached)
	}
	write(t, filepath.Join(dir, "L3.dxf"), []byte("drawing"))
	eventually(t, "the attachment", func() bool { _, a := p.got(); return len(a) == 1 })
	if ingested, attached := p.got(); !slices.Equal(ingested, []string{"L3.dxf"}) ||
		attached[0] != [3]string{"L3.dxf", "L3 - space plan.dxf", RoleSpaces} {
		t.Fatalf("%q %q", ingested, attached)
	}
}

// TestSpacePlanWithoutADrawing is a file only named like a space plan:
// once Hold passes with no drawing of its stem, it is a plan of its own.
func TestSpacePlanWithoutADrawing(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "Open space.dwg"), []byte("a drawing"))
	p := &packager{}
	logs, stop := p.watch(t, dir, 100*time.Millisecond)
	defer stop()
	eventually(t, "the ingest", func() bool { i, _ := p.got(); return len(i) == 1 })
	if !logs.has("no drawing for this space plan") {
		t.Fatal("the log says why")
	}
}

// TestAttachFailure is a space plan its plan refuses: logged, and offered
// again at the next rescan.
func TestAttachFailure(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "A.dwg"), []byte("drawing"))
	write(t, filepath.Join(dir, "A space.dwg"), []byte("spaces"))
	p := &packager{fail: errors.New("approved")}
	logs, stop := p.watch(t, dir, time.Hour)
	defer stop()
	eventually(t, "the refusal", func() bool { return logs.has("inbox: attach") })
	p.mu.Lock()
	p.fail = nil
	p.mu.Unlock()
	eventually(t, "the attachment", func() bool { _, a := p.got(); return len(a) == 1 })
	// Gone before it settles: forgotten.
	write(t, filepath.Join(dir, "B space.dwg"), []byte("spaces"))
	if err := os.Remove(filepath.Join(dir, "B space.dwg")); err != nil {
		t.Fatal(err)
	}
}

// TestNoAttachIngestsEverything is a watcher with no Attach: every file is
// a plan, as before floor packages.
func TestNoAttachIngestsEverything(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "A.dwg"), []byte("drawing"))
	write(t, filepath.Join(dir, "A space.dwg"), []byte("spaces"))
	rec := &recorder{}
	w := &Watcher{
		Dir:    dir,
		Settle: 20 * time.Millisecond,
		Rescan: 50 * time.Millisecond,
		Ingest: rec.ingest,
	}
	stop := start(t, w)
	defer stop()
	eventually(t, "both", func() bool { return len(rec.got()) == 2 })
}
