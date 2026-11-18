package cadfront

import (
	"math"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
	"github.com/weaveplatform/spaceweave/internal/floorgraph"
)

func TestMeasuredWallJunction(t *testing.T) {
	d := dxftest.New(4).Layer("A-WALL", 7, 0).Add(
		dxftest.Rect("A-WALL", -100, -100, 12100, 8100),
		dxftest.Rect("A-WALL", 100, 100, 11900, 7900),
		dxftest.Rect("A-WALL", 0, 3940, 6000, 4060),
		dxftest.Rect("A-WALL", 6000, 3820, 12000, 4180),
	)
	in, o := read(t, d)
	result, err := floorgraph.Build(in)
	if err != nil {
		t.Fatal(err)
	}
	inv, _ := inverse(o.View)
	walls := tracedWalls(result.Trace, inv)
	for _, want := range []struct{ x, thickness float64 }{{3000, 120}, {9000, 360}} {
		found := false
		for _, w := range walls {
			if segDist2(point{X: want.x, Y: 4000}, w.a, w.b) > 5 {
				continue
			}
			found = true
			if math.Abs(w.thick-want.thickness) > 2 {
				t.Errorf("partition thickness %g mm, want %g mm", w.thick, want.thickness)
			}
		}
		if !found {
			t.Errorf("no wall at synthetic junction sample %g", want.x)
		}
	}
}
