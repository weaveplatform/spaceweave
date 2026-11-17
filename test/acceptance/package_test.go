package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/catalog"
	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
	"github.com/weaveplatform/spaceweave/internal/plan"
	"github.com/weaveplatform/spaceweave/internal/server"
	"github.com/weaveplatform/spaceweave/internal/trace"
)

// attach sends a file to a plan's floor package the way the page does.
func (v *serving) attach(t *testing.T, id, name, role string, data []byte) (int, string) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("role", role)
	fw, err := mw.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write(data)
	_ = mw.Close()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		v.base+"/api/plans/"+id+"/files", &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set(server.RequestHeader, "1")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close() //nolint:errcheck // test
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}

type packageReport struct {
	Attempt    int           `json:"attempt"`
	Status     string        `json:"status"`
	Conversion *struct{}     `json:"conversion"`
	Spaces     *trace.Spaces `json:"spaces"`
	Caveats    []string      `json:"caveats"`
}

func readPackageReport(t *testing.T, s *site, id string) packageReport {
	t.Helper()
	var r packageReport
	b, err := os.ReadFile(filepath.Join(s.output, id, "report.json"))
	if err != nil || json.Unmarshal(b, &r) != nil {
		t.Fatalf("report.json: %v %s", err, b)
	}
	return r
}

// traced waits for a plan's nth attempt to finish.
func (v *serving) traced(t *testing.T, id string, attempt int) catalog.Entry {
	t.Helper()
	ps := v.s.waitFor(fmt.Sprintf("attempt %d of %s", attempt, id), func(ps []catalog.Entry) bool {
		for _, p := range ps {
			if p.ID == id && p.Attempts == attempt &&
				(p.Status == catalog.Draft || p.Status == catalog.Failed) {
				return true
			}
		}
		return false
	}, v.log)
	for _, p := range ps {
		if p.ID == id {
			if p.Status != catalog.Draft {
				t.Fatalf("%+v\n%s", p, v.log)
			}
			return p
		}
	}
	return catalog.Entry{}
}

