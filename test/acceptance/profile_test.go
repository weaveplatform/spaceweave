package acceptance

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/catalog"
	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
	"github.com/weaveplatform/spaceweave/internal/server"
)

// oddLayer is a wall layer named by no convention the built-in profiles
// know, as a client's drawings might name theirs.
const oddLayer = "Q-TRENNWAND"

// call sends a request to the API the way the page does, and decodes the
// reply into v when it is given.
func (v *serving) call(t *testing.T, method, path, body string, out any) int {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, v.base+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(server.RequestHeader, "1")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close() //nolint:errcheck // test
	b, _ := io.ReadAll(res.Body)
	if out != nil {
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatalf("%s %s: %v: %s", method, path, err, b)
		}
	}
	return res.StatusCode
}

type layerReport struct {
	Profile string `json:"profile"`
	Layers  []struct {
		Name     string `json:"name"`
		Role     string `json:"role"`
		Entities int    `json:"entities"`
		Source   string `json:"source"`
	} `json:"layers"`
}

func (r layerReport) layer(t *testing.T, name string) (role, source string) {
	t.Helper()
	for _, l := range r.Layers {
		if l.Name == name {
			return l.Role, l.Source
		}
	}
	t.Fatalf("no layer %s in %+v", name, r.Layers)
	return "", ""
}

func readReport(t *testing.T, s *site, id string) (layerReport, []string) {
	t.Helper()
	var r layerReport
	b, err := os.ReadFile(filepath.Join(s.output, id, "report.json"))
	if err != nil || json.Unmarshal(b, &r) != nil {
		t.Fatalf("report.json: %v %s", err, b)
	}
	var p struct {
		Rooms []struct {
			Name string `json:"name"`
		} `json:"rooms"`
	}
	b, err = os.ReadFile(filepath.Join(s.output, id, "plan.json"))
	if err != nil || json.Unmarshal(b, &p) != nil {
		t.Fatalf("plan.json: %v", err)
	}
	var rooms []string
	for _, room := range p.Rooms {
		rooms = append(rooms, room.Name)
	}
	return r, rooms
}

// TestLayerRoleFromThePage is the layers panel end to end: a drawing whose
// partition is on a layer no profile names is traced without it, the
// report says the layer's role was a guess, and giving the layer the wall
// role through the API saves it in a client profile and retraces the plan,
// which then finds the partition and the two rooms it divides.
func TestLayerRoleFromThePage(t *testing.T) {
	v := serve(t, newAutoSite(t))
	var info server.Info
	if code := v.call(t, http.MethodGet, "/api/info", "", &info); code != 200 || !info.Profiles || info.Profile != "" {
		t.Fatalf("info: %d %+v", code, info)
	}
	if code, body := v.upload(t, "partitioned.dxf", dxftest.Partitioned(4, oddLayer).Bytes()); code != http.StatusCreated {
		t.Fatalf("upload: %d %s", code, body)
	}
	ps := v.s.waitFor("the first draft", func(ps []catalog.Entry) bool {
		return len(ps) == 1 && (ps[0].Status == catalog.Draft || ps[0].Status == catalog.Failed)
	}, v.log)
	e := ps[0]
	if e.Status != catalog.Draft {
		t.Fatalf("%+v\n%s", e, v.log)
	}
	report, rooms := readReport(t, v.s, e.ID)
	if role, source := report.layer(t, oddLayer); role != "unknown" || source != "guess" {
		t.Fatalf("the odd layer is read by what's on it, which is nothing yet: %s from %s", role, source)
	}
	if role, source := report.layer(t, "A-WALL"); role != "wall" || source != "builtin:aia" {
		t.Fatalf("A-WALL: %s from %s", role, source)
	}
	if len(rooms) != 1 {
		t.Fatalf("with no partition, West and East are one room: %q", rooms)
	}

	// What the page does when the reviewer picks "Walls" for the layer: the
	// profile (empty, not saved yet) with the layer's rule added, saved,
	// then the plan sent to be traced again.
	var got struct {
		Exists  bool `json:"exists"`
		Active  bool `json:"active"`
		Profile struct {
			Rules []map[string]any `json:"rules"`
		} `json:"profile"`
	}
	if code := v.call(t, http.MethodGet, "/api/profiles/acme", "", &got); code != 200 || got.Exists {
		t.Fatalf("not saved yet: %d %+v", code, got)
	}
	rules := `{"rules": [{"role": "wall", "layer": "` + oddLayer + `"}]}`
	if code := v.call(t, http.MethodPut, "/api/profiles/acme", rules, &got); code != 200 || !got.Active {
		t.Fatalf("save: %d %+v", code, got)
	}
	if code := v.call(t, http.MethodPut, "/api/profiles/acme", `{"rules": [{"role": "wal", "layer": "x"}]}`, nil); code != 422 {
		t.Fatalf("an invalid profile is refused: %d", code)
	}
	if code := v.call(t, http.MethodPost, "/api/plans/"+e.ID+"/retrace", "", nil); code != 200 {
		t.Fatalf("retrace: %d\n%s", code, v.log)
	}
	ps = v.s.waitFor("the retrace", func(ps []catalog.Entry) bool {
		return len(ps) == 1 && ps[0].Attempts == 2 && (ps[0].Status == catalog.Draft || ps[0].Status == catalog.Failed)
	}, v.log)
	if ps[0].Status != catalog.Draft {
		t.Fatalf("%+v\n%s", ps[0], v.log)
	}
	report, rooms = readReport(t, v.s, e.ID)
	if role, source := report.layer(t, oddLayer); role != "wall" || source != "profile:acme" || report.Profile != "acme" {
		t.Fatalf("the retrace reads the saved role: %s from %s (profile %q)", role, source, report.Profile)
	}
	if len(rooms) != 2 || !strings.Contains(strings.Join(rooms, "|"), "West") ||
		!strings.Contains(strings.Join(rooms, "|"), "East") {
		t.Fatalf("the partition divides West from East: %q", rooms)
	}
	b, err := os.ReadFile(filepath.Join(v.s.output, "profiles", "acme.yaml"))
	if err != nil || !strings.Contains(string(b), oddLayer) {
		t.Fatalf("the profile is a file beside the plans: %s %v", b, err)
	}
	if code := v.call(t, http.MethodGet, "/api/info", "", &info); code != 200 || info.Profile != "acme" {
		t.Fatalf("the first profile saved is the one in use: %+v", info)
	}
}
