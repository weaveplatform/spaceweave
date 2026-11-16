// Package server is the HTTP side of `spaceweave serve`: the microsite, the
// tracer's output folder, and a small API the page uses to upload plan images
// and review drafts.
//
// Every upload spends money on the Claude API, so the server is cautious
// about who it listens to. It answers only to loopback host names (plus any
// configured), which stops a DNS-rebound page from reaching it, and the API
// refuses state-changing requests without a custom header, which a
// cross-origin page can't send without a CORS preflight this server never
// approves.
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // decoders for the upload check
	_ "image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
	_ "golang.org/x/image/webp"

	"github.com/weaveplatform/spaceweave/internal/catalog"
	"github.com/weaveplatform/spaceweave/internal/inbox"
)

// RequestHeader must accompany every POST to the API. It must match
// js/api.js.
const RequestHeader = "X-Spaceweave"

// Upload limits. MaxPixels stops a small, highly compressed file from
// decoding into gigabytes in the worker.
const (
	DefaultMaxUpload = 40 << 20
	MaxPixels        = 100_000_000
	maxNote          = 64 << 10
)

// Info is what the page shows about the tracer before you upload.
type Info struct {
	Version        string  `json:"version"`
	Model          string  `json:"model"`
	MaxUSD         float64 `json:"maxUsd"`
	MaxUploadBytes int64   `json:"maxUploadBytes"`
}

// Server serves the site and the API.
type Server struct {
	Site  string         // the microsite's folder
	Store *catalog.Store // its generated/ folder
	Info  Info
	// Hosts are host names allowed besides loopback ones.
	Hosts []string
	// Wake tells the workers a plan has been queued.
	Wake      func()
	Log       *slog.Logger
	MaxUpload int64 // DefaultMaxUpload when 0
}

// Handler routes requests.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(s.allowHost, noStore)
	r.Route("/api", func(r chi.Router) {
		r.Use(sameOrigin)
		r.Get("/info", s.info)
		r.Post("/plans", s.upload)
		r.Post("/plans/{id}/{action}", s.review)
		r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
			writeError(w, http.StatusNotFound, "no such API")
		})
	})
	// Mounted separately so the output folder can live outside the site.
	r.Handle("/generated/*", http.StripPrefix("/generated", files(s.Store.Dir)))
	r.Handle("/*", files(s.Site))
	return r
}

func (s *Server) log() *slog.Logger {
	if s.Log == nil {
		return slog.New(slog.DiscardHandler)
	}
	return s.Log
}

func (s *Server) maxUpload() int64 {
	if s.MaxUpload > 0 {
		return s.MaxUpload
	}
	return DefaultMaxUpload
}

func (s *Server) wake() {
	if s.Wake != nil {
		s.Wake()
	}
}

// files serves a folder, hiding dotfiles: the catalogue's lock and the
// temporary files it renames into place.
func files(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for seg := range strings.SplitSeq(r.URL.Path, "/") {
			if strings.HasPrefix(seg, ".") {
				http.NotFound(w, r)
				return
			}
		}
		fs.ServeHTTP(w, r)
	})
}

