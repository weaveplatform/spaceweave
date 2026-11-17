package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/autotrace/profile"
	"github.com/weaveplatform/spaceweave/internal/catalog"
)

func profileFixture(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	f.srv.Profiles = &profile.Library{Dir: filepath.Join(f.srv.Store.Dir, "profiles")}
	f.h = f.srv.Handler()
	return f
}

func (f *fixture) put(path, body string, header bool) *httptest.ResponseRecorder {
	f.t.Helper()
	r := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if header {
		r.Header.Set(RequestHeader, "1")
	}
	return f.do(r)
}

func (f *fixture) get(path string) *httptest.ResponseRecorder {
	f.t.Helper()
	return f.do(httptest.NewRequest(http.MethodGet, path, nil))
}

func TestProfiles(t *testing.T) {
	f := profileFixture(t)
	if info := decode[Info](t, f.get("/api/info")); !info.Profiles || info.Profile != "" {
		t.Fatalf("no profile in use yet: %+v", info)
	}
	w := f.get("/api/profiles/acme")
	got := decode[profileReply](t, w)
	if w.Code != 200 || got.Exists || got.Builtin || got.Active || got.Profile == nil ||
		got.Profile.Rules == nil || len(got.Roles) != len(profile.Roles) {
		t.Fatalf("a client profile not saved yet reads as empty: %d %s", w.Code, w.Body)
	}
	if got := decode[profileReply](t, f.get("/api/profiles/uniclass")); !got.Builtin || !got.Exists ||
		len(got.Profile.Rules) == 0 {
		t.Fatalf("a built-in reads: %+v", got)
	}

	body := `{"rules": [{"role": "wall", "layer": "Q-TRENNWAND"}]}`
	if w := f.put("/api/profiles/acme", body, false); w.Code != 403 {
		t.Fatalf("a PUT needs the header a cross-origin page can't send: %d", w.Code)
	}
	w = f.put("/api/profiles/acme", body, true)
	got = decode[profileReply](t, w)
	if w.Code != 200 || !got.Exists || !got.Active || got.Profile.Rules[0].Role != profile.Wall {
		t.Fatalf("saved, and in use as the first saved: %d %s", w.Code, w.Body)
	}
	b, err := os.ReadFile(filepath.Join(f.srv.Store.Dir, "profiles", "acme.yaml"))
	if err != nil || !strings.Contains(string(b), "layer: Q-TRENNWAND") {
		t.Fatalf("written as YAML: %s %v", b, err)
	}
	if info := decode[Info](t, f.get("/api/info")); info.Profile != "acme" {
		t.Fatalf("the page learns the profile in use: %+v", info)
	}
	if got := decode[profileReply](t, f.get("/api/profiles/acme")); !got.Exists || len(got.Profile.Rules) != 1 {
		t.Fatalf("read back: %+v", got)
	}

	for _, c := range []struct {
		path, body string
		code       int
		msg        string
	}{
		{"/api/profiles/generic", body, 403, "built in"},
		{"/api/profiles/Bad%20Name", body, 400, "invalid profile name"},
		{"/api/profiles/acme", `{"rules": [{"role": "walls", "layer": "x"}]}`, 422, `unknown role "walls"`},
		{"/api/profiles/acme", `{"rules": [{"role": "wall", "layers": "x"}]}`, 422, `unknown key "layers"`},
		{"/api/profiles/acme", strings.Repeat(" ", maxProfile+1), 413, "over 1 MB"},
	} {
		w := f.put(c.path, c.body, true)
		if w.Code != c.code || !strings.Contains(errorOf(t, w), c.msg) {
			t.Errorf("PUT %s: %d %s", c.path, w.Code, w.Body)
		}
	}
	if w := f.get("/api/profiles/Bad%20Name"); w.Code != 400 {
		t.Errorf("GET a bad name: %d", w.Code)
	}
	// A profile broken by hand says how, rather than reading as empty and
	// being overwritten by the page.
	if err := os.WriteFile(filepath.Join(f.srv.Store.Dir, "profiles", "acme.yaml"), []byte("rules: [{rol: wall}]"), 0o600); err != nil {
		t.Fatal(err)
	}
	if w := f.get("/api/profiles/acme"); w.Code != 422 || !strings.Contains(errorOf(t, w), `unknown key "rol"`) {
		t.Errorf("a broken file: %d %s", w.Code, w.Body)
	}
}

func TestProfileServerErrors(t *testing.T) {
	f := profileFixture(t)
	dir := filepath.Join(f.srv.Store.Dir, "profiles")
	if err := os.MkdirAll(filepath.Join(dir, "acme.yaml"), 0o750); err != nil {
		t.Fatal(err)
	}
	if w := f.put("/api/profiles/acme", `{"rules": []}`, true); w.Code != 500 {
		t.Errorf("an unwritable profile: %d %s", w.Code, w.Body)
	}
	if w := f.get("/api/profiles/acme"); w.Code != 500 {
		t.Errorf("an unreadable profile: %d %s", w.Code, w.Body)
	}
	if err := os.WriteFile(filepath.Join(dir, ".active"), []byte("../x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if w := f.get("/api/info"); w.Code != 500 {
		t.Errorf("a tampered record: %d %s", w.Code, w.Body)
	}
	if w := f.get("/api/profiles/uniclass"); w.Code != 500 {
		t.Errorf("a tampered record: %d %s", w.Code, w.Body)
	}
}

func TestProfilesNeedALibrary(t *testing.T) {
	f := newFixture(t)
	if w := f.get("/api/profiles/acme"); w.Code != 404 {
		t.Fatalf("no profiles without a library: %d", w.Code)
	}
	if info := decode[Info](t, f.get("/api/info")); info.Profiles {
		t.Fatalf("%+v", info)
	}
}

// TestRetrace is a layer's new role taking effect: the draft goes back to
// the queue with no note, so the tracer starts afresh with the profile.
func TestRetrace(t *testing.T) {
	f := profileFixture(t)
	src := filepath.Join(t.TempDir(), "a.png")
	if err := os.WriteFile(src, pngBytes(t, 4, 4, 0), 0o600); err != nil {
		t.Fatal(err)
	}
	store := f.srv.Store
	e, _, _ := store.Ingest(src)
	_ = store.Transition(e.ID, catalog.Tracing, nil)
	_ = store.Transition(e.ID, catalog.Failed, func(x *catalog.Entry) { x.Error, x.Note = "no walls", "old" })
	w := f.post("/api/plans/"+e.ID+"/retrace", "", nil)
	got := decode[reviewReply](t, w)
	if w.Code != 200 || got.Plan.Status != catalog.Queued || got.Plan.Error != "" || got.Plan.Note != "" ||
		f.wakes.Load() != 1 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if w := f.post("/api/plans/"+e.ID+"/retrace", "", nil); w.Code != 409 {
		t.Fatalf("a queued plan is already going to be retraced: %d", w.Code)
	}
}
