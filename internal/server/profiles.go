package server

import (
	"errors"
	"io"
	"net/http"
	"slices"

	"github.com/go-chi/chi/v5"

	"github.com/weaveplatform/spaceweave/internal/autotrace/profile"
)

// maxProfile bounds a profile sent to be saved. A client profile is a rule
// per corrected layer; a drawing has tens of layers, not tens of
// thousands.
const maxProfile = 1 << 20

// profileReply is a profile as the page sees it. Active says the tracer
// reads drawings with it; Exists is false for a client profile not saved
// yet, which reads as one with no rules. Roles are what a rule can say, for
// the page's role menus.
type profileReply struct {
	Name    string             `json:"name"`
	Builtin bool               `json:"builtin"`
	Exists  bool               `json:"exists"`
	Active  bool               `json:"active"`
	Profile *profile.Profile   `json:"profile"`
	Roles   []profile.RoleInfo `json:"roles"`
}

func (s *Server) reply(w http.ResponseWriter, code int, name string, p *profile.Profile) {
	active, err := s.Profiles.Active()
	if err != nil {
		s.internal(w, err)
		return
	}
	r := profileReply{
		Name: name, Builtin: slices.Contains(profile.BuiltinNames, name), Exists: p != nil,
		Active: name == active, Profile: p, Roles: profile.Roles,
	}
	if p == nil {
		r.Profile = &profile.Profile{Rules: []profile.Rule{}}
	}
	writeJSON(w, code, r)
}

func (s *Server) getProfile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	p, err := s.Profiles.Get(name)
	switch {
	case errors.Is(err, profile.ErrNotFound):
		s.reply(w, http.StatusOK, name, nil)
	case errors.Is(err, profile.ErrName):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, profile.ErrInvalid):
		// The file is there but broken, by hand: say how, so it can be
		// fixed, rather than offer an empty profile that would replace it.
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	case err != nil:
		s.internal(w, err)
	default:
		s.reply(w, http.StatusOK, name, p)
	}
}

// putProfile saves a client profile whole. The body is the profile as
// JSON (or YAML), decoded as strictly as the file is read, so a key the
// tracer would ignore is refused here rather than saved.
func (s *Server) putProfile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := profile.CheckName(name); err != nil {
		code := http.StatusBadRequest
		if errors.Is(err, profile.ErrBuiltin) {
			code = http.StatusForbidden
		}
		writeError(w, code, err.Error())
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxProfile))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "the profile is over 1 MB")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	p, err := profile.Parse(name, body)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := s.Profiles.Put(name, p); err != nil {
		s.internal(w, err)
		return
	}
	s.log().Info("profile saved", "profile", name, "rules", len(p.Rules))
	s.reply(w, http.StatusOK, name, p)
}
