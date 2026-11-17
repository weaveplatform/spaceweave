package cadfront

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"strings"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/floorgraph"
	"github.com/weaveplatform/spaceweave/internal/plan"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// Space plans. A floor package can carry the facilities team's space plan
// beside the architect's drawing: every room and open-plan area drawn as
// a closed outline, its number and name written inside. Those outlines are
// what the rooms are, but they are only useful placed on the drawing
// traced. Both usually come from the same model and share coordinates, and
// then nothing needs doing; otherwise (a space plan exported in other
// units, or moved to its own origin) it is moved onto the drawing by
// fitting its outlines' corners to the drawing's walls.

// ErrNoSpaces is returned for a space plan with no layer of room-sized
// outlines on an area layer.
var ErrNoSpaces = errors.New("cadfront: the space plan has no layer of space outlines")

// Registration methods.
const (
	// Shared means the space plan is in the drawing's coordinates already.
	Shared = "shared"
	// Fitted means it was turned and moved onto the drawing's walls.
	Fitted = "fitted"
)

// Registration is how a space plan was placed on the drawing.
type Registration struct {
	Layer  string // the layer of outlines read
	Method string // Shared or Fitted
	// Transform maps the space plan's coordinates to the drawing's.
	Transform vector.Affine
	// ResidualM is the RMS distance, in metres, from the outlines'
	// corners that lie on a wall to that wall's face; Matched is the share
	// of corners that do, or -1 when the drawing had no walls to check.
	ResidualM float64
	Matched   float64
	// Poor says the placing is not to be trusted: too few corners land on
	// a wall for the outlines to be where the drawing's rooms are.
	Poor  bool
	Notes []string
}

// Registration thresholds. A corner is on a wall when it is within onWall
// of the wall's face; outlines are drawn to faces or to centrelines, and
// both are within that of a face. Open-plan zones have corners on no wall,
// so a good placing still leaves some unmatched: goodMatch is the share
// below which the outlines are taken not to be on the drawing's walls.
const (
	onWall    = 0.3  // metres
	goodMatch = 0.7  // share of corners
	overlapAt = 0.5  // share of the smaller extent two drawings must share
	enclosed  = 0.9  // share of an outline on walls or openings for a room
	edgeStep  = 0.25 // metres between samples along an outline's edge
)

