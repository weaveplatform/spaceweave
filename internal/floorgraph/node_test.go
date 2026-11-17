package floorgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/plan"
)

// The site's own validator, on the plans built from the golden evidence and
// the synthetic cases: what the worker publishes must pass it.
func TestSiteValidator(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	v := plan.NodeValidator{
		Node:   node,
		Script: filepath.Join("..", "..", "web", "floorplan-iso", "tools", "validate.mjs"),
	}
	inputs := map[string]Input{}
	for _, name := range []string{"office", "corporate"} {
		inputs[name] = evidence(loadTrace(t, name))
	}
	for name, in := range synthetic() {
		inputs[name] = in
	}
	// And a few random ones, as the fuzz test makes them, that build.
	rng := rand.New(rand.NewPCG(5, 6))
	for i := 0; i < 200 && len(inputs) < 17; i++ {
		data := make([]byte, 120)
		for k := range data {
			data[k] = byte(rng.IntN(256))
		}
		if in := fuzzInput(data); func() bool { _, err := Build(in); return err == nil }() {
			inputs[fmt.Sprintf("random-%d", i)] = in
		}
	}
	for name, in := range inputs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := mustBuild(t, in)
			p, err := plan.Convert(
				r.Trace,
				plan.Meta{URL: "generated/x/source.png", Credit: "test", SizePx: in.Size},
			)
			if err != nil {
				t.Fatal(err)
			}
			b, err := json.Marshal(p)
			if err != nil {
				t.Fatal(err)
			}
			errs, err := v.Validate(context.Background(), b)
			if err != nil || len(errs) > 0 {
				t.Fatalf("validator: %v %v", errs, err)
			}
		})
	}
}
