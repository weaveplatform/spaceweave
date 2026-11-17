package cadfront

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/dwg"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// TestRealSpaces places each floor's space plan on its architectural
// drawing, from the same SPACEWEAVE_DWG_DIR as TestRealCAD, and reports how
// well: the client's two sets of drawings share coordinates, so every floor
// should be placed as it is, with nearly all its outlines' corners on
// walls. Nothing about the drawings is written anywhere but the test log.
func TestRealSpaces(t *testing.T) {
	dir := os.Getenv("SPACEWEAVE_DWG_DIR")
	if dir == "" {
		t.Skip("set SPACEWEAVE_DWG_DIR to a directory with arex/ and space/ drawings")
	}
	arex, _ := filepath.Glob(filepath.Join(dir, "arex", "*.dwg"))
	spaces, _ := filepath.Glob(filepath.Join(dir, "space", "*.dwg"))
	byFloor := map[string]string{}
	for _, s := range spaces {
		byFloor[floorOf(s)] = s
	}
	slices.Sort(arex)
	var rows []string
	for _, a := range arex {
		s, ok := byFloor[floorOf(a)]
		if !ok {
			continue
		}
		doc, err := dwg.ReadFile(a)
		if err != nil {
			t.Fatal(err)
		}
		o, err := Frame(doc, vector.Options{})
		if err != nil {
			t.Fatal(err)
		}
		arch, err := Read(doc, o)
		if err != nil {
			t.Fatal(err)
		}
		got, reg, err := ReadSpaces(s, o, arch)
		if err != nil {
			t.Fatalf("floor %s: %v", floorOf(a), err)
		}
		named, zones := 0, 0
		for _, sp := range got {
			if sp.Name != "" || sp.Number != "" {
				named++
			}
			if sp.Kind == "zone" {
				zones++
			}
		}
		rows = append(
			rows,
			fmt.Sprintf(
				"floor %s: %d spaces (%d numbered or named, %d zones) on %q, %s, "+
					"%.1f%% of corners on walls, residual %.3f m, poor %v; %s",
				floorOf(
					a,
				),
				len(got),
				named,
				zones,
				reg.Layer,
				reg.Method,
				100*reg.Matched,
				reg.ResidualM,
				reg.Poor,
				strings.Join(reg.Notes, " "),
			),
		)
		if reg.Method != Shared || reg.Poor {
			t.Errorf("floor %s: the client's drawings share coordinates: %+v", floorOf(a), reg)
		}
		// The same space plan 40 m east and 25 m north of the drawing, as
		// if exported from its own origin, is fitted back onto the walls.
		moved := o
		moved.View = vector.Translate(-40000, -25000).Then(o.View)
		back, fitted, err := ReadSpaces(s, moved, arch)
		if err != nil {
			t.Fatal(err)
		}
		// How far the fitted corners are from the shared ones, in metres.
		sum, n := 0.0, 0
		for i := range got {
			for j, c := range got[i].Polygon {
				d := point{X: back[i].Polygon[j][0] - c[0], Y: back[i].Polygon[j][1] - c[1]}
				sum += d.X*d.X + d.Y*d.Y
				n++
			}
		}
		off := math.Sqrt(sum/float64(n)) / arch.Scale.PxPerMetre[0]
		rows = append(rows, fmt.Sprintf("  moved 40 m, 25 m: %s back to within %.3f m RMS, "+
			"%.1f%% of corners on walls, residual %.3f m, poor %v",
			fitted.Method, off, 100*fitted.Matched, fitted.ResidualM, fitted.Poor))
		if fitted.Method != Fitted || fitted.Poor || off > 0.1 {
			t.Errorf("floor %s: moved away, it is fitted back: %+v", floorOf(a), fitted)
		}
	}
	t.Log("\n" + strings.Join(rows, "\n"))
}