// ReadSpaces reads the space outlines of the DXF or DWG at path and places
// them on the drawing that arch was read from: o are that drawing's
// options, as given to Analyse, and arch what Analyse found in it. The
// spaces come back in source-image pixels, with how they were placed.
//
// The layer of outlines is the one areaRooms would take: of the area
// layers, the one whose room-sized outlines cover the most floor. Units
// come from the areas written in the outlines when they disagree with the
// file's own, as they do in exports that state inches over millimetres.
func ReadSpaces(
	path string,
	o Options,
	arch floorgraph.Input,
) ([]floorgraph.Space, Registration, error) {
	reg := Registration{Matched: -1}
	px := o.View.Factor()
	view, ok := inverse(o.View)
	if !ok || !(px > 0) || math.IsInf(px, 0) {
		return nil, reg, ErrView
	}
	f, err := os.Open(path) //nolint:gosec // the caller names the file to read
	if err != nil {
		return nil, reg, fmt.Errorf("open space plan: %w", err)
	}
	defer func() { _ = f.Close() }()
	doc, err := readDoc(f)
	if err != nil {
		return nil, reg, err
	}
	// Read in the space plan's own coordinates: one pixel per unit, so
	// checkUnits weighs the written areas against the file's units.
	so := Options{View: vector.Identity, Profile: o.Profile}
	if m := doc.Units.Metres(); m > 0 {
		so.PxPerMetre = 1 / m
	}
	sr := &reader{
		o: so, px: 1, byPlaced: map[*cad.Placed]*node{}, owners: map[*cad.Insert][]*node{},
		byLayer: map[string]*layerInfo{},
	}
	if _, err := cad.Walk(doc, sr.visit); err != nil {
		return nil, reg, fmt.Errorf("walk the space plan: %w", err)
	}
	sr.extents()

	archPPM := o.PxPerMetre
	if arch.Scale.Known() {
		archPPM = arch.Scale.PxPerMetre[0]
	}
	archUnit := 0.0 // metres per drawing unit; 0 when unknown
	if archPPM > 0 {
		archUnit = px / archPPM
	}
	unitless := false
	if !sr.checkUnits() {
		switch {
		case so.PxPerMetre > 0:
			sr.unit = 1 / so.PxPerMetre
		case archUnit > 0:
			sr.unit, unitless = archUnit, true
			sr.note(
				"The space plan states no units and writes no areas; each common unit was tried.",
			)
		default:
			sr.unit = 1
		}
	}
	// A supplied partial space plan may describe only one room. The
	// architectural fallback still needs several outlines to identify a layer.
	byLayer, layer, _ := sr.areaLayersMinimum(1)
	// Rooms are told from workplaces by their size, which in a drawing of
	// unknown units the first unit to make any outlines room-sized says.
	for _, u := range standardUnits {
		if !unitless || layer != "" {
			break
		}
		sr.unit = u
		byLayer, layer, _ = sr.areaLayersMinimum(1)
	}
	if layer == "" {
		return nil, reg, ErrNoSpaces
	}
	outs := byLayer[layer]
	reg.Layer = layer
	if archUnit <= 0 {
		// Without the drawing's scale its units can't be compared; the
		// space plan's are the best guess at them.
		archUnit = sr.unit
	}
	// The scales to try, in drawing units per space-plan unit: the one the
	// two files' units say, or with the space plan's unknown, every common
	// unit's.
	ks := []float64{sr.unit / archUnit}
	if unitless {
		for _, u := range standardUnits {
			ks = append(ks, u/archUnit)
		}
	}
	// The fit works in the drawing's coordinates, where a space plan that
	// matches is the same shape: the image's y runs down, so in pixels the
	// match would be a mirror image.
	fit := newFitter(arch, view, 1/archUnit)
	var frame vector.Rect
	w, h := float64(o.Size[0]), float64(o.Size[1])
	for _, c := range []point{{}, {X: w}, {Y: h}, {X: w, Y: h}} {
		frame = frame.Extend(view.Apply(c))
	}
	reg.Transform = register(&reg, outs, ks, frame, fit)
	// Areas in the units the placing found.
	sr.unit = reg.Transform.Factor() * archUnit
	reg.Notes = append(reg.Notes, sr.in.Notes...)
	return sr.spaces(outs, reg.Transform, o.View, fit), reg, nil
}

// register places the outlines, returning the transform from the space
// plan's coordinates to the drawing's. ks are the scales to try, drawing
// units per space-plan unit, the one the files' units say first; frame is
// the drawing's extent.
//
// Shared coordinates are taken when the units agree, the outlines lie over
// the drawing and their corners land on its walls. Otherwise the space plan
// is scaled, centred on the walls and fitted, and the fit is kept if it
// puts more corners on walls. The scale is never fitted: units are exact,
// and a scale left free drifts by tenths of a per cent, tens of
// centimetres across a floor, or stretches another floor's outlines over
// whichever walls they are near.
func register(
	reg *Registration,
	outs []outline,
	ks []float64,
	frame vector.Rect,
	fit *fitter,
) vector.Affine {
	var pts []point
	var box vector.Rect
	for _, out := range outs {
		pts = append(pts, out.ring...)
		box = box.Union(out.box)
	}
	shared := math.Abs(ks[0]-1) <= unitSlack && boxShare(box, frame) >= overlapAt
	best := vector.Identity
	reg.Method = Shared
	if fit.empty() {
		// Nothing to fit to: shared coordinates are believed when they
		// look right, and nothing else is.
		if !shared {
			reg.Method, reg.Poor = "", true
			reg.Notes = append(reg.Notes, "The drawing has no walls to place the space plan on.")
		}
		return best
	}
	bestMatched, bestRMS := -1.0, 0.0
	if shared {
		bestMatched, bestRMS = fit.score(pts, best)
	}
	if bestMatched < goodMatch {
		var starts []vector.Affine
		if shared {
			starts = append(starts, vector.Identity)
		}
		// Scaled, centred on the walls, then moved to where most corners
		// land on walls: the fit takes it the rest of the way, as long as
		// nothing turned it far.
		for _, k := range ks {
			shift := fit.box().Center().Sub(box.Center().Mul(k))
			starts = append(
				starts,
				fit.seeds(pts, vector.Scale(k, k).Then(vector.Translate(shift.X, shift.Y)))...)
		}
		for _, start := range starts {
			m := fit.fit(pts, start)
			matched, rms := fit.score(pts, m)
			if matched > bestMatched || matched == bestMatched && rms < bestRMS {
				best, bestMatched, bestRMS, reg.Method = m, matched, rms, Fitted
			}
		}
	}
	reg.Matched, reg.ResidualM = bestMatched, bestRMS
	reg.Poor = bestMatched < goodMatch
	return best
}

