package cadfront

import (
	"cmp"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/floorgraph"
	"github.com/weaveplatform/spaceweave/internal/plan"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// Area plans. A space-management (CAFM) export draws each room and each
// workplace as a closed outline with its number and area written inside,
// and often nothing else: no walls, doors or furniture. Two things follow
// from that. The written areas are a check on the drawing's units, which
// such exports are known to get wrong (a header saying inches over
// coordinates in millimetres). And with no walls to find, the room
// outlines are the only evidence of where rooms are.

// outline is a closed polyline, in drawing coordinates.
type outline struct {
	leaf int
	ring []point
	box  vector.Rect
	area float64 // square drawing units
}

func ringArea(ps []point) float64 {
	a := 0.0
	for i := range ps {
		j := (i + 1) % len(ps)
		a += ps[i].X*ps[j].Y - ps[j].X*ps[i].Y
	}
	return math.Abs(a) / 2
}

// outlines are every closed polyline outside blocks: inside one it is a
// symbol's, not an area's.
func (r *reader) outlines() []outline {
	var out []outline
	for i := range r.leaves {
		l := &r.leaves[i]
		p, ok := l.e.(*cad.Polyline)
		if !ok || l.node != nil || len(p.Vertices) < 3 {
			continue
		}
		first, last := p.Vertices[0].P, p.Vertices[len(p.Vertices)-1].P
		if !p.Closed && first != last {
			continue
		}
		var ring []point
		for _, q := range polylineSamples(p) {
			ring = append(ring, l.m.Apply(q))
		}
		if a := ringArea(ring); a > 0 {
			out = append(out, outline{leaf: i, ring: ring, box: l.box, area: a})
		}
	}
	return out
}

// areaText is an area written out: a number and a unit of area.
var areaText = regexp.MustCompile(
	`(?i)^\s*(\d{1,3}(?:,\d{3})+|\d+)(\.\d+)?\s*` +
		`(sf|sq\.?\s*ft\.?|ft2|ft²|square\s+feet|m2|m²|sq\.?\s*m|sqm)\s*$`,
)

const sqFt = 0.09290304 // square metres

// statedArea reads "16.8 sf" or "1,204 m²" as square metres.
func statedArea(s string) (float64, bool) {
	m := areaText.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", "")+m[2], 64)
	if err != nil || v <= 0 {
		return 0, false
	}
	u := strings.ToLower(m[3])
	if strings.Contains(u, "f") {
		v *= sqFt
	}
	return v, true
}

// Area labels must agree before they outvote the drawing's units: at
// least minAreaLabels of them, and a third of all, within areaSlack.
// A drawing's own units are kept when the labels agree with them to within
// unitSlack, which allows for areas rounded and measured to wall faces.
const (
	minAreaLabels = 3
	areaSlack     = 0.1
	unitSlack     = 0.1
)

// areaUnit is the metres per drawing unit that the areas written inside
// outlines imply, and how many labels said so; 0 when they don't agree.
func (r *reader) areaUnit(outs []outline) (float64, int) {
	var us []float64
	for i := range r.leaves {
		l := &r.leaves[i]
		if l.node != nil {
			continue
		}
		for _, t := range cad.TextLines(l.e, l.m) {
			m2, ok := statedArea(t.Content)
			if !ok {
				continue
			}
			if o, ok := smallestAround(outs, t.Bounds().Center()); ok {
				us = append(us, math.Sqrt(m2/o.area))
			}
		}
	}
	// The commonest unit, not the median: a label set beside its outline
	// rather than in it lands in the room around, and says nothing.
	slices.Sort(us)
	best, agree := 0.0, 0
	for i, u := range us {
		n := 0
		for _, v := range us[i:] {
			// Areas go with the square of the unit.
			if v*v > u*u*(1+areaSlack) {
				break
			}
			n++
		}
		if n > agree {
			best, agree = us[i+n/2], n
		}
	}
	if agree < minAreaLabels || 3*agree < len(us) {
		return 0, 0
	}
	// Written areas are rounded; a unit they put within a couple of per
	// cent of a real one is that one.
	for _, u := range standardUnits {
		if math.Abs(best/u-1) <= snapUnit {
			return u, agree
		}
	}
	return best, agree
}

// standardUnits are metres per millimetre, centimetre, inch, foot, yard
// and metre.
var standardUnits = []float64{0.001, 0.01, 0.0254, 0.3048, 0.9144, 1}

const snapUnit = 0.02

func smallestAround(outs []outline, p point) (outline, bool) {
	best, found := outline{}, false
	for _, o := range outs {
		if p.X < o.box.Min.X || p.X > o.box.Max.X || p.Y < o.box.Min.Y || p.Y > o.box.Max.Y {
			continue
		}
		if (!found || o.area < best.area) && inside([][]point{o.ring}, p) {
			best, found = o, true
		}
	}
	return best, found
}

// checkUnits believes the written areas over the drawing's units when they
// disagree, and reports whether it set the scale.
func (r *reader) checkUnits() bool {
	u, n := r.areaUnit(r.outlines())
	if u <= 0 {
		return false
	}
	if r.o.PxPerMetre > 0 {
		stated := r.px / r.o.PxPerMetre
		if math.Abs(u/stated-1) <= unitSlack {
			return false
		}
		r.note(
			"The drawing's units make it %.0f m across, but the areas written in %d of its "+
				"outlines make it %.0f m; the written areas were believed.",
			r.extent()*stated, n, r.extent()*u,
		)
	}
	r.unit, r.known = u, true
	p := r.px / u
	r.in.Scale = floorgraph.Scale{
		PxPerMetre: [2]float64{p, p},
		Evidence:   "from the areas written in " + strconv.Itoa(n) + " outlines",
	}
	return true
}

// extent is the drawing's longer side, in drawing units.
func (r *reader) extent() float64 {
	var all vector.Rect
	for i := range r.leaves {
		all = all.Union(r.leaves[i].box)
	}
	if all.Empty() {
		return 0
	}
	return math.Max(all.Size().X, all.Size().Y)
}

// minRoomArea is the least median area, in square metres, of a layer of
// room outlines: workplace outlines are desk-sized and smaller.
const minRoomArea = 2.0

// minDeskArea is the least median area, in square metres, of a layer of
// workplace outlines; smaller ones are symbols.
const minDeskArea = 0.5

// areaRooms falls back on room outlines when the drawing has no line work
// that could be a wall. Of the layers of room-sized outlines, the one
// covering most floor is taken (desk-sized ones are workplaces), and each
// outline's edges become walls: where two rooms' outlines run a wall's
// thickness apart they pair into that wall, and where they share an edge
// it is one thin wall. That last is a boundary the export draws between
// open areas, not always a wall built; the note says so.
func (r *reader) areaRooms() bool {
	for i := range r.leaves {
		l := &r.leaves[i]
		if ok, _ := r.candidate(l, false); !ok {
			continue
		}
		switch l.e.(type) {
		case *cad.Line, *cad.Polyline, *cad.Arc:
			return false
		}
	}
	byLayer := map[string][]outline{}
	for _, o := range r.outlines() {
		l := &r.leaves[o.leaf]
		if l.owner == nil && l.info.area {
			byLayer[l.layer] = append(byLayer[l.layer], o)
		}
	}
	best, bestArea := "", 0.0
	desks, deskN := "", 0
	for layer, os := range byLayer {
		if len(os) < minAreaLabels {
			continue
		}
		as := make([]float64, len(os))
		total := 0.0
		for i, o := range os {
			as[i] = o.area * r.unit * r.unit
			total += as[i]
		}
		slices.Sort(as)
		med := as[len(as)/2]
		switch {
		case med >= minRoomArea:
			// The layer that covers most of the floor: rooms tile it,
			// where a layer of bookable spaces leaves the cores out.
			if total > bestArea || total == bestArea && cmp.Less(layer, best) {
				best, bestArea = layer, total
			}
		case med >= minDeskArea:
			// Workplaces: the outlines a desk is booked by, desk-sized.
			if len(os) > deskN || len(os) == deskN && cmp.Less(layer, desks) {
				desks, deskN = layer, len(os)
			}
		}
	}
	if best == "" {
		return false
	}
	if r.known {
		r.desks = byLayer[desks]
	}
	for _, o := range byLayer[best] {
		r.leaves[o.leaf].area = true
	}
	r.areaPlan = true
	r.note(
		"The drawing has no walls; rooms were read from the %d outlines on layer %q, their edges "+
			"taken as walls. Where two areas meet with no wall drawn, the boundary between them "+
			"is a wall in the plan too.",
		len(byLayer[best]), best,
	)
	return true
}

// findDesks places a desk on each workplace outline of an area plan, as
// long as the outline's longest edge and as deep as it reaches across.
func (r *reader) findDesks() {
	n := 0
	for _, o := range r.desks {
		var along point
		for i := range o.ring {
			if e := o.ring[(i+1)%len(o.ring)].Sub(o.ring[i]); e.Len() > along.Len() {
				along = e
			}
		}
		u := unit(along)
		across := point{X: -u.Y, Y: u.X}
		lo, hi := point{X: math.Inf(1), Y: math.Inf(1)}, point{X: math.Inf(-1), Y: math.Inf(-1)}
		for _, p := range o.ring {
			s, t := dot(p, u), dot(p, across)
			lo = point{X: math.Min(lo.X, s), Y: math.Min(lo.Y, t)}
			hi = point{X: math.Max(hi.X, s), Y: math.Max(hi.Y, t)}
		}
		w, d := round3((hi.X-lo.X)*r.unit), round3((hi.Y-lo.Y)*r.unit)
		if w < minItem || d < minItem || w > maxItem || d > maxItem {
			continue
		}
		mid := u.Mul((lo.X + hi.X) / 2).Add(across.Mul((lo.Y + hi.Y) / 2))
		at := r.toPx(mid)
		r.in.Furniture = append(r.in.Furniture, plan.Item{
			Type: "desk", X: at.X, Y: at.Y, W: ptr(w), D: ptr(d),
			Rot: ptr(imageAngle(r.o.View, u)),
		})
		r.leaves[o.leaf].used = true
		n++
	}
	if n > 0 {
		r.note("%d workplace outlines were placed as desks of their size.", n)
	}
}

// deskLabel says a label sits in a workplace outline: the desk's number
// or what it is, not a room's name.
func (r *reader) deskLabel(at point) bool {
	_, in := smallestAround(r.desks, at)
	return in
}
