package cadfront

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/weaveplatform/spaceweave/internal/plan"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// The real-drawing eval's checks for three ways a trace goes visibly wrong,
// each scored from the drawing, not from what the tracer decided:
//
//   - a void floored: a room-name label that says the floor is open (an
//     atrium on an upper floor) inside a traced room, and the area of the
//     CAFM areas round it that the rooms cover;
//   - the façade cut into rooms: traced rooms under facadeEvalWidth wide
//     that touch an exterior wall;
//   - a room named from the wrong text: named rooms whose name is no
//     room-name or room-number text of the drawing.
//
// It keeps to its own vocabulary, independent of the tracer's.

// voidTruth is a room-name label for a hole in the floor. An atrium's
// ground floor is a floor, so the eval counts these only where the CAFM
// drawing has no area round the label: CAFM measures floor, not holes.
var voidTruth = regexp.MustCompile(`(?i)^\s*(atrium|void|open to below|light ?well)\b`)

// facadeEvalWidth is the narrowest a real room along the façade is, mm.
const facadeEvalWidth = 1200.0

type defects struct {
	voidLabels, voidFloored int
	voidM2                  float64 // traced void area, m²
	narrow                  int
	named, offName          int
	offNames                []string
}

func (d defects) String() string {
	return fmt.Sprintf("defects: void labels floored %d/%d, traced void area %.0f m²; "+
		"façade rooms under %.1f m wide %d; named rooms not named by room-name text %d/%d %q",
		d.voidFloored, d.voidLabels, d.voidM2, facadeEvalWidth/1000, d.narrow, d.offName, d.named, d.offNames)
}

func scoreDefects(at arexTruth, areas []truthArea, tr *plan.Trace, inv vector.Affine) defects {
	var d defects
	var rooms []tracedRoom
	for _, r := range tr.Rooms {
		x := tracedRoom{name: r.Name}
		for _, q := range r.Polygon {
			p := inv.Apply(point{X: q[0], Y: q[1]})
			x.poly = append(x.poly, p)
			x.box = x.box.Extend(p)
		}
		rooms = append(rooms, x)
	}
	inRoom := func(p point) bool {
		for _, r := range rooms {
			if boxHas(r.box, p) && insidePoly(r.poly, p) {
				return true
			}
		}
		return false
	}
	for _, l := range at.labels {
		if !voidTruth.MatchString(l.text) {
			continue
		}
		measured := false
		for _, a := range areas {
			measured = measured || boxHas(a.box, l.at) && insidePoly(a.poly, l.at)
		}
		if measured {
			continue
		}
		d.voidLabels++
		if inRoom(l.at) {
			d.voidFloored++
		}
	}
	for _, v := range voidsOf(tr) {
		var poly []point
		for _, q := range v {
			poly = append(poly, inv.Apply(point{X: q[0], Y: q[1]}))
		}
		d.voidM2 += polyArea2(poly) / 1e6
	}

	tw := tracedWalls(tr, inv)
	for _, r := range rooms {
		perim := 0.0
		for i := range r.poly {
			perim += r.poly[(i+1)%len(r.poly)].Sub(r.poly[i]).Len()
		}
		area := polyArea2(r.poly)
		// The narrower side of the rectangle with this area and perimeter.
		width := perim / 4
		if disc := perim*perim/16 - area; disc > 0 {
			width = perim/4 - math.Sqrt(disc)
		}
		if width >= facadeEvalWidth {
			continue
		}
		touches := false
		for _, w := range tw {
			if w.kind != "exterior" {
				continue
			}
			for _, p := range r.poly {
				touches = touches || segDist2(p, w.a, w.b) <= w.thick/2+100
			}
		}
		if touches {
			d.narrow++
		}
	}

	numbered := regexp.MustCompile(`^Room \d+$`)
	for _, r := range rooms {
		if numbered.MatchString(r.name) || r.name == "Open plan" {
			continue
		}
		d.named++
		ok := false
		tn := normName(r.name)
		for _, l := range at.labels {
			ln := normName(l.text)
			ok = ok || ln != "" && (ln == tn || strings.HasPrefix(ln, tn) || strings.Contains(tn, ln))
		}
		if !ok {
			d.offName++
			if len(d.offNames) < 8 {
				d.offNames = append(d.offNames, r.name)
			}
		}
	}
	return d
}

func voidsOf(tr *plan.Trace) [][][2]float64 { return tr.Voids }