// boxShare is the share of the smaller box that the two boxes share.
func boxShare(a, b vector.Rect) float64 {
	if a.Empty() || b.Empty() {
		return 0
	}
	w := math.Min(a.Max.X, b.Max.X) - math.Max(a.Min.X, b.Min.X)
	h := math.Min(a.Max.Y, b.Max.Y) - math.Max(a.Min.Y, b.Min.Y)
	small := math.Min(a.Size().X*a.Size().Y, b.Size().X*b.Size().Y)
	if w <= 0 || h <= 0 || small <= 0 {
		return 0
	}
	return w * h / small
}

// fitter matches points to the drawing's walls and openings, in drawing
// coordinates.
type fitter struct {
	segs []floorgraph.Segment
	// g is filed for the fit's first, far-reaching rounds and fine for
	// looking a metre or two around a point, which most lookups do.
	g, fine *grid
	per     float64 // drawing units per metre
	reach   float64 // how far the first fitting round looks
}

// newFitter takes the walls and openings found in a drawing back from
// pixels to its coordinates through view.
func newFitter(arch floorgraph.Input, view vector.Affine, per float64) *fitter {
	f := &fitter{per: per}
	k := view.Factor()
	for _, w := range arch.Walls {
		f.segs = append(f.segs, floorgraph.Segment{
			A: view.Apply(w.A), B: view.Apply(w.B), Thickness: w.Thickness * k,
		})
	}
	// An opening is a stretch of wall too: a room's outline runs across
	// its door.
	for _, o := range arch.Openings {
		f.segs = append(f.segs, floorgraph.Segment{A: view.Apply(o.A), B: view.Apply(o.B)})
	}
	if len(f.segs) == 0 {
		return f
	}
	box := f.box()
	f.reach = math.Max(0.1*math.Hypot(box.Size().X, box.Size().Y), 2*per)
	// Cells of half the first round's reach: a lookup then searches a few
	// cells however far it looks.
	f.g, f.fine = newGrid(f.reach/2), newGrid(per)
	for i, s := range f.segs {
		f.g.add(i, s.A, s.B)
		f.fine.add(i, s.A, s.B)
	}
	return f
}

func (f *fitter) empty() bool { return len(f.segs) == 0 }

func (f *fitter) box() vector.Rect {
	var b vector.Rect
	for _, s := range f.segs {
		b = b.Extend(s.A).Extend(s.B)
	}
	return b
}

// nearest is the closest point to p on the centreline of a wall whose face
// is within rad of it, and the distance from p to that face (0 inside the
// wall); ok is false when there is none. Pairing with centrelines rather
// than faces gives the fit something to pull towards: inside a wall every
// face distance is 0, and the outlines could slide anywhere within it.
func (f *fitter) nearest(p point, rad float64) (point, float64, bool) {
	best, bestD, bestFace, ok := point{}, math.Inf(1), 0.0, false
	g := f.g
	if rad <= 2*f.per {
		g = f.fine
	}
	for _, i := range g.near(p, rad) {
		s := f.segs[i]
		ab := s.B.Sub(s.A)
		t := 0.0
		if l2 := dot(ab, ab); l2 > 0 {
			t = math.Max(0, math.Min(1, dot(p.Sub(s.A), ab)/l2))
		}
		c := s.A.Add(ab.Mul(t))
		d := p.Sub(c).Len()
		face := math.Max(0, d-s.Thickness/2)
		if face <= rad && d < bestD {
			best, bestD, bestFace, ok = c, d, face, true
		}
	}
	return best, bestFace, ok
}

// Fitting rounds: the reach shrinks from the first round's to onWall over
// the first shrinkRounds, so the far-off start is pulled in by everything
// near it and the last rounds by the walls the corners lie on only.
const (
	fitRounds    = 30
	shrinkRounds = 20
)

