package cadfront

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/weaveplatform/spaceweave/internal/autotrace/score"
	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/dwg"
	"github.com/weaveplatform/spaceweave/internal/floorgraph"
	"github.com/weaveplatform/spaceweave/internal/plan"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// TestQualityTrials keeps architecture-only reconstruction distinct from
// preservation of supplied space geometry. It emits portable JSON rows;
// private drawings and overlays stay in the caller's scratch directory.
func TestQualityTrials(t *testing.T) {
	dir := os.Getenv("SPACEWEAVE_DWG_DIR")
	if dir == "" {
		t.Skip("set SPACEWEAVE_DWG_DIR")
	}
	paths, _ := filepath.Glob(filepath.Join(dir, "arex", "*.dwg"))
	spaceFiles, _ := filepath.Glob(filepath.Join(dir, "space", "*.dwg"))
	if len(paths) == 0 || len(spaceFiles) == 0 {
		t.Fatal("need architectural and space drawings")
	}
	byFloor := map[string]string{}
	for _, p := range spaceFiles {
		byFloor[floorOf(p)] = p
	}
	slices.Sort(paths)
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
			drawing, err := cad.Draw(doc)
			if err != nil {
				t.Fatal(err)
			}
			ras, err := vector.Rasterize(drawing, vector.Options{})
			if err != nil {
				t.Fatal(err)
			}
			o, err := Frame(doc, vector.Options{})
			if err != nil {
				t.Fatal(err)
			}
			input, err := Read(doc, o)
			if err != nil {
				t.Fatal(err)
			}
			spaces, reg, err := ReadSpaces(byFloor[floor], o, input)
			if err != nil {
				t.Fatal(err)
			}
			if reg.Poor {
				t.Fatal("registration rejected")
			}
			want := &plan.Trace{}
			for _, sp := range spaces {
				want.Rooms = append(
					want.Rooms,
					plan.Room{
						Name:    spaceDisplayName(sp),
						Number:  sp.Number,
						Kind:    sp.Kind,
						Polygon: sp.Polygon,
					},
				)
			}
			inv, _ := inverse(o.View)
			truth := readArexTruth(doc)
			conflicts := score.SpaceConflicts(want)
			for _, trial := range []struct {
				name     string
				mode     floorgraph.SpaceSnap
				supplied bool
				tails    bool
			}{
				{"architecture", floorgraph.SpaceSnapWalls, false, false},
				{"package-walls", floorgraph.SpaceSnapWalls, true, false},
				{"package-none", floorgraph.SpaceSnapNone, true, false},
				{"package-300mm", floorgraph.SpaceSnap300mm, true, false},
				{"package-wall-continuations", floorgraph.SpaceSnapNone, true, true},
			} {
				t.Run(trial.name, func(t *testing.T) {
					in := input
					if trial.tails {
						opts := o
						opts.WallContinuations = true
						in, err = Read(doc, opts)
						if err != nil {
							t.Fatal(err)
						}
					}
					if trial.supplied {
						in.Spaces = spaces
						in.SpaceSnap = trial.mode
					}
					start := time.Now()
					result, err := floorgraph.Build(in)
					if err != nil {
						if trial.mode == floorgraph.SpaceSnapNone {
							t.Fatal(err)
						}
						rejected, _ := json.Marshal(
							map[string]string{
								"floor": floor,
								"mode":  trial.name,
								"valid": "rejected: " + err.Error(),
							},
						)
						t.Log("QUALITY " + string(rejected))
						return
					}
					valid := finalize(t, result.Trace, ras)
					if valid != "ok" && trial.mode == floorgraph.SpaceSnapNone {
						t.Errorf("validation: %s", valid)
					}
					row := struct {
						ReferenceConflicts []score.SpaceConflict   `json:"referenceConflicts"`
						OpeningPlacement   floorgraph.OpeningStats `json:"openingPlacement"`
						Floor              string                  `json:"floor"`
						Mode               string                  `json:"mode"`
						Spaces             score.Spaces            `json:"spaces"`
						WallRecall         float64                 `json:"wallRecall"`
						WallPrecision      float64                 `json:"wallPrecision"`
						DoorRecall         float64                 `json:"doorRecall"`
						DoorPrecision      float64                 `json:"doorPrecision"`
						Seconds            float64                 `json:"seconds"`
						Valid              string                  `json:"valid"`
					}{ReferenceConflicts: conflicts, OpeningPlacement: result.Openings, Floor: floor, Mode: trial.name, Spaces: score.SpaceIdentity(result.Trace, want), Seconds: time.Since(start).Seconds(), Valid: valid}
					tw := tracedWalls(result.Trace, inv)
					row.WallRecall, row.WallPrecision = wallScores(truth.walls, tw)
					doors := evalRow{}
					scoreDoors(&doors, truth.doors, tw)
					row.DoorRecall, row.DoorPrecision = doors.doorR, doors.dP
					encoded, err := json.Marshal(row)
					if err != nil {
						t.Fatal(err)
					}
					t.Log("QUALITY " + string(encoded))
					if out := os.Getenv("SPACEWEAVE_EVAL_OUT"); out != "" {
						writeQualityPlan(t, out, floor, trial.name, result.Trace)
						if err := os.MkdirAll(out, 0o700); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(
							filepath.Join(
								out,
								fmt.Sprintf("quality-%s-%s.json", floor, trial.name),
							),
							encoded,
							0o600,
						); err != nil {
							t.Fatal(err)
						}
					}
				})
			}
		})
	}
}

func writeQualityPlan(t *testing.T, out, floor, mode string, tr *plan.Trace) {
	t.Helper()
	p, err := plan.Convert(tr, plan.Meta{URL: "source.png", Credit: "private trial"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(out, "plans")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, fmt.Sprintf("quality-%s-%s.json", floor, mode)),
		data,
		0o600,
	); err != nil {
		t.Fatal(err)
	}
}

func spaceDisplayName(sp floorgraph.Space) string {
	if sp.Name != "" {
		return sp.Name
	}
	return sp.Number
}
