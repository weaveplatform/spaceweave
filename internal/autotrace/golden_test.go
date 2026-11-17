package autotrace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"testing"

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
	jobs := map[string]trace.Job{
		"office.dxf":      drawingJob(t, dxftest.Office(4).Bytes()),
		"office-unitless": drawingJob(t, dxftest.Office(-1).Bytes()),
		"plan.dxf":        drawingJob(t, dxftest.Plan().Bytes()),
	}
	packageJob := drawingJob(t, dxftest.OfficeUnlabelled(4).Bytes())
	packageJob.Spaces = []trace.Attachment{
		spacesFile(t, "spaces.dxf", dxftest.OfficeSpaces(4, same).Bytes()),
	}
	jobs["office-package"] = packageJob
	partialJob := drawingJob(t, dxftest.Office(4).Bytes())
	partialJob.Spaces = []trace.Attachment{
		spacesFile(
			t,
			"partial.dxf",
			dxftest.SpacePlan(4, same, dxftest.OfficeSpaceList[:2]).Bytes(),
		),
	}
	jobs["office-partial-package"] = partialJob
	// The images take over a minute under the race detector, which with
	// coverage pushes the package past go test's ten-minute limit. The
	// drawings still take every path through floorgraph there, and `make
	// eval` (run on every CI operating system, without -race) checks the
	// image hashes.
	if !raceDetector {
		jobs["office.png"], _ = imageJob(t, "office")
		jobs["corporate.png"], _ = imageJob(t, "corporate")
	}
	return jobs
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
		if raceDetector {
			t.Fatal("-update would drop the image hashes under -race; run it without")
		}
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
