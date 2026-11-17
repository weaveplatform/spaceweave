package dxftest

import (
	"fmt"
	"math"
)

// OfficeSpace is one area of OfficeSpaces, its corners in Office's
// millimetres.
type OfficeSpace struct {
	Number, Name string
	X0, Y0       float64
	X1, Y1       float64
}

// OfficeSpaceList are OfficeSpaces' areas. Office's open office is two
// areas here, split at x = 3000 where no wall stands: open-plan zones,
// where the other three are rooms walled all round.
var OfficeSpaceList = []OfficeSpace{
	{"0.01", "Open office", 0, 0, 3000, 4000},
	{"0.02", "Team area", 3000, 0, 6000, 4000},
	{"0.03", "Meeting", 0, 4000, 6000, 8000},
	{"0.04", "Kitchen", 6000, 0, 12000, 4000},
	{"0.05", "Director", 6000, 4000, 12000, 8000},
}

// OfficeSpaces is Office's space plan, as a facilities system exports one:
// each area of OfficeSpaceList a closed outline on its wall centrelines
// (EC1 Space Polygons), with its number, name and area written inside on
// layers of their own, and nothing else. place maps Office's millimetres to
// the space plan's coordinates, so the same plan can be drawn in Office's
// coordinates or moved, scaled and turned away from them; units is
// $INSUNITS, which an export may state wrongly.
func OfficeSpaces(units int, place func(x, y float64) (float64, float64)) *Doc {
	d := New(units).Layer("EC1 Space Polygons", 3, 0).Layer("EC1 Space Numbers", 7, 0).
		Layer("EC1 Space Names", 7, 0).Layer("EC1 Space Areas", 7, 0)
	for _, s := range OfficeSpaceList {
		p := []any{8, "EC1 Space Polygons", 90, 4, 70, 1}
		for _, c := range [][2]float64{{s.X0, s.Y0}, {s.X1, s.Y0}, {s.X1, s.Y1}, {s.X0, s.Y1}} {
			x, y := place(c[0], c[1])
			p = append(p, 10, x, 20, y)
		}
		d.Add(E("LWPOLYLINE", p...))
		cx, cy := (s.X0+s.X1)/2, (s.Y0+s.Y1)/2
		// Lettering 200 mm high in Office's millimetres, however place
		// scales them.
		x0, y0 := place(0, 0)
		x1, y1 := place(200, 0)
		h := math.Hypot(x1-x0, y1-y0)
		for i, t := range []struct{ layer, text string }{
			{"EC1 Space Names", s.Name},
			{"EC1 Space Numbers", s.Number},
			{"EC1 Space Areas", fmt.Sprintf("%.1f m2", (s.X1-s.X0)*(s.Y1-s.Y0)/1e6)},
		} {
			x, y := place(cx-800, cy+400-float64(i)*400)
			d.Add(Text(t.layer, x, y, h, t.text))
		}
	}
	return d
}
