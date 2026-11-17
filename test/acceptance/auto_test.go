package acceptance

import (
	"bytes"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/autotrace/synth"
	"github.com/weaveplatform/spaceweave/internal/catalog"
	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
	"github.com/weaveplatform/spaceweave/internal/plan"
)

// autoTrace traces file in a fresh key-less site with the one-off command
// and returns the draft's entry and its plan.json and report.json.
func autoTrace(t *testing.T, name string, data []byte) (catalog.Entry, []byte, map[string]any) {
	t.Helper()
	s := newAutoSite(t)
	path := filepath.Join(s.dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if out, errOut, code := s.run("trace", path); code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	ps := s.plans()
	if len(ps) != 1 || ps[0].Status != catalog.Draft || ps[0].Tracer != "auto" || ps[0].CostUSD != 0 {
		t.Fatalf("%+v", ps)
	}
	dir := filepath.Join(s.output, ps[0].ID)
	planJSON, err := os.ReadFile(filepath.Join(dir, "plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var report map[string]any
	if err := json.Unmarshal(b, &report); err != nil {
		t.Fatal(err)
	}
	if report["tracer"] != "auto" || report["model"] != nil {
		t.Errorf("report names the tracer, and no model: %s", b)
	}
	return ps[0], planJSON, report
}

// TestAutoTraceNeedsNoKey traces a drawing and an image with no API key in
// the environment, and checks a second site gets byte-identical plans.
func TestAutoTraceNeedsNoKey(t *testing.T) {
	drawing := dxftest.Office(4).Bytes()
	entry, planJSON, _ := autoTrace(t, "office.dxf", drawing)
	var p struct {
		Source struct {
			PxPerMetre float64 `json:"pxPerMetre"`
		} `json:"SOURCE"`
		Rooms []struct {
			Name string `json:"name"`
		} `json:"rooms"`
	}
	if err := json.Unmarshal(planJSON, &p); err != nil {
		t.Fatal(err)
	}
	if entry.Scale == nil || p.Source.PxPerMetre != roundTo(entry.Scale.PxPerMetre) {
		t.Errorf("the plan's scale is the drawing's: %v vs %+v", p.Source.PxPerMetre, entry.Scale)
	}
	names := make([]string, 0, len(p.Rooms))
	for _, r := range p.Rooms {
		names = append(names, r.Name)
	}
	for name := range dxftest.OfficeRooms {
		if !strings.Contains(strings.Join(names, "|"), name) {
			t.Errorf("room %q missing from %v", name, names)
		}
	}
	if _, again, _ := autoTrace(t, "office.dxf", drawing); !bytes.Equal(planJSON, again) {
		t.Error("the same drawing traced twice must give the same plan.json")
	}

	b, err := os.ReadFile(filepath.Join(repoRoot, "internal", "plan", "testdata", "office.trace.json"))
	if err != nil {
		t.Fatal(err)
	}
	var hand plan.Trace
	if err := json.Unmarshal(b, &hand); err != nil {
		t.Fatal(err)
	}
	var img bytes.Buffer
	if err := png.Encode(&img, synth.Render(&hand, synth.Options{Text: true, Furniture: true})); err != nil {
		t.Fatal(err)
	}
	_, first, _ := autoTrace(t, "office.png", img.Bytes())
	if _, again, _ := autoTrace(t, "office.png", img.Bytes()); !bytes.Equal(first, again) {
		t.Error("the same image traced twice must give the same plan.json")
	}
}

// roundTo matches plan.json's rounding of the scale.
func roundTo(v float64) float64 {
	b, _ := json.Marshal(v)
	var out float64
	_ = json.Unmarshal(b, &out)
	return out
}
