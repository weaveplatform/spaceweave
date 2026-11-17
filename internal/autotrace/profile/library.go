package profile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Library is the folder of client profiles, profiles/ in the tracer's
// output folder: one <name>.yaml each, and the name of the one in use.
//
// The profile in use is the configured one (Fixed). With none configured,
// the first profile saved from the page becomes it, recorded in .active so
// a restart keeps tracing with it; dotfiles are never served, so the
// record isn't either.
type Library struct {
	Dir   string
	Fixed string
}

const activeFile = ".active"

// validName keeps a profile's name a plain file name, and so a plain URL
// segment: no path, no dot files, nothing to escape.
var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// CheckName rejects a name that can't be a client profile's.
func CheckName(name string) error {
	if !validName.MatchString(name) {
		return fmt.Errorf(
			"%w: %q: use lower-case letters, digits, - and _, up to 64",
			ErrName,
			name,
		)
	}
	if slices.Contains(BuiltinNames, name) {
		return fmt.Errorf(
			"%w: %q is built in; save to a client profile of another name",
			ErrBuiltin,
			name,
		)
	}
	return nil
}

func (l *Library) path(name string) string { return filepath.Join(l.Dir, name+".yaml") }

// Active is the name of the client profile in use, or "" for none.
func (l *Library) Active() (string, error) {
	if l == nil {
		return "", nil
	}
	if l.Fixed != "" {
		return l.Fixed, nil
	}
	b, err := os.ReadFile(filepath.Join(l.Dir, activeFile))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", nil
	case err != nil:
		return "", fmt.Errorf("read the active profile: %w", err)
	}
	name := strings.TrimSpace(string(b))
	if err := CheckName(name); err != nil {
		return "", fmt.Errorf("%s: %w", filepath.Join(l.Dir, activeFile), err)
	}
	return name, nil
}

// Get reads a profile: a built-in by its name, or a client's file. A
// client profile with no file yet is ErrNotFound.
func (l *Library) Get(name string) (*Profile, error) {
	if p, ok := Builtin(name); ok {
		return p, nil
	}
	if err := CheckName(name); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(l.path(name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	if err != nil {
		return nil, fmt.Errorf("read profile %s: %w", name, err)
	}
	return Parse(l.path(name), b)
}

// Put validates and saves a client profile, replacing the file whole so a
// tracer reading it at the same moment sees the old profile or the new.
// With no profile in use yet, this one becomes it.
func (l *Library) Put(name string, p *Profile) error {
	if err := CheckName(name); err != nil {
		return err
	}
	if _, err := p.compile(name, ""); err != nil {
		return err
	}
	b, err := p.Encode()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(l.Dir, 0o750); err != nil {
		return fmt.Errorf("create the profiles folder: %w", err)
	}
	if err := writeFile(l.path(name), b); err != nil {
		return err
	}
	active, err := l.Active()
	if err != nil || active != "" {
		return err
	}
	return writeFile(filepath.Join(l.Dir, activeFile), []byte(name+"\n"))
}

// Set is the client profile in use over the built-ins, and its name ("" for
// none). A profile in use whose file doesn't exist yet adds nothing.
func (l *Library) Set() (*Set, string, error) {
	name, err := l.Active()
	if err != nil || name == "" {
		return &Set{}, "", err
	}
	p, err := l.Get(name)
	switch {
	case errors.Is(err, ErrNotFound):
		return &Set{}, name, nil
	case err != nil:
		return nil, name, err
	}
	s, err := NewSet(name, p)
	return s, name, err
}

// writeFile writes through a temporary file and a rename. It is
// catalog.WriteFile's twin, kept here so the tracer's rules don't depend on
// the output folder's bookkeeping.
func writeFile(path string, data []byte) error {
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
