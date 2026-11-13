package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/weaveplatform/spaceweave/internal/catalog"
)

type fixture struct {
	t     *testing.T
	srv   *Server
	h     http.Handler
	wakes atomic.Int32
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	site := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(site, "index.html"),
		[]byte("<title>site</title>"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	store, err := catalog.Open(filepath.Join(t.TempDir(), "generated"))
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t}
	f.srv = &Server{
		Site: site, Store: store, Hosts: []string{"studio.local"},
		Info: Info{Version: "v1", Model: "claude-opus-5-5", MaxUSD: 15},
		Wake: func() { f.wakes.Add(1) },
	}
	f.h = f.srv.Handler()
	return f
}

func (f *fixture) do(r *http.Request) *httptest.ResponseRecorder {
	f.t.Helper()
	if r.Host == "" || r.Host == "example.com" {
		r.Host = "localhost:8000"
	}
	w := httptest.NewRecorder()
	f.h.ServeHTTP(w, r)
	return w
}

func (f *fixture) post(path, contentType string, body io.Reader) *httptest.ResponseRecorder {
	f.t.Helper()
	r := httptest.NewRequest(http.MethodPost, path, body)
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	r.Header.Set(RequestHeader, "1")
	return f.do(r)
}

func pngBytes(t *testing.T, w, h int, mark uint8) []byte {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, w, h))
	img.Pix[0] = mark
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// form builds a multipart body with one file field.
func form(t *testing.T, field, name string, data []byte) (string, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("comment", "ignored"); err != nil {
		t.Fatal(err)
	}
	fw, err := mw.CreateFormFile(field, name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return mw.FormDataContentType(), &buf
}

func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("%v: %s", err, w.Body)
	}
	return v
}

func errorOf(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	return decode[map[string]string](t, w)["error"]
}

