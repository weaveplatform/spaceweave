package autotrace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/autotrace/score"
	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
	"github.com/weaveplatform/spaceweave/internal/trace"
)

var update = flag.Bool("update", false, "rewrite testdata/golden.json")

const goldenFile = "testdata/golden.json"

// golden is every fixture the hashes cover. Each is traced on every CI
// operating system, and arm64 (macOS) may fuse a multiply and add where
// amd64 doesn't: a hash that differs between them means the rounding in
// floorgraph or a front end isn't holding, and "the same file gives the
// same plan" is only true per machine.
func golden(t *testing.T) map[string]trace.Job {
	t.Helper()
	office, _ := imageJob(t, "office")
	corporate, _ := imageJob(t, "corporate")
	return map[string]trace.Job{
		"office.dxf":      drawingJob(t, dxftest.Office(4).Bytes()),
		"office-unitless": drawingJob(t, dxftest.Office(-1).Bytes()),
		"plan.dxf":        drawingJob(t, dxftest.Plan().Bytes()),
		"office.png":      office,
		"corporate.png":   corporate,
	}
}

func TestGoldenHashes(t *testing.T) {
	got := map[string]string{}
	for name, job := range golden(t) {
		res, err := Run(context.Background(), trace.Options{Validator: verdict{}}, job)
		if err != nil {
			t.Fatalf("%s: %v %q", name, err, res.Problems)
		}
		sum := sha256.Sum256(res.PlanJSON)
		got[name] = hex.EncodeToString(sum[:])
	}
	if *update {
		b, _ := json.MarshalIndent(got, "", "  ")
		if err := os.WriteFile(goldenFile, append(b, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	b, err := os.ReadFile(goldenFile)
	if err != nil {
		t.Fatalf("%v (run go test ./internal/autotrace -run Golden -update)", err)
	}
	var want map[string]string
	if err := json.Unmarshal(b, &want); err != nil {
		t.Fatal(err)
	}
	for name, sum := range got {
		if want[name] != sum {
			t.Errorf("%s: plan.json hash %s, want %s. If the tracer's rules changed on purpose, "+
				"bump autotrace.Version and run with -update; otherwise this machine traces differently.",
				name, sum[:12], want[name][:min(12, len(want[name]))])
		}
	}
}

// TestAccuracy scores the image tracer against the hand traces the test
// images were drawn from, and logs the table `make eval` prints. The floors
// are what this version reaches, so a change that loses accuracy fails.
func TestAccuracy(t *testing.T) {
	floors := map[string]float64{"office": 0.95, "corporate": 0.75}
	var rows []string
	for _, name := range slices.Sorted(maps.Keys(floors)) {
		job, want := imageJob(t, name)
		res, err := Run(context.Background(), trace.Options{Validator: verdict{}}, job)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		recall, precision := score.Walls(res.Trace, want, 3)
		var opened, wantOpen int
		for _, m := range score.Openings(res.Trace, want, 8) {
			opened += m.Matched
			wantOpen += m.Want
		}
		scale := res.Trace.Calibration.PxPerMetre[0]/want.Calibration.PxPerMetre[0] - 1
		rows = append(rows, fmt.Sprintf("%-10s walls %.2f/%.2f  rooms %d/%d  openings %d/%d  scale %+.0f%%  confidence %.2f",
			name, recall, precision, len(res.Trace.Rooms), len(want.Rooms), opened, wantOpen, scale*100, res.Confidence))
		if recall < floors[name] {
			t.Errorf("%s: wall recall %.3f fell below %.2f", name, recall, floors[name])
		}
	}
	t.Logf("synthetic renders of the hand traces (walls recall/precision within 3 px):\n%s",
		strings.Join(rows, "\n"))
}
