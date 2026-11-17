package server

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/weaveplatform/spaceweave/internal/catalog"
)

const (
	dxfBytes = "  0\nSECTION\n  2\nENTITIES\n  0\nENDSEC\n  0\nEOF\n"
	dwgBytes = "AC1032\x00\x00\x00\x00\x00\x00"
)

// packageForm is a multipart body with a file field and, unless role is
// "-", a role field, before or after the file.
func packageForm(
	t *testing.T,
	name string,
	data []byte,
	role string,
	roleFirst bool,
) (string, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	field := func() {
		if role != "-" {
			if err := mw.WriteField("role", role); err != nil {
				t.Fatal(err)
			}
		}
	}
	if roleFirst {
		field()
	}
	fw, err := mw.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatal(err)
	}
	if !roleFirst {
		field()
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return mw.FormDataContentType(), &buf
}

// draftDrawing is a DXF plan traced to a draft.
func (f *fixture) draftDrawing() catalog.Entry {
	f.t.Helper()
	p := filepath.Join(f.t.TempDir(), "Floor 02.dxf")
	if err := os.WriteFile(p, []byte(dxfBytes), 0o600); err != nil {
		f.t.Fatal(err)
	}
	s := f.srv.Store
	e, _, err := s.Ingest(p)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := s.Transition(e.ID, catalog.Converting, nil); err != nil {
		f.t.Fatal(err)
	}
	if _, err := s.Converted(e.ID, "source.png", "drawing.svg", nil); err != nil {
		f.t.Fatal(err)
	}
	if err := s.Transition(e.ID, catalog.Draft, nil); err != nil {
		f.t.Fatal(err)
	}
	e, _ = s.Get(e.ID)
	return e
}

func (f *fixture) del(path string) *httptest.ResponseRecorder {
	f.t.Helper()
	r := httptest.NewRequest(http.MethodDelete, path, nil)
	r.Header.Set(RequestHeader, "1")
	return f.do(r)
}