func TestStaticFilesAndHosts(t *testing.T) {
	f := newFixture(t)
	if err := os.WriteFile(
		filepath.Join(f.srv.Store.Dir, "catalog.json"),
		[]byte(`{"version":1}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.srv.Store.Dir, "catalog.lock"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		path, host string
		code       int
		body       string
	}{
		{"/", "localhost:8000", 200, "<title>site</title>"},
		{"/index.html", "127.0.0.1:8000", 301, ""}, // FileServer canonicalises to /
		{"/generated/catalog.json", "[::1]:8000", 200, `{"version":1}`},
		{"/generated/catalog.lock", "localhost", 200, ""},
		{"/generated/.catalog.json.123", "app.localhost:8000", 404, ""},
		{"/", "studio.local:8000", 200, "site"},
		{"/", "evil.example:8000", 421, "--allow-host"},
		{"/", "169.254.1.1", 421, ""},
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodGet, c.path, nil)
		r.Host = c.host
		w := f.do(r)
		if w.Code != c.code || !strings.Contains(w.Body.String(), c.body) {
			t.Errorf("GET %s (Host %s): %d %q", c.path, c.host, w.Code, w.Body)
		}
		if w.Header().Get("Cache-Control") != "no-store" && w.Code != 421 {
			t.Errorf("%s: caching not disabled", c.path)
		}
	}
	// A dotfile anywhere in the path is hidden, not just the last segment.
	if w := f.do(httptest.NewRequest(http.MethodGet, "/.git/config", nil)); w.Code != 404 {
		t.Fatalf("dot directory served: %d", w.Code)
	}
}

func TestInfoAndUnknownAPI(t *testing.T) {
	f := newFixture(t)
	w := f.do(httptest.NewRequest(http.MethodGet, "/api/info", nil))
	info := decode[Info](t, w)
	if w.Code != 200 || info.Model != "claude-opus-5-5" || info.MaxUSD != 15 ||
		info.MaxUploadBytes != DefaultMaxUpload {
		t.Fatalf("%d %+v", w.Code, info)
	}
	if w := f.do(
		httptest.NewRequest(http.MethodGet, "/api/nope", nil),
	); w.Code != 404 ||
		errorOf(t, w) != "no such API" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestUpload(t *testing.T) {
	f := newFixture(t)
	ct, body := form(t, "image", "Studio Plan.png", pngBytes(t, 20, 10, 0))
	w := f.post("/api/plans", ct, body)
	got := decode[uploadReply](t, w)
	if w.Code != http.StatusCreated || got.Duplicate || got.Plan.Status != catalog.Queued ||
		got.Plan.Title != "Studio Plan" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if f.wakes.Load() != 1 {
		t.Fatal("the worker wasn't woken")
	}
	if _, err := os.Stat(
		filepath.Join(f.srv.Store.PlanDir(got.Plan.ID), "source.png"),
	); err != nil {
		t.Fatal(err)
	}

	ct, body = form(t, "image", "renamed.png", pngBytes(t, 20, 10, 0))
	w = f.post("/api/plans", ct, body)
	again := decode[uploadReply](t, w)
	if w.Code != 200 || !again.Duplicate || again.Plan.ID != got.Plan.ID || f.wakes.Load() != 1 {
		t.Fatalf("the same image again: %d %s", w.Code, w.Body)
	}
}

// A PNG header claiming a huge image: cheap to send, ruinous to decode.
func hugePNG() []byte {
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], 20000)
	binary.BigEndian.PutUint32(ihdr[4:], 20000)
	ihdr[8], ihdr[9] = 8, 0 // 8-bit greyscale
	chunk := append([]byte("IHDR"), ihdr...)
	var b bytes.Buffer
	b.WriteString("\x89PNG\r\n\x1a\n")
	_ = binary.Write(&b, binary.BigEndian, uint32(len(ihdr)))
	b.Write(chunk)
	_ = binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(chunk))
	return b.Bytes()
}

func TestUploadRejections(t *testing.T) {
	f := newFixture(t)
	f.srv.MaxUpload = 4096
	ok := pngBytes(t, 4, 4, 1)
	cases := []struct {
		name  string
		field string
		file  string
		data  []byte
		code  int
		want  string
	}{
		{"wrong field", "file", "a.png", ok, 400, "no image field"},
		{"extension", "image", "notes.txt", ok, 415, "only .png"},
		{"dotfile", "image", ".png", ok, 415, "only .png"},
		{"not an image", "image", "a.png", []byte("hello"), 422, "isn't a PNG"},
		{
			"gif",
			"image",
			"a.png",
			[]byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"),
			422,
			"isn't a PNG",
		},
		{"too many pixels", "image", "a.png", hugePNG(), 422, "limit is 100 megapixels"},
		{"too big", "image", "a.png", bytes.Repeat([]byte{0}, 8192), 413, "upload limit"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ct, body := form(t, c.field, c.file, c.data)
			w := f.post("/api/plans", ct, body)
			if w.Code != c.code || !strings.Contains(errorOf(t, w), c.want) {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
		})
	}
	if w := f.post("/api/plans", "application/json", strings.NewReader("{}")); w.Code != 400 {
		t.Fatalf("not multipart: %d", w.Code)
	}
	ct, body := form(t, "image", "a.png", ok)
	cut := body.Bytes()[:body.Len()-60] // the client went away mid-file
	if w := f.post("/api/plans", ct, bytes.NewReader(cut)); w.Code != 400 {
		t.Fatalf("truncated upload: %d %s", w.Code, w.Body)
	}
	if w := f.post(
		"/api/plans",
		"multipart/form-data; boundary=x",
		strings.NewReader("--x\r\nbroken"),
	); w.Code != 400 {
		t.Fatalf("malformed form: %d %s", w.Code, w.Body)
	}
	if c, _ := f.srv.Store.Load(); len(c.Plans) != 0 || f.wakes.Load() != 0 {
		t.Fatal("a rejected upload was queued")
	}
}

func TestUploadServerErrors(t *testing.T) {
	f := newFixture(t)
	missing := filepath.Join(t.TempDir(), "absent")
	t.Setenv("TMPDIR", missing)
	t.Setenv("TMP", missing)
	t.Setenv("TEMP", missing)
	ct, body := form(t, "image", "a.png", pngBytes(t, 4, 4, 2))
	if w := f.post("/api/plans", ct, body); w.Code != 500 {
		t.Fatalf("no temp dir: %d %s", w.Code, w.Body)
	}
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("TMP", os.Getenv("TMPDIR"))
	t.Setenv("TEMP", os.Getenv("TMPDIR"))
	if err := os.WriteFile(
		filepath.Join(f.srv.Store.Dir, "catalog.json"),
		[]byte("{"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	ct, body = form(t, "image", "a.png", pngBytes(t, 4, 4, 3))
	if w := f.post("/api/plans", ct, body); w.Code != 500 {
		t.Fatalf("unreadable catalogue: %d %s", w.Code, w.Body)
	}
}

func TestRequestsFromOtherPagesAreRefused(t *testing.T) {
	f := newFixture(t)
	ct, body := form(t, "image", "a.png", pngBytes(t, 4, 4, 0))
	r := httptest.NewRequest(http.MethodPost, "/api/plans", body)
	r.Header.Set("Content-Type", ct)
	if w := f.do(r); w.Code != 403 || !strings.Contains(errorOf(t, w), RequestHeader) {
		t.Fatalf("a plain form post: %d %s", w.Code, w.Body)
	}
	for origin, code := range map[string]int{"http://evil.example": 403, "::bad": 403, "http://localhost:8000": 201} {
		ct, body := form(t, "image", "a.png", pngBytes(t, 4, 4, uint8(len(origin))))
		r := httptest.NewRequest(http.MethodPost, "/api/plans", body)
		r.Header.Set("Content-Type", ct)
		r.Header.Set(RequestHeader, "1")
		r.Header.Set("Origin", origin)
		if w := f.do(r); w.Code != code {
			t.Errorf("Origin %s: %d %s", origin, w.Code, w.Body)
		}
	}
}

func TestReview(t *testing.T) {
	f := newFixture(t)
	f.srv.Wake, f.srv.Log = nil, nil // optional
	f.h = f.srv.Handler()
	store := f.srv.Store
	src := filepath.Join(t.TempDir(), "a.png")
	if err := os.WriteFile(src, pngBytes(t, 4, 4, 0), 0o600); err != nil {
		t.Fatal(err)
	}
	e, _, err := store.Ingest(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []catalog.Status{catalog.Tracing, catalog.Draft} {
		if err := store.Transition(e.ID, s, nil); err != nil {
			t.Fatal(err)
		}
	}
	act := func(action, body string) *httptest.ResponseRecorder {
		return f.post("/api/plans/"+e.ID+"/"+action, "application/json", strings.NewReader(body))
	}
	w := act("approve", "")
	if got := decode[reviewReply](
		t,
		w,
	); w.Code != 200 || got.Was != catalog.Draft ||
		got.Plan.Status != catalog.Approved {
		t.Fatalf("approve: %d %s", w.Code, w.Body)
	}
	if w := act("approve", ""); w.Code != 409 {
		t.Fatalf("approving twice: %d %s", w.Code, w.Body)
	}
	if w := act("reject", "{"); w.Code != 400 {
		t.Fatalf("bad JSON: %d", w.Code)
	}
	w = act("reject", `{"note": "  The kitchen is missing  "}`)
	if got := decode[reviewReply](
		t,
		w,
	); w.Code != 200 || got.Plan.Status != catalog.Queued ||
		got.Plan.Note != "The kitchen is missing" {
		t.Fatalf("reject: %d %s", w.Code, w.Body)
	}
	if w := act("retry", ""); w.Code != 409 {
		t.Fatalf("retrying a queued plan: %d", w.Code)
	}
	if w := act("delete", ""); w.Code != 404 {
		t.Fatalf("unknown action: %d", w.Code)
	}
	if w := f.post(
		"/api/plans/nope/approve",
		"",
		nil,
	); w.Code != 404 ||
		!strings.Contains(errorOf(t, w), "no such plan") {
		t.Fatalf("unknown plan: %d %s", w.Code, w.Body)
	}

	store.LockWait = 10 * time.Millisecond
	lock := filepath.Join(store.Dir, "catalog.lock")
	if err := os.WriteFile(lock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if w := act("retry", ""); w.Code != 503 {
		t.Fatalf("locked: %d %s", w.Code, w.Body)
	}
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(store.Dir, "catalog.json"),
		[]byte("{"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if w := act("retry", ""); w.Code != 500 {
		t.Fatalf("unreadable catalogue: %d %s", w.Code, w.Body)
	}
}

func TestRejectWakesTheWorker(t *testing.T) {
	f := newFixture(t)
	src := filepath.Join(t.TempDir(), "a.png")
	if err := os.WriteFile(src, pngBytes(t, 4, 4, 0), 0o600); err != nil {
		t.Fatal(err)
	}
	e, _, _ := f.srv.Store.Ingest(src)
	_ = f.srv.Store.Transition(e.ID, catalog.Tracing, nil)
	_ = f.srv.Store.Transition(e.ID, catalog.Failed, func(x *catalog.Entry) { x.Error = "boom" })
	w := f.post("/api/plans/"+e.ID+"/retry", "", nil)
	if got := decode[reviewReply](
		t,
		w,
	); w.Code != 200 || got.Plan.Error != "" ||
		f.wakes.Load() != 1 {
		t.Fatalf("%d %s wakes=%d", w.Code, w.Body, f.wakes.Load())
	}
}
