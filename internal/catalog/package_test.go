package catalog

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// drawing ingests a DXF and moves it on to the status given, as the
// worker would.
func drawing(t *testing.T, s *Store, to ...Status) Entry {
	t.Helper()
	e, _, err := s.Ingest(image(t, "Floor 02.dxf", "0\nSECTION\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range to {
		if st == Tracing && e.Status == Converting {
			if e, err = s.Converted(e.ID, "source.png", "drawing.svg", nil); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := s.Transition(e.ID, st, nil); err != nil {
			t.Fatal(err)
		}
		e, _ = s.Get(e.ID)
	}
	return e
}

// TestOldCatalogLoadsAsOneFilePackages is a catalog.json from before floor
// packages: each plan reads as a package of its own drawing, and nothing
// else about it changes.
func TestOldCatalogLoadsAsOneFilePackages(t *testing.T) {
	s, dir := open(t)
	old := `{"version": 1, "plans": [{"id": "a-1", "title": "A", "status": "draft", "source": "source.png",
		"upload": "original.dwg", "format": "dwg", "original": "A.dwg", "sha256": "abc", "costUsd": 0,
		"attempts": 1, "createdAt": "2026-11-01T00:00:00Z", "updatedAt": "2026-11-01T00:00:00Z"}]}`
	if err := os.WriteFile(filepath.Join(dir, "catalog.json"), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	e, err := s.Get("a-1")
	if err != nil {
		t.Fatal(err)
	}
	want := File{
		Role:   RoleArchitecture,
		Upload: "original.dwg",
		Format: "dwg",
		SHA256: "abc",
		Name:   "A.dwg",
	}
	if len(e.Files) != 1 || e.Files[0] != want || e.Upload != "original.dwg" || e.Status != Draft {
		t.Fatalf("%+v", e)
	}
	if e.Primary() != want || len(e.Attached(RoleSpaces)) != 0 {
		t.Fatalf("%+v", e)
	}
	// A plan with nothing uploaded has no package to speak of.
	var none Entry
	none.migrate()
	if none.Files != nil {
		t.Fatalf("%+v", none)
	}
}

func TestAttachSpacesAndReferences(t *testing.T) {
	s, dir := open(t)
	e := drawing(t, s, Converting, Tracing, Draft)
	if len(e.Files) != 1 || e.Files[0].Role != RoleArchitecture {
		t.Fatalf("a new plan is a package of its drawing: %+v", e.Files)
	}

	spaces := image(t, "Floor 02 space.dwg", "AC1032 one")
	got, added, err := s.Attach(e.ID, spaces, RoleSpaces)
	if err != nil || !added {
		t.Fatalf("%v %v", added, err)
	}
	// Back in the queue as a retrace: its drawing converted already, so
	// the next claim goes straight to tracing.
	f := got.Files[1]
	if got.Status != Queued || got.Source != "source.png" || f.Role != RoleSpaces || f.Name != "Floor 02 space.dwg" ||
		f.Format != "dwg" ||
		f.Upload != "files/spaces-"+f.SHA256[:6]+".dwg" {
		t.Fatalf("%+v", got)
	}
	if b, err := os.ReadFile(
		filepath.Join(dir, e.ID, "files", "spaces-"+f.SHA256[:6]+".dwg"),
	); err != nil ||
		string(b) != "AC1032 one" {
		t.Fatalf("copied into the plan: %q %v", b, err)
	}
	if c, ok, err := s.Claim(); err != nil || !ok || c.Status != Tracing || c.ID != e.ID {
		t.Fatalf("%+v %v %v", c, ok, err)
	}

	// Attached while traced: the run under way would miss it.
	if _, _, err := s.Attach(
		e.ID,
		image(t, "other.dwg", "AC1032 two"),
		RoleSpaces,
	); !errors.Is(
		err,
		ErrTransition,
	) {
		t.Fatalf("%v", err)
	}
	// The same content again changes nothing, whatever the plan is doing.
	if again, added, err := s.Attach(
		e.ID,
		image(t, "renamed.dwg", "AC1032 one"),
		RoleSpaces,
	); err != nil ||
		added ||
		len(again.Files) != 2 {
		t.Fatalf("%+v %v %v", again, added, err)
	}
	if err := s.Transition(e.ID, Draft, nil); err != nil {
		t.Fatal(err)
	}

	// A second space plan replaces the first, file and all.
	got, added, err = s.Attach(e.ID, image(t, "Floor 02 space v2.dwg", "AC1032 two"), RoleSpaces)
	if err != nil || !added || len(got.Attached(RoleSpaces)) != 1 ||
		got.Attached(RoleSpaces)[0].Name != "Floor 02 space v2.dwg" {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := os.Stat(
		filepath.Join(dir, e.ID, filepath.FromSlash(f.Upload)),
	); !errors.Is(
		err,
		os.ErrNotExist,
	) {
		t.Fatalf("the replaced file is gone: %v", err)
	}
	// References add up; a queued plan just gains them.
	for _, name := range []string{"print.png", "photo.jpg"} {
		got, added, err = s.Attach(e.ID, image(t, name, name), RoleReference)
		if err != nil || !added || got.Status != Queued {
			t.Fatalf("%+v %v", got, err)
		}
	}
	if refs := got.Attached(RoleReference); len(refs) != 2 || refs[1].Format != "jpeg" {
		t.Fatalf("%+v", refs)
	}
	// The plan's own drawing is in the package already.
	if _, added, err := s.Attach(
		e.ID,
		image(t, "copy.dxf", "0\nSECTION\n"),
		RoleReference,
	); err != nil ||
		added {
		t.Fatalf("%v %v", added, err)
	}
}

func TestAttachRefusals(t *testing.T) {
	s, _ := open(t)
	e := drawing(t, s, Converting, Tracing, Draft)
	png, _, err := s.Ingest(image(t, "photo.png", "pixels"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		id, name, role string
		want           error
	}{
		{e.ID, "a.dwg", RoleArchitecture, ErrRole},
		{e.ID, "a.dwg", "", ErrRole},
		{e.ID, "notes.txt", RoleReference, ErrFormat},
		{"nope", "a.dwg", RoleSpaces, ErrNotFound},
		{e.ID, "spaces.png", RoleSpaces, ErrPackage},
		{png.ID, "a.dwg", RoleSpaces, ErrPackage},
	} {
		if _, _, err := s.Attach(c.id, image(t, c.name, c.name), c.role); !errors.Is(err, c.want) {
			t.Errorf("%s %s as %q: %v, want %v", c.id, c.name, c.role, err, c.want)
		}
	}
	if _, _, err := s.Attach(
		e.ID,
		filepath.Join(t.TempDir(), "missing.dwg"),
		RoleSpaces,
	); err == nil {
		t.Error("a missing file")
	}
	if err := s.Transition(e.ID, Approved, nil); err != nil {
		t.Fatal(err)
	}
	_, _, err = s.Attach(e.ID, image(t, "s.dwg", "s"), RoleSpaces)
	if !errors.Is(err, ErrTransition) || !strings.Contains(err.Error(), "is approved") {
		t.Fatalf("an approved plan is sent back first: %v", err)
	}
}

// TestAttachCannotCopy is a plan directory that can't hold the files
// folder: nothing is recorded.
func TestAttachCannotCopy(t *testing.T) {
	s, dir := open(t)
	e := drawing(t, s, Converting, Tracing, Draft)
	if err := os.WriteFile(filepath.Join(dir, e.ID, "files"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Attach(e.ID, image(t, "s.dwg", "s"), RoleSpaces); err == nil {
		t.Fatal("want an error")
	}
	if after, _ := s.Get(e.ID); after.Status != Draft || len(after.Files) != 1 {
		t.Fatalf("%+v", after)
	}
}

func TestDetach(t *testing.T) {
	s, dir := open(t)
	e := drawing(t, s, Converting, Tracing, Draft)
	got, _, err := s.Attach(e.ID, image(t, "s.dwg", "s"), RoleSpaces)
	if err != nil {
		t.Fatal(err)
	}
	f := got.Files[1]
	for _, c := range []struct {
		id, sum string
		want    error
	}{
		{"nope", f.SHA256, ErrNotFound},
		{e.ID, "0000", ErrNotFound},
		{e.ID, e.SHA256, ErrPrimary},
	} {
		if _, err := s.Detach(c.id, c.sum); !errors.Is(err, c.want) {
			t.Errorf("%s %s: %v, want %v", c.id, c.sum, err, c.want)
		}
	}
	if _, ok, _ := s.Claim(); !ok {
		t.Fatal("claim")
	}
	if _, err := s.Detach(e.ID, f.SHA256); !errors.Is(err, ErrTransition) {
		t.Fatalf("not while it is traced: %v", err)
	}
	if err := s.Transition(e.ID, Failed, func(x *Entry) { x.Error = "boom" }); err != nil {
		t.Fatal(err)
	}
	got, err = s.Detach(e.ID, f.SHA256)
	if err != nil || got.Status != Queued || got.Error != "" || len(got.Files) != 1 {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := os.Stat(
		filepath.Join(dir, e.ID, filepath.FromSlash(f.Upload)),
	); !errors.Is(
		err,
		os.ErrNotExist,
	) {
		t.Fatalf("the file is gone: %v", err)
	}
}

func TestPlanOf(t *testing.T) {
	s, _ := open(t)
	e := drawing(t, s)
	if got, err := s.PlanOf(image(t, "same.dxf", "0\nSECTION\n")); err != nil || got.ID != e.ID {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := s.PlanOf(image(t, "other.dxf", "other")); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.PlanOf(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("a missing file")
	}
	if err := os.WriteFile(s.path(), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PlanOf(image(t, "same.dxf", "0\nSECTION\n")); err == nil {
		t.Fatal("a broken catalogue")
	}
}