// TestFloorPackage is a space plan added to a draft from the page: the
// plan is traced again without converting its drawing again, and the
// report says how many spaces the space plan held and how they were placed
// on the drawing, whether it shares the drawing's coordinates or was
// exported in centimetres from an origin of its own. Taken out again, the
// plan is traced from its drawing alone.
func TestFloorPackage(t *testing.T) {
	v := serve(t, newAutoSite(t))
	if code, body := v.upload(
		t,
		"office.dxf",
		dxftest.OfficeUnlabelled(4).Bytes(),
	); code != http.StatusCreated {
		t.Fatalf("upload: %d %s", code, body)
	}
	ps := v.s.waitFor("the first draft", func(ps []catalog.Entry) bool {
		return len(ps) == 1 && (ps[0].Status == catalog.Draft || ps[0].Status == catalog.Failed)
	}, v.log)
	id := ps[0].ID
	v.traced(t, id, 1)

	same := func(x, y float64) (float64, float64) { return x, y }
	code, body := v.attach(
		t,
		id,
		"office space.dxf",
		catalog.RoleSpaces,
		dxftest.OfficeSpaces(4, same).Bytes(),
	)
	if code != http.StatusCreated {
		t.Fatalf("attach: %d %s", code, body)
	}
	e := v.traced(t, id, 2)
	if len(e.Files) != 2 || e.Files[1].Role != catalog.RoleSpaces ||
		e.Files[1].Name != "office space.dxf" {
		t.Fatalf("%+v", e.Files)
	}
	if _, err := os.Stat(
		filepath.Join(v.s.output, id, filepath.FromSlash(e.Files[1].Upload)),
	); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(v.s.output, id, "plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	var traced plan.Plan
	if err := json.Unmarshal(data, &traced); err != nil {
		t.Fatal(err)
	}
	for _, sp := range dxftest.OfficeSpaceList {
		if !slices.ContainsFunc(
			traced.Rooms,
			func(r plan.MRoom) bool { return r.Name == sp.Name && r.Number == sp.Number },
		) {
			t.Fatalf("missing source space %q", sp.Name)
		}
	}
	if !slices.ContainsFunc(
		traced.Rooms,
		func(r plan.MRoom) bool { return r.Name == "Team area" && r.Kind == plan.KindZone },
	) {
		t.Fatal("wall-less zone lost")
	}
	r := readPackageReport(t, v.s, id)
	if r.Conversion == nil || strings.Count(v.log.String(), "msg=converting") != 1 {
		t.Fatalf("the retrace reads the drawing already converted:\n%s", v.log)
	}
	if s := r.Spaces; s == nil || s.Spaces != len(dxftest.OfficeSpaceList) ||
		s.Method != "shared" ||
		!s.Used ||
		s.Matched < 0.99 ||
		s.ResidualM > 0.01 {
		t.Fatalf("%+v", r.Spaces)
	}

	// The same floor's space plan in centimetres, from its own origin,
	// with a header that says inches: fitted onto the walls.
	moved := func(x, y float64) (float64, float64) { return x/10 + 5000, y/10 - 3000 }
	code, body = v.attach(
		t,
		id,
		"office space (cm).dxf",
		catalog.RoleSpaces,
		dxftest.OfficeSpaces(1, moved).Bytes(),
	)
	if code != http.StatusCreated {
		t.Fatalf("attach: %d %s", code, body)
	}
	e = v.traced(t, id, 3)
	if len(e.Files) != 2 || e.Files[1].Name != "office space (cm).dxf" {
		t.Fatalf("the new space plan replaces the old: %+v", e.Files)
	}
	r = readPackageReport(t, v.s, id)
	if s := r.Spaces; s == nil || s.Method != "fitted" || !s.Used || s.Matched < 0.99 ||
		s.ResidualM > 0.02 ||
		s.Transform == nil ||
		s.Transform[0] < 9.99 ||
		s.Transform[0] > 10.01 {
		t.Fatalf("%+v", r.Spaces)
	}

	// Taken out: traced from the drawing alone.
	var out struct {
		Plan catalog.Entry `json:"plan"`
	}
	if code := v.call(
		t,
		http.MethodDelete,
		"/api/plans/"+id+"/files/"+e.Files[1].SHA256,
		"",
		&out,
	); code != 200 ||
		len(out.Plan.Files) != 1 {
		t.Fatalf("detach: %d %+v", code, out)
	}
	v.traced(t, id, 4)
	if r = readPackageReport(t, v.s, id); r.Spaces != nil {
		t.Fatalf("%+v", r.Spaces)
	}
	if code := v.call(
		t,
		http.MethodDelete,
		"/api/plans/"+id+"/files/"+e.SHA256,
		"",
		nil,
	); code != http.StatusConflict {
		t.Fatalf("the plan's own drawing stays: %d", code)
	}
}

// TestFloorPackageFromTheInbox is a floor's drawing and space plan copied
// into the inbox together: one plan, with the space plan attached and
// read.
func TestFloorPackageFromTheInbox(t *testing.T) {
	v := serve(t, newAutoSite(t))
	same := func(x, y float64) (float64, float64) { return x, y }
	if err := os.WriteFile(filepath.Join(v.s.inbox, "Level 1 - space plan.dxf"),
		dxftest.OfficeSpaces(4, same).Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(v.s.inbox, "Level 1.dxf"),
		dxftest.Office(4).Bytes(),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	ps := v.s.waitFor("the plan with its space plan read", func(ps []catalog.Entry) bool {
		if len(ps) != 1 || len(ps[0].Files) != 2 || ps[0].Status != catalog.Draft {
			return false
		}
		r, err := os.ReadFile(filepath.Join(v.s.output, ps[0].ID, "report.json"))
		return err == nil && strings.Contains(string(r), `"spaces": {`)
	}, v.log)
	if f := ps[0].Files[1]; f.Role != catalog.RoleSpaces || f.Name != "Level 1 - space plan.dxf" {
		t.Fatalf("%+v", ps[0].Files)
	}
	if r := readPackageReport(
		t,
		v.s,
		ps[0].ID,
	); r.Spaces.Spaces != len(dxftest.OfficeSpaceList) ||
		!r.Spaces.Used {
		t.Fatalf("%+v", r.Spaces)
	}
}
