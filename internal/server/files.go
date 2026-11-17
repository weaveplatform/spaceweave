package server

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/weaveplatform/spaceweave/internal/catalog"
)

// maxField bounds a form's small text fields, such as an attachment's role.
const maxField = 1 << 10

// noFile is a form without the field its file goes in, named in the
// message the page shows.
type noFile string

func (f noFile) Error() string { return "the form has no " + string(f) + " field" }

// received is an upload saved under its own name in a temporary folder,
// with the form's other fields.
type received struct {
	path   string
	fields map[string]string
	dir    string
}

func (r *received) close() { _ = os.RemoveAll(r.dir) }

// receive reads a multipart upload whose file is in the named field,
// saving the file under its own name and keeping the other fields, which
// may come before or after it. It checks the file as an upload is checked.
// On failure it has answered, and returns nil.
func (s *Server) receive(w http.ResponseWriter, r *http.Request, field string) *received {
	r.Body = http.MaxBytesReader(w, r.Body, s.maxUpload())
	mr, err := r.MultipartReader()
	if err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			"send the "+field+" as multipart/form-data in a field named "+field,
		)
		return nil
	}
	tmp, err := os.MkdirTemp("", "spaceweave-upload-")
	if err != nil {
		s.internal(w, err)
		return nil
	}
	got := &received{fields: map[string]string{}, dir: tmp}
	if err := got.read(mr, field); err != nil {
		got.close()
		var named nameError
		if errors.As(err, &named) {
			writeError(w, http.StatusBadRequest, err.Error())
			return nil
		}
		s.bodyError(w, err)
		return nil
	}
	if code, msg := check(got.path); msg != "" {
		got.close()
		writeError(w, code, msg)
		return nil
	}
	return got
}

// nameError is a file sent without a usable name.
type nameError struct{}

func (nameError) Error() string { return "the file needs a name" }

func (r *received) read(mr *multipart.Reader, field string) error {
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read form: %w", err)
		}
		name := part.FormName()
		switch {
		case name == field && r.path == "":
			base := filepath.Base(part.FileName())
			if strings.HasPrefix(base, ".") || part.FileName() == "" {
				return nameError{}
			}
			// Ingest and Attach take a path and keep the file's name, so it
			// is saved under its own name in a folder of its own.
			r.path = filepath.Join(r.dir, base)
			if err := save(r.path, part); err != nil {
				return err
			}
		case part.FileName() == "":
			b, err := io.ReadAll(io.LimitReader(part, maxField))
			if err != nil {
				return fmt.Errorf("read form: %w", err)
			}
			r.fields[name] = strings.TrimSpace(string(b))
		}
	}
	if r.path == "" {
		return noFile(field)
	}
	return nil
}

// attachReply is the answer to attaching a file: the plan as it is now,
// and whether the file was in its package already.
type attachReply struct {
	Plan      catalog.Entry `json:"plan"`
	Duplicate bool          `json:"duplicate"`
}

// attach adds a file to a plan's floor package: a multipart file and the
// role it plays (catalog.RoleSpaces or catalog.RoleReference). The plan is
// traced again from the drawing it has, which is not converted again.
func (s *Server) attach(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	got := s.receive(w, r, "file")
	if got == nil {
		return
	}
	defer got.close()
	role := got.fields["role"]
	if role == "" {
		writeError(w, http.StatusBadRequest, "say what the file is in a field named role: "+
			catalog.RoleSpaces+" or "+catalog.RoleReference)
		return
	}
	e, added, err := s.Store.Attach(id, got.path, role)
	if s.storeError(w, err) {
		return
	}
	name := filepath.Base(got.path)
	if !added {
		s.log().Info("attachment already in the package", "file", name, "plan", id)
		writeJSON(w, http.StatusOK, attachReply{Plan: e, Duplicate: true})
		return
	}
	s.log().Info("attached", "file", name, "plan", id, "role", role)
	s.requeued(e)
	writeJSON(w, http.StatusCreated, attachReply{Plan: e})
}

// detach removes a file from a plan's floor package by its SHA-256, and
// traces the plan again without it.
func (s *Server) detach(w http.ResponseWriter, r *http.Request) {
	id, sum := chi.URLParam(r, "id"), chi.URLParam(r, "sha")
	e, err := s.Store.Detach(id, sum)
	if s.storeError(w, err) {
		return
	}
	s.log().Info("detached", "plan", id, "sha256", sum)
	s.requeued(e)
	writeJSON(w, http.StatusOK, attachReply{Plan: e})
}

func (s *Server) requeued(e catalog.Entry) {
	if e.Status == catalog.Queued {
		s.wake()
	}
}

// storeError answers for a catalogue error, and reports whether there was
// one.
func (s *Server) storeError(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, catalog.ErrNotFound), errors.Is(err, catalog.ErrAction):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, catalog.ErrRole):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, catalog.ErrFormat):
		writeError(w, http.StatusUnsupportedMediaType, err.Error())
	case errors.Is(err, catalog.ErrPackage):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, catalog.ErrTransition), errors.Is(err, catalog.ErrPrimary):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, catalog.ErrLocked):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	default:
		s.internal(w, err)
	}
	return true
}