func TestAttachAndDetach(t *testing.T) {
	f := newFixture(t)
	e := f.draftDrawing()
	files := "/api/plans/" + e.ID + "/files"

	ct, body := packageForm(t, "Floor 02 space.dwg", []byte(dwgBytes), "spaces", false)
	w := f.post(files, ct, body)
	if w.Code != http.StatusCreated {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	got := decode[attachReply](t, w)
	if got.Duplicate || got.Plan.Status != catalog.Queued || len(got.Plan.Files) != 2 ||
		f.wakes.Load() != 1 {
		t.Fatalf("%+v", got)
	}
	spaces := got.Plan.Files[1]
	if spaces.Role != catalog.RoleSpaces || spaces.Name != "Floor 02 space.dwg" {
		t.Fatalf("%+v", spaces)
	}
	// The same file again, its role first this time: nothing changes.
	ct, body = packageForm(t, "again.dwg", []byte(dwgBytes), "spaces", true)
	if w := f.post(
		files,
		ct,
		body,
	); w.Code != http.StatusOK ||
		!decode[attachReply](t, w).Duplicate {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	// A reference joins the queued plan.
	ct, body = packageForm(t, "print.png", pngBytes(t, 4, 4, 9), "reference", true)
	if w := f.post(files, ct, body); w.Code != http.StatusCreated || f.wakes.Load() != 2 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}

	if w := f.del(files + "/" + e.SHA256); w.Code != http.StatusConflict {
		t.Fatalf("the plan's own drawing stays: %d %s", w.Code, w.Body)
	}
	if w := f.del(files + "/nope"); w.Code != http.StatusNotFound {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if w := f.del(files + "/" + spaces.SHA256); w.Code != http.StatusOK ||
		len(decode[attachReply](t, w).Plan.Files) != 2 || f.wakes.Load() != 3 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	r := httptest.NewRequest(http.MethodDelete, files+"/"+spaces.SHA256, nil)
	if w := f.do(r); w.Code != http.StatusForbidden {
		t.Fatalf("a delete without the header: %d", w.Code)
	}
}

func TestAttachRejections(t *testing.T) {
	f := newFixture(t)
	f.srv.MaxUpload = 4096
	e := f.draftDrawing()
	files := "/api/plans/" + e.ID + "/files"
	cases := []struct {
		name, plan, file, role string
		data                   []byte
		code                   int
		want                   string
	}{
		{"no role", e.ID, "s.dwg", "-", []byte(dwgBytes), 400, "field named role"},
		{
			"primary role",
			e.ID,
			"s.dwg",
			"architecture",
			[]byte(dwgBytes),
			400,
			"space plan or a reference",
		},
		{
			"extension",
			e.ID,
			"notes.txt",
			"reference",
			[]byte("hello"),
			415,
			".txt files aren't supported",
		},
		{"misnamed", e.ID, "s.dwg", "spaces", []byte(dxfBytes), 422, "Rename it"},
		{"dotfile", e.ID, ".dwg", "spaces", []byte(dwgBytes), 400, "needs a name"},
		{"too big", e.ID, "s.dwg", "spaces", bytes.Repeat([]byte{'A'}, 8192), 413, "upload limit"},
		{
			"image space plan",
			e.ID,
			"s.png",
			"spaces",
			pngBytes(t, 4, 4, 1),
			422,
			"read from a DXF or DWG",
		},
		{"no plan", "nope", "s.dwg", "spaces", []byte(dwgBytes), 404, "no such plan"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ct, body := packageForm(t, c.file, c.data, c.role, true)
			w := f.post("/api/plans/"+c.plan+"/files", ct, body)
			if w.Code != c.code || !strings.Contains(errorOf(t, w), c.want) {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
		})
	}
	ct, body := form(t, "image", "s.dwg", []byte(dwgBytes))
	if w := f.post(
		files,
		ct,
		body,
	); w.Code != 400 ||
		!strings.Contains(errorOf(t, w), "no file field") {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if w := f.post(files, "application/json", strings.NewReader("{}")); w.Code != 400 {
		t.Fatalf("not multipart: %d", w.Code)
	}
	ct, body = packageForm(t, "s.dwg", []byte(dwgBytes), strings.Repeat("x", 2*maxField), true)
	if w := f.post(files, ct, body); w.Code != 400 {
		t.Fatalf("a role cut short is no role: %d %s", w.Code, w.Body)
	}

	// Busy, then locked by another process, then unreadable.
	if _, ok, _ := f.srv.Store.Claim(); ok {
		t.Fatal("nothing should be queued")
	}
	if err := f.srv.Store.Transition(e.ID, catalog.Approved, nil); err != nil {
		t.Fatal(err)
	}
	ct, body = packageForm(t, "s.dwg", []byte(dwgBytes), "spaces", true)
	if w := f.post(files, ct, body); w.Code != http.StatusConflict {
		t.Fatalf("approved: %d %s", w.Code, w.Body)
	}
	if err := os.WriteFile(filepath.Join(f.srv.Store.Dir, "catalog.lock"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	f.srv.Store.LockWait = 50 * time.Millisecond
	ct, body = packageForm(t, "s.dwg", []byte(dwgBytes), "spaces", true)
	if w := f.post(files, ct, body); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("locked: %d %s", w.Code, w.Body)
	}
	if err := os.Remove(filepath.Join(f.srv.Store.Dir, "catalog.lock")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(f.srv.Store.Dir, "catalog.json"),
		[]byte("{"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	ct, body = packageForm(t, "s.dwg", []byte(dwgBytes), "spaces", true)
	if w := f.post(files, ct, body); w.Code != http.StatusInternalServerError {
		t.Fatalf("unreadable catalogue: %d %s", w.Code, w.Body)
	}
	if f.wakes.Load() != 0 {
		t.Fatal("a refused file woke the workers")
	}
}

func TestAttachNoTempDir(t *testing.T) {
	f := newFixture(t)
	e := f.draftDrawing()
	missing := filepath.Join(t.TempDir(), "absent")
	t.Setenv("TMPDIR", missing)
	t.Setenv("TMP", missing)
	t.Setenv("TEMP", missing)
	ct, body := packageForm(t, "s.dwg", []byte(dwgBytes), "spaces", true)
	if w := f.post("/api/plans/"+e.ID+"/files", ct, body); w.Code != 500 {
		t.Fatalf("no temp dir: %d %s", w.Code, w.Body)
	}
}