// fit is closest-point fitting (ICP): each round moves the points by m,
// pairs each with the nearest wall within the round's reach, and solves
// for the turn and shift at m's scale that best map the points onto their
// pairs. It is
// deterministic: the same points and walls give the same transform.
func (f *fitter) fit(pts []point, m vector.Affine) vector.Affine {
	scale := m.Factor()
	last := onWall * f.per
	for round := range fitRounds {
		rad := last
		if round < shrinkRounds {
			rad = f.reach * math.Pow(last/f.reach, float64(round)/shrinkRounds)
		}
		var from, to []point
		for _, p := range pts {
			if q, _, ok := f.nearest(m.Apply(p), rad); ok {
				from, to = append(from, p), append(to, q)
			}
		}
		next, ok := fitTurn(from, to, scale)
		if !ok {
			break // too few corners near a wall to say which way to move
		}
		m = next
	}
	return m
}

// The search for where to start fitting from: moves of searchStep, as far
// as the first round reaches each way, scored on at most searchPoints
// corners; the best searchKeep are fitted from.
const (
	searchStep   = 1.0 // metres
	searchPoints = 400
	searchKeep   = 3
)

// seeds are start moved to where the most corners land within a step of a
// wall, best first. Fitting from the walls' centre alone can lock onto a
// floor's repeating bays a module or two off, where many corners line up
// but not most.
func (f *fitter) seeds(pts []point, start vector.Affine) []vector.Affine {
	sample := pts
	if len(pts) > searchPoints {
		sample = make([]point, 0, searchPoints)
		for i := range searchPoints {
			sample = append(sample, pts[i*len(pts)/searchPoints])
		}
	}
	type seed struct {
		m          vector.Affine
		share      float64
		dx, dy, at int
	}
	step := searchStep * f.per
	n := int(f.reach / step)
	var all []seed
	for dy := -n; dy <= n; dy++ {
		for dx := -n; dx <= n; dx++ {
			m := start.Then(vector.Translate(float64(dx)*step, float64(dy)*step))
			share, _ := f.share(sample, m, step)
			all = append(all, seed{m, share, dx, dy, len(all)})
		}
	}
	slices.SortFunc(all, func(a, b seed) int {
		return cmp.Or(
			cmp.Compare(b.share, a.share),
			cmp.Compare(
				max(abs(a.dx), abs(a.dy)),
				max(abs(b.dx), abs(b.dy)),
			),
			cmp.Compare(a.at, b.at),
		)
	})
	out := make([]vector.Affine, 0, searchKeep)
	for _, s := range all[:min(searchKeep, len(all))] {
		out = append(out, s.m)
	}
	return out
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// score is the share of points that m puts on a wall, and their RMS
// distance from its face in metres.
func (f *fitter) score(pts []point, m vector.Affine) (float64, float64) {
	return f.share(pts, m, onWall*f.per)
}

// share is the share of points that m puts within tol of a wall's face,
// and their RMS distance from it in metres.
func (f *fitter) share(pts []point, m vector.Affine, tol float64) (float64, float64) {
	if len(pts) == 0 {
		return 0, 0
	}
	n, sum := 0, 0.0
	for _, p := range pts {
		if _, d, ok := f.nearest(m.Apply(p), tol); ok {
			n++
			sum += d * d
		}
	}
	rms := 0.0
	if n > 0 {
		rms = math.Sqrt(sum/float64(n)) / f.per
	}
	return float64(n) / float64(len(pts)), rms
}

// fitTurn is the turn and shift that, after scaling by scale, map from
// onto to with the least squared error; ok is false without two distinct
// points to say which way to turn.
func fitTurn(from, to []point, scale float64) (vector.Affine, bool) {
	if len(from) < 2 || len(from) != len(to) {
		return vector.Affine{}, false
	}
	// Centred, so sums stay well conditioned for coordinates in the
	// millions (millimetres from a site origin).
	var cf, ct point
	for i := range from {
		cf, ct = cf.Add(from[i]), ct.Add(to[i])
	}
	cf, ct = cf.Mul(1/float64(len(from))), ct.Mul(1/float64(len(to)))
	var c, s float64
	for i := range from {
		p, q := from[i].Sub(cf), to[i].Sub(ct)
		c += dot(p, q)
		s += cross(p, q)
	}
	r := math.Hypot(c, s)
	if r == 0 {
		return vector.Affine{}, false
	}
	c, s = scale*c/r, scale*s/r
	m := vector.Affine{A: c, B: s, C: -s, D: c}
	o := m.Apply(cf)
	m.E, m.F = ct.X-o.X, ct.Y-o.Y
	return m, true
}

// spaceText is what is written inside one outline.
type spaceText struct {
	names, numbers []string
	area           float64
}

// spaces turns the outlines into floorgraph.Spaces through m, with the
// text written inside each: a name, a number and an area.
func (r *reader) spaces(outs []outline, m, view vector.Affine, fit *fitter) []floorgraph.Space {
	texts := make([]spaceText, len(outs))
	identityLayers := slices.ContainsFunc(
		r.leaves,
		func(l leaf) bool { return l.info.text == textName || l.info.text == textNumber },
	)
	for i := range r.leaves {
		l := &r.leaves[i]
		if l.info.text == textNever {
			continue
		}
		lines := cad.TextLines(l.e, l.m)
		if len(lines) == 0 {
			continue
		}
		var box vector.Rect
		for _, t := range lines {
			box = box.Union(t.Bounds())
		}
		at, ok := smallestIndex(outs, box.Center())
		if !ok {
			continue
		}
		st := &texts[at]
		for _, t := range lines {
			s := strings.TrimSpace(t.Content)
			if a, ok := statedArea(s); ok {
				st.area = a
				continue
			}
			// Typed name/number layers are authoritative. Generic block
			// attributes also contain flags and desk metadata inside large zones.
			if identityLayers && l.info.text != textName && l.info.text != textNumber {
				continue
			}
			switch {
			case productSize.MatchString(s):
			case l.info.text == textName:
				st.names = append(st.names, s)
			case l.info.text == textNumber:
				st.numbers = append(st.numbers, s)
			case looksLikeName(s):
				st.names = append(st.names, s)
			default:
				st.numbers = append(st.numbers, s)
			}
		}
	}
	out := make([]floorgraph.Space, 0, len(outs))
	for i, o := range outs {
		sp := floorgraph.Space{Area: texts[i].area}
		if sp.Area == 0 {
			sp.Area = round3(o.area * r.unit * r.unit)
		}
		// Names are joined in drawing order: a name written over two
		// lines is one name.
		sp.Name = strings.Join(texts[i].names, " ")
		if len(texts[i].numbers) > 0 {
			sp.Number = texts[i].numbers[0]
		}
		ring := make([]point, len(o.ring))
		for j, p := range o.ring {
			ring[j] = m.Apply(p)
			q := view.Apply(ring[j])
			sp.Polygon = append(sp.Polygon, [2]float64{q.X, q.Y})
		}
		sp.Kind = fit.kind(ring)
		out = append(out, sp)
	}
	slices.SortStableFunc(out, func(a, b floorgraph.Space) int {
		return cmp.Or(
			cmp.Compare(a.Number, b.Number),
			cmp.Compare(a.Name, b.Name),
			cmp.Compare(
				a.Polygon[0][1],
				b.Polygon[0][1],
			),
			cmp.Compare(a.Polygon[0][0], b.Polygon[0][0]),
		)
	})
	return out
}

// smallestIndex is smallestAround's outline, by index.
func smallestIndex(outs []outline, p point) (int, bool) {
	best, found := -1, false
	for i, o := range outs {
		if p.X < o.box.Min.X || p.X > o.box.Max.X || p.Y < o.box.Min.Y || p.Y > o.box.Max.Y {
			continue
		}
		if (!found || o.area < outs[best].area) && inside([][]point{o.ring}, p) {
			best, found = i, true
		}
	}
	return best, found
}

// kind says whether an outline, in drawing coordinates, is a room or a zone: a room's
// edges run along walls and openings nearly all the way round. With no
// walls to judge by it says nothing.
func (f *fitter) kind(ring []point) string {
	if f.empty() {
		return ""
	}
	on, all := 0, 0
	for i := range ring {
		a, b := ring[i], ring[(i+1)%len(ring)]
		n := max(1, int(math.Ceil(b.Sub(a).Len()/(edgeStep*f.per))))
		for k := range n {
			p := a.Add(b.Sub(a).Mul((float64(k) + 0.5) / float64(n)))
			all++
			if _, _, ok := f.nearest(p, onWall*f.per); ok {
				on++
			}
		}
	}
	if float64(on) >= enclosed*float64(all) {
		return plan.KindRoom
	}
	return plan.KindZone
}
