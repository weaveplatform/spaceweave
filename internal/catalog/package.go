package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/weaveplatform/spaceweave/internal/convert"
)

// Floor packages. A plan is traced from one drawing, but a client often has
// more than one file for the same floor: the architect's drawing, a
// facilities team's space plan with every room's outline, number and name,
// and a PDF print. The space plan is attached to the plan traced from the
// drawing rather than traced on its own, and changing the package traces
// the plan again from the drawing already converted.

// readsSpaces says the tracer can read space outlines from a format: CAD
// entities only. An image or SVG of a space plan has lost its outlines'
// layers and its text, so it is a reference at most.
func readsSpaces(format string) bool { return format == convert.DXF.ID || format == convert.DWG.ID }

// Attach adds the file at path to a plan's package and queues the plan to be
// traced again. A space plan replaces the plan's previous one; a reference
// is added beside any others. added is false when the package already holds
// the same content, which changes nothing.
//
// A queued plan just gains the file, which its coming trace reads. A draft
// or failure goes back to the queue as a retrace does. A plan being
// converted or traced is refused: the run under way would miss the file
// and publish a draft that looked as if it had read it. So is an approved
// plan, which a reviewer must send back first.
func (s *Store) Attach(id, path, role string) (Entry, bool, error) {
	if role != RoleSpaces && role != RoleReference {
		return Entry{}, false, fmt.Errorf("%w, not %q", ErrRole, role)
	}
	format, ok := convert.ByExt(path)
	if !ok {
		return Entry{}, false, fmt.Errorf("%w: %s", ErrFormat, filepath.Base(path))
	}
	sum, err := hashFile(path)
	if err != nil {
		return Entry{}, false, err
	}
	var now Entry
	added := false
	var replaced []string
	err = s.Update(func(c *Catalog) error {
		e, err := c.find(id)
		if err != nil {
			return err
		}
		if slices.ContainsFunc(e.Files, func(f File) bool { return f.SHA256 == sum }) {
			now = *e
			return nil
		}
		if role == RoleSpaces && (!readsSpaces(format.ID) || !readsSpaces(e.Format)) {
			return fmt.Errorf(
				"%w: a space plan is read from a DXF or DWG and placed on the plan's own DXF or DWG; "+
					"this is a %s space plan for a %s plan",
				ErrPackage,
				format.Label,
				strings.ToUpper(e.Format),
			)
		}
		if err := s.requeue(e); err != nil {
			return err
		}
		// Named by content, so a replacement never overwrites the file a
		// worker may be reading, and the same file always lands in the
		// same place.
		f := File{
			Role: role,
			Upload: filepath.ToSlash(
				filepath.Join(filesDir, role+"-"+sum[:6]+strings.ToLower(filepath.Ext(path))),
			),
			Format: format.ID,
			SHA256: sum,
			Name:   filepath.Base(path),
		}
		if err := os.MkdirAll(filepath.Join(s.PlanDir(id), filesDir), 0o750); err != nil {
			return fmt.Errorf("create plan files directory: %w", err)
		}
		if err := copyFile(
			path,
			filepath.Join(s.PlanDir(id), filepath.FromSlash(f.Upload)),
		); err != nil {
			return err
		}
		if role == RoleSpaces {
			e.Files = slices.DeleteFunc(e.Files, func(old File) bool {
				if old.Role == RoleSpaces {
					replaced = append(replaced, old.Upload)
					return true
				}
				return false
			})
		}
		e.Files = append(e.Files, f)
		added, now = true, *e
		return nil
	})
	if err != nil {
		return Entry{}, false, err
	}
	s.remove(id, replaced)
	return now, added, nil
}

// Detach removes an attached file from a plan's package, by its SHA-256,
// and queues the plan to be traced again without it, under the same rules
// as Attach.
func (s *Store) Detach(id, sum string) (Entry, error) {
	var now Entry
	var gone string
	err := s.Update(func(c *Catalog) error {
		e, err := c.find(id)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(e.Files, func(f File) bool { return f.SHA256 == sum })
		switch {
		case i < 0:
			return fmt.Errorf("%w: %s has no file %s", ErrNotFound, id, sum)
		case e.Files[i].Role == RoleArchitecture:
			return ErrPrimary
		}
		if err := s.requeue(e); err != nil {
			return err
		}
		gone = e.Files[i].Upload
		e.Files = slices.Delete(e.Files, i, i+1)
		now = *e
		return nil
	})
	if err != nil {
		return Entry{}, err
	}
	s.remove(id, []string{gone})
	return now, nil
}

// requeue sends a plan whose package changed back to be traced, as the
// Retrace action does; a plan still queued stays as it is.
func (s *Store) requeue(e *Entry) error {
	if e.Status == Queued {
		return nil
	}
	if !slices.Contains(reviews[Retrace].from, e.Status) {
		return fmt.Errorf("%w: %s is %s; its files can change once it is a draft or has failed",
			ErrTransition, e.ID, e.Status)
	}
	e.Status = Queued
	e.UpdatedAt = s.Now().UTC()
	e.Progress = nil
	retrace(e)
	return nil
}

// remove deletes files a package no longer holds. It runs after the
// catalogue is saved, so a failure leaves a stray file, never an entry
// pointing at nothing; the next trace doesn't read it either way.
func (s *Store) remove(id string, uploads []string) {
	for _, u := range uploads {
		_ = os.Remove(filepath.Join(s.PlanDir(id), filepath.FromSlash(u)))
	}
}

// PlanOf is the plan ingested from a file with the same content as path:
// the inbox's way to find the plan a space plan belongs to from the
// drawing beside it.
func (s *Store) PlanOf(path string) (Entry, error) {
	sum, err := hashFile(path)
	if err != nil {
		return Entry{}, err
	}
	c, err := s.Load()
	if err != nil {
		return Entry{}, err
	}
	for _, e := range c.Plans {
		if e.SHA256 == sum {
			return e, nil
		}
	}
	return Entry{}, fmt.Errorf("%w: none from %s", ErrNotFound, filepath.Base(path))
}
