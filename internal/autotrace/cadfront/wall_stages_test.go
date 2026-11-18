package cadfront

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/dwg"
	"github.com/weaveplatform/spaceweave/internal/floorgraph"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// TestWallStages separates CAD extraction losses from graph construction
// losses on private drawings, without incorporating those drawings in fixtures.
func TestWallStages(t *testing.T) {
	dir := os.Getenv("SPACEWEAVE_DWG_DIR")
	if dir == "" {
		t.Skip("private drawings required")
	}
	paths, _ := filepath.Glob(filepath.Join(dir, "arex", "*.dwg"))
	if len(paths) == 0 {
		t.Fatal("no architectural drawings")
	}
	for _, path := range paths {
		floor := floorOf(path)
		if only := os.Getenv(
			"SPACEWEAVE_EVAL_FLOORS",
		); only != "" &&
			!slices.Contains(strings.Split(only, ","), floor) {
			continue
		}
		t.Run(floor, func(t *testing.T) {
			doc, err := dwg.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			o, err := Frame(doc, vector.Options{})
			if err != nil {
				t.Fatal(err)
			}
			in, err := Read(doc, o)
			if err != nil {
				t.Fatal(err)
			}
			inv, _ := inverse(o.View)
			var raw []tracedWall
			for _, w := range in.Walls {
				raw = append(
					raw,
					tracedWall{
						a:     inv.Apply(w.A),
						b:     inv.Apply(w.B),
						thick: w.Thickness * inv.Factor(),
					},
				)
			}
			truth := readArexTruth(doc)
			rr, rp := wallScores(truth.walls, raw)
			built, err := floorgraph.Build(in)
			if err != nil {
				t.Fatal(err)
			}
			br, bp := wallScores(truth.walls, tracedWalls(built.Trace, inv))
			t.Logf(
				"WALLSTAGES floor=%s rawR=%.6f rawP=%.6f builtR=%.6f builtP=%.6f walls=%d",
				floor,
				rr,
				rp,
				br,
				bp,
				len(in.Walls),
			)
		})
	}
}
