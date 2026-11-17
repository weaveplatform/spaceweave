package autotrace

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/weaveplatform/spaceweave/internal/autotrace/rasterfront"
	"github.com/weaveplatform/spaceweave/internal/autotrace/score"
	"github.com/weaveplatform/spaceweave/internal/autotrace/synth"
	"github.com/weaveplatform/spaceweave/internal/floorgraph"
	"github.com/weaveplatform/spaceweave/internal/plan"
	"github.com/weaveplatform/spaceweave/internal/raster"
	"github.com/weaveplatform/spaceweave/internal/trace"
)

// TestRasterTrials runs both unassisted and supplied-scale/label modes.
// Assistance is input to evidence building, never a refit during scoring.
func TestRasterTrials(t *testing.T) {
	if os.Getenv("SPACEWEAVE_QUALITY_TRIALS") == "" {
		t.Skip("set SPACEWEAVE_QUALITY_TRIALS=1")
	}
	for _, name := range evalPlans {
		want := readTrace(t, name)
		for _, variant := range variants {
			o := variant.o
			o.Text = true
			o.Furniture = true
			src := raster.NewSource(synth.Render(want, o))
			for _, assisted := range []bool{false, true} {
				for _, faces := range []bool{false, true} {
					mode := "automatic"
					if assisted {
						mode = "assisted"
					}
					algorithm := "bands"
					if faces {
						algorithm = "faces"
					}
					t.Run(name+"/"+variant.name+"/"+mode+"/"+algorithm, func(t *testing.T) {
						start := time.Now()
						opts := rasterfront.Options{WallFaces: faces}
						if assisted {
							opts.Scale = floorgraph.Scale{
								PxPerMetre: want.Calibration.PxPerMetre,
								Exact:      true,
								Evidence:   "supplied calibration",
							}
						}
						in, err := rasterfront.Read(src, opts)
						if err != nil {
							t.Fatal(err)
						}
						if assisted {
							for _, r := range want.Rooms {
								if r.Label != nil {
									in.Labels = append(
										in.Labels,
										floorgraph.Label{
											At:     floorgraph.Point{X: r.Label[0], Y: r.Label[1]},
											Text:   r.Name,
											Kind:   floorgraph.LabelName,
											Height: 8,
										},
									)
								}
							}
						}
						result, err := floorgraph.Build(in)
						if err != nil {
							t.Fatal(err)
						}
						f, err := trace.Finalize(
							context.Background(),
							trace.Job{
								Image: src,
								Meta: plan.Meta{
									URL:    "trial.png",
									Credit: "synthetic",
									SizePx: in.Size,
								},
							},
							result.Trace,
							validator(t),
						)
						valid := "ok"
						if err != nil {
							valid = err.Error()
						} else if !f.Valid() {
							valid = "invalid geometry"
						}
						rec, prec := score.Walls(result.Trace, want, 3)
						row := struct {
							Valid         string                        `json:"valid"`
							Plan          string                        `json:"plan"`
							Variant       string                        `json:"variant"`
							Mode          string                        `json:"mode"`
							Algorithm     string                        `json:"algorithm"`
							WallRecall    float64                       `json:"wallRecall"`
							WallPrecision float64                       `json:"wallPrecision"`
							Spaces        score.Spaces                  `json:"spaces"`
							ScaleError    [2]float64                    `json:"scaleError"`
							Seconds       float64                       `json:"seconds"`
							Openings      map[string]score.OpeningMatch `json:"openings"`
						}{Valid: valid, Plan: name, Variant: variant.name, Mode: mode, Algorithm: algorithm, WallRecall: rec, WallPrecision: prec, Spaces: score.SpaceIdentity(result.Trace, want), Seconds: time.Since(start).Seconds(), Openings: score.Openings(result.Trace, want, 8)}
						for i := range 2 {
							row.ScaleError[i] = result.Trace.Calibration.PxPerMetre[i]/want.Calibration.PxPerMetre[i] - 1
						}
						data, err := json.Marshal(row)
						if err != nil {
							t.Fatal(err)
						}
						t.Log("QUALITY " + string(data))
						if out := os.Getenv("SPACEWEAVE_EVAL_OUT"); out != "" {
							if err := os.MkdirAll(out, 0o700); err != nil {
								t.Fatal(err)
							}
							file := name + "-" + variant.name + "-" + mode + "-" + algorithm + ".json"
							if err := os.WriteFile(
								filepath.Join(out, file),
								data,
								0o600,
							); err != nil {
								t.Fatal(err)
							}
						}
					})
				}
			}
		}
	}
}