// noStore stops the browser caching modules and the catalogue, which change
// while the page is open (tools/serve.py does the same).
func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) allowHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		host = strings.ToLower(strings.Trim(host, "[]"))
		ip := net.ParseIP(host)
		ok := host == "localhost" || strings.HasSuffix(host, ".localhost") ||
			(ip != nil && ip.IsLoopback()) || slices.Contains(s.Hosts, host)
		if !ok {
			writeError(w, http.StatusMisdirectedRequest,
				fmt.Sprintf("this server doesn't answer to %q; add it with --allow-host", host))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get(RequestHeader) != "1" {
			writeError(w, http.StatusForbidden, "missing "+RequestHeader+" header")
			return
		}
		if o := r.Header.Get("Origin"); o != "" {
			if u, err := url.Parse(o); err != nil || u.Host != r.Host {
				writeError(w, http.StatusForbidden, "cross-origin request refused")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) info(w http.ResponseWriter, _ *http.Request) {
	i := s.Info
	i.MaxUploadBytes = s.maxUpload()
	writeJSON(w, http.StatusOK, i)
}

// uploadReply is the answer to an upload. Duplicate means the same image was
// uploaded before, and Plan is that earlier plan.
type uploadReply struct {
	Plan      catalog.Entry `json:"plan"`
	Duplicate bool          `json:"duplicate"`
}

func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.maxUpload())
	mr, err := r.MultipartReader()
	if err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			"send the image as multipart/form-data in a field named image",
		)
		return
	}
	part, err := imagePart(mr)
	if err != nil {
		s.bodyError(w, err)
		return
	}
	name := filepath.Base(part.FileName())
	if !inbox.Eligible(name) {
		writeError(
			w,
			http.StatusUnsupportedMediaType,
			"only .png, .jpg, .jpeg and .webp images can be traced",
		)
		return
	}
	// Ingest takes a path and names the plan after the file, so the upload is
	// saved under its own name in a folder of its own.
	tmp, err := os.MkdirTemp("", "spaceweave-upload-")
	if err != nil {
		s.internal(w, err)
		return
	}
	defer os.RemoveAll(tmp) //nolint:errcheck // temp dir
	path := filepath.Join(tmp, name)
	if err := save(path, part); err != nil {
		s.bodyError(w, err)
		return
	}
	if msg := checkImage(path); msg != "" {
		writeError(w, http.StatusUnprocessableEntity, msg)
		return
	}
	e, created, err := s.Store.Ingest(path)
	if err != nil {
		s.internal(w, err)
		return
	}
	if !created {
		s.log().Info("upload already seen", "file", name, "plan", e.ID, "status", e.Status)
		writeJSON(w, http.StatusOK, uploadReply{Plan: e, Duplicate: true})
		return
	}
	s.log().Info("uploaded", "file", name, "plan", e.ID)
	s.wake()
	writeJSON(w, http.StatusCreated, uploadReply{Plan: e})
}

var errNoImage = errors.New("the form has no image field")

func imagePart(mr *multipart.Reader) (*multipart.Part, error) {
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return nil, errNoImage
		}
		if err != nil {
			return nil, fmt.Errorf("read form: %w", err)
		}
		if part.FormName() == "image" {
			return part, nil
		}
	}
}

func save(path string, from io.Reader) error {
	f, err := os.Create(path) //nolint:gosec // a base name inside our own temp dir
	if err != nil {
		return fmt.Errorf("save upload: %w", err)
	}
	if _, err := io.Copy(f, from); err != nil {
		_ = f.Close()
		return fmt.Errorf("save upload: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("save upload: %w", err)
	}
	return nil
}

// checkImage returns why an upload can't be traced, or "". It reads only
// the header, so it's cheap even for a large image.
func checkImage(path string) string {
	f, err := os.Open(path) //nolint:gosec // our own temp file
	if err != nil {
		return "the upload couldn't be read back"
	}
	defer f.Close() //nolint:errcheck // read-only
	cfg, format, err := image.DecodeConfig(f)
	if err != nil || !slices.Contains([]string{"png", "jpeg", "webp"}, format) {
		return "that file isn't a PNG, JPEG or WebP image"
	}
	if cfg.Width*cfg.Height > MaxPixels {
		return fmt.Sprintf("the image is %d × %d pixels; the limit is %d megapixels",
			cfg.Width, cfg.Height, MaxPixels/1_000_000)
	}
	return ""
}

func (s *Server) bodyError(w http.ResponseWriter, err error) {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		writeError(w, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("the image is over the %d MB upload limit", tooBig.Limit>>20))
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}

type reviewReply struct {
	Plan catalog.Entry  `json:"plan"`
	Was  catalog.Status `json:"was"`
}

func (s *Server) review(w http.ResponseWriter, r *http.Request) {
	id, action := chi.URLParam(r, "id"), catalog.Action(chi.URLParam(r, "action"))
	var body struct {
		Note string `json:"note"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxNote))
	if err := dec.Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "the body must be JSON like {\"note\": \"…\"}")
		return
	}
	was, e, err := s.Store.Review(id, action, strings.TrimSpace(body.Note))
	switch {
	case errors.Is(err, catalog.ErrAction), errors.Is(err, catalog.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
		return
	case errors.Is(err, catalog.ErrTransition):
		writeError(w, http.StatusConflict, err.Error())
		return
	case errors.Is(err, catalog.ErrLocked):
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	case err != nil:
		s.internal(w, err)
		return
	}
	s.log().Info("reviewed", "plan", id, "action", action, "from", was, "to", e.Status)
	if e.Status == catalog.Queued {
		s.wake()
	}
	writeJSON(w, http.StatusOK, reviewReply{Plan: e, Was: was})
}

func (s *Server) internal(w http.ResponseWriter, err error) {
	s.log().Error("request failed", "err", err)
	writeError(w, http.StatusInternalServerError, err.Error())
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
