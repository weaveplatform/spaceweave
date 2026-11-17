package floorgraph

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/weaveplatform/spaceweave/internal/plan"
)

// ErrNoWalls is returned when the evidence holds no usable wall.
var ErrNoWalls = errors.New("floorgraph: no usable walls in the evidence")

// ErrFlat is returned when the walls span less than minSpanM one way: a
// line of walls encloses no floor, and the site can't draw a building with
// no depth.
var ErrFlat = errors.New("floorgraph: the walls lie on a line and enclose no floor")

// ErrInconsistent is returned if the trace built fails plan.Check: a bug
// here, not a problem with the evidence.
var ErrInconsistent = errors.New("floorgraph: built trace fails plan.Check")

// minSpanM is the least width and depth of the walls' bounding box, metres:
// the site validator allows wall ends 0.2 m outside it.
const minSpanM = 0.25

// minWallM is the shortest wall kept, metres: the site's validator rejects
// walls under 5 cm, and plan.json rounds to the millimetre.
const minWallM = 0.06

// Build turns the evidence into a trace. It is deterministic: the same input,
// in any order, gives the same trace, to the byte, on any machine.
//
// The steps: snap walls to the drawing's dominant directions and merge
// collinear pieces; join the pieces either side of each opening; close
// corners and split walls where they meet into a planar graph; walk its faces.
// The largest outer face is the building's outline, its walls exterior; the
// bounded faces big enough to stand in are rooms, named from the labels inside
// them. A free-standing group of walls inside a room (a building core) is
// bridged to the wall above it, so its faces are rooms too and the room
// around it goes round it. Without a scale from the front end one is
// estimated from door widths, then doorway widths, then wall thickness.
//
// It fails with ErrNoWalls when no usable wall is left, and ErrFlat when the
// walls lie on a line.
//
// Confidence is a weighted mean of six scores in 0–1, then halved if no room
// was found: the scale's source (0.25: exact 1, measured 0.85, door widths
// 0.6, doorway widths 0.45, wall thickness 0.3, a guess 0.1); the share of
// wall length in closed loops rather than dangling (0.2); the front end's
// mean wall weight, by length (0.15); its coverage when it knows it (0.15);
// the share of rooms named from a label (0.15); the share of doors whose
// swing was seen rather than guessed (0.1).
func Build(in Input) (*Result, error) {
	b := &builder{in: in}
	if err := b.graph(); err != nil {
		return nil, err
	}
	b.findFaces()
	if err := b.rooms(); err != nil {
		return nil, err
	}
	if err := b.walls(); err != nil {
		return nil, err
	}
	t := b.trace()
	if t.Outline != nil && !b.keepOutline(t) {
		t.Outline = nil
		b.outlineDropped = true
	}
	if problems := plan.Check(t); len(problems) > 0 {
		return nil, fmt.Errorf("%w: %s", ErrInconsistent, strings.Join(problems, "; "))
	}
	return &Result{
		Trace: t,
		Openings: OpeningStats{
			Input:      len(b.ops),
			Placed:     b.pl.placed,
			Unplaced:   b.pl.unplaced,
			Narrow:     b.pl.narrow,
			Overlapped: b.pl.overlapped,
		},
		Confidence: b.confidence(),
		Summary:    b.summary(t),
		Caveats:    b.caveats(),
	}, nil
}

// builder carries the pipeline's state between steps.
type builder struct {
	in      Input
	segs    []seg
	ops     []opening
	g       *graph
	alive   []bool
	fs      []face
	outer   int // index into fs, or -1 when the walls enclose nothing
	kinds   []string
	chains  []chain
	sc      scaleInfo
	rs      []room
	outline []Point
	pl      placement

	// What the caveats report.
	badWalls, badOps, oddOps   int
	islands, overlapRooms      int
	strayLabels, unnamed, stub int
	outlineDropped             bool
	inputLen, inputWeight      float64

	// voidAt are the void points whose void was found, voidMissed how many
	// weren't; voids are the voids' polygons.
	voidAt     []Point
	voidMissed int
	voids      [][]Point
	// strips counts façade strips that would have been rooms, openPlan
	// the rooms called openPlanName.
	strips, openPlan int
	// outsideWalls were left out for standing outside the outline.
	outsideWalls int
	// spaced says the rooms were grown from labels (labelSpaces), not
	// taken from faces; shared are its caveats for rooms holding several
	// names.
	spaced bool
	shared []string
}

// graph cleans the walls and openings and builds the planar wall graph.
func (b *builder) graph() error {
	segs, bad := cleanWalls(b.in.Walls)
	b.badWalls = bad
	for _, s := range segs {
		b.inputLen += s.length()
		b.inputWeight += float64(s.w * s.length())
	}
	if len(segs) == 0 {
		return ErrNoWalls
	}
	segs = mergeCollinear(snap(segs, dominantDirections(segs)))
	b.ops, b.badOps, b.oddOps = cleanOpenings(b.in.Openings)
	segs = mergeCollinear(joinOpenings(segs, b.ops))
	b.segs = anchorEnds(closeCorners(segs))
	b.segs = append(b.segs, b.voidEdges(b.segs)...)
	b.g = node(b.segs)
	if len(b.g.edges) == 0 {
		return ErrNoWalls
	}
	return nil
}

// findFaces walks the graph's faces, picks the building's outer boundary and
// sorts the edges into kinds.
func (b *builder) findFaces() {
	g := b.g
	b.alive = g.prune()
	b.fs = g.faces(b.alive)
	var bridged int
	if b.alive, bridged = g.bridge(b.alive, b.fs); bridged > 0 {
		b.fs = g.faces(b.alive)
	}
	b.outer = -1
	for i, f := range b.fs {
		if f.area < 0 && (b.outer < 0 || f.area < b.fs[b.outer].area) {
			b.outer = i
		}
	}
	b.kinds = make([]string, len(g.edges))
	for e := range b.kinds {
		b.kinds[e] = interior
		if g.edges[e].glass {
			b.kinds[e] = glass
		}
	}
	if b.outer >= 0 {
		for _, h := range b.fs[b.outer].half {
			if !g.edges[h/2].virtual {
				b.kinds[h/2] = exterior
			}
		}
	} else {
		g.hullExterior(b.kinds)
	}
	b.chains = g.chains(b.kinds)

	var ts, ls []float64
	for _, k := range []string{interior, glass, exterior} {
		for _, c := range b.chains {
			if c.kind == k && c.t > 0 {
				ts, ls = append(ts, c.t), append(ls, c.length)
			}
		}
		// Interior walls if there are any, else whatever there is.
		if len(ts) > 0 {
			break
		}
	}
	lo, hi := g.pts[0], g.pts[0]
	for _, p := range g.pts {
		lo = Point{X: math.Min(lo.X, p.X), Y: math.Min(lo.Y, p.Y)}
		hi = Point{X: math.Max(hi.X, p.X), Y: math.Max(hi.Y, p.Y)}
	}
	b.sc = estimateScale(b.in.Scale, b.ops, ts, ls, math.Max(hi.X-lo.X, hi.Y-lo.Y))
}

// rooms reserves registered spaces and fills the remaining floor from
// label-grown regions, or graph faces when there are no labels. Inference
// never displaces a supplied space's identity or kind.
func (b *builder) rooms() error {
	if b.outer < 0 && len(b.in.Spaces) == 0 {
		return nil
	}
	if b.outer >= 0 {
		voids := b.voidFaces()
		labels := slices.Clone(b.in.Labels)
		slices.SortFunc(labels, func(x, y Label) int {
			if c := cmpPoint(x.At, y.At); c != 0 {
				return c
			}
			if c := cmpFloat(x.Height, y.Height); c != 0 {
				return c
			}
			return cmpString(x.Text, y.Text)
		})
		labels = slices.DeleteFunc(labels, func(l Label) bool {
			// A void's own label ("ATRIUM") names no room.
			return slices.ContainsFunc(b.voids, func(v []Point) bool { return inside(l.At, v) })
		})
		if !b.labelSpaces(labels, voids) {
			b.faceRooms(voids)
			b.strayLabels = nameRooms(b.rs, labels)
		}
	}
	if err := b.packageSpaces(); err != nil {
		return err
	}
	sy := b.sc.sy
	readingOrder(b.rs, 0.5*sy)
	for i := range b.rs {
		r := &b.rs[i]
		// Unlabelled floor bigger than a store between walls is floor no
		// room was drawn round: a corridor, a bay of desks.
		unwalled := b.spaced && !r.seeded && r.areaM >= smallRoomArea
		switch {
		case r.name != "":
		case r.areaM >= openPlanArea || unwalled:
			r.name = openPlanName
			b.openPlan++
		default:
			b.unnamed++
			r.name = fmt.Sprintf("Room %d", b.unnamed)
		}
		if !r.labeled {
			r.label = qp(pole(r.poly, 1))
		}
		// A meeting or training room this big is still behind walls.
		zone, _ := classify(r.name)
		if !r.explicit {
			r.open = (r.areaM >= openPlanArea || unwalled) && zone != "meet"
		}
	}
	return nil
}

// faceRooms makes the outer component's bounded faces, less voids and
// façade strips, into rooms.
func (b *builder) faceRooms(voids map[int]bool) {
	g, sx, sy := b.g, b.sc.sx, b.sc.sy
	comp := g.components(b.alive)
	main := comp.find(b.fs[b.outer].verts[0])
	side := faceSides(b.fs, 2*len(g.edges))
	strip := b.facadeStrips(side, voids)
	for i, f := range b.fs {
		if f.area <= 0 || voids[i] {
			continue
		}
		r, ok := faceRoom(g.poly(f.verts), sx, sy)
		switch {
		case ok && strip[i]:
			b.strips++
		case !ok, strip[i]:
		case comp.find(f.verts[0]) != main:
			// A group of walls standing apart from the building.
			b.islands++
		default:
			b.rs = append(b.rs, r)
		}
	}
	// In a planar graph the rooms tile the outer face. Should near misses
	// leave crossings unsplit, faces could overlap; keep the largest rooms
	// that fit rather than publish overlapping ones.
	limit := math.Abs(b.fs[b.outer].area) / float64(sx*sy) * 1.01
	total := 0.0
	for _, r := range b.rs {
		total += r.areaM
	}
	if total > limit {
		slices.SortStableFunc(b.rs, func(x, y room) int { return cmpFloat(y.areaM, x.areaM) })
		total = 0
		kept := b.rs[:0]
		for _, r := range b.rs {
			if total+r.areaM <= limit {
				total += r.areaM
				kept = append(kept, r)
			} else {
				b.overlapRooms++
			}
		}
		b.rs = kept
	}
}

// walls drops stubs, orders and places openings on the output walls, and
// finds the outline.
func (b *builder) walls() error {
	sx, sy := b.sc.sx, b.sc.sy
	kept := b.chains[:0]
	for _, c := range b.chains {
		if c.lengthM(sx, sy) >= minWallM {
			kept = append(kept, c)
		} else {
			b.stub++
		}
	}
	b.chains = kept
	if len(b.chains) == 0 {
		return ErrNoWalls
	}
	lo, hi := b.chains[0].from, b.chains[0].from
	for _, c := range b.chains {
		for _, p := range [2]Point{c.from, c.to} {
			lo = Point{X: math.Min(lo.X, p.X), Y: math.Min(lo.Y, p.Y)}
			hi = Point{X: math.Max(hi.X, p.X), Y: math.Max(hi.Y, p.Y)}
		}
	}
	if (hi.X-lo.X)/sx < minSpanM || (hi.Y-lo.Y)/sy < minSpanM {
		return ErrFlat
	}
	if !slices.ContainsFunc(b.chains, func(c chain) bool { return c.kind == exterior }) {
		// Every exterior wall was a stub: the drawing is too small to tell,
		// and plan.Check needs one.
		longest := 0
		for i, c := range b.chains {
			if c.length > b.chains[longest].length {
				longest = i
			}
		}
		b.chains[longest].kind = exterior
	}
	slices.SortFunc(b.chains, cmpChain)
	b.pl = placeOpenings(b.chains, b.ops, sx, sy, func(p Point) (float64, bool) {
		for _, r := range b.rs {
			if inside(p, r.poly) {
				return r.areaM, true
			}
		}
		return 0, false
	})
	if b.outer >= 0 {
		poly := simplify(b.g.poly(b.fs[b.outer].verts), 0.25)
		if len(poly) >= 3 && !axisRect(poly) {
			b.outline = poly
		}
	}
	return nil
}

func axisRect(poly []Point) bool {
	if len(poly) != 4 {
		return false
	}
	for i := range poly {
		d := sub(poly[(i+1)%4], poly[i])
		if d.X != 0 && d.Y != 0 {
			return false
		}
	}
	return true
}

// maxLoose is the longest wall, metres, taken for loose linework when it
// stands outside the building's outline: a façade fin, a brise-soleil
// bracket, a canopy's post. A longer one is a wall of the building (a
// parapet, a terrace's edge) and says the outline is what's wrong.
const maxLoose = 3.0

// outsideWalls are the walls of a trace that fall outside its outline by
// the site validator's test (test/invariants.mjs): an end not inside, nor
// inside once moved 0.2 m along x, y or both (0.15 m for room and void
// corners and furniture centres). The test is run on the plan Convert
// makes, the very numbers the validator will see: the moved points are
// nine probes, not a disc, so a slightly different slack or rounding can
// disagree with it on a thin outline. ok is false when anything else is
// outside too, or a wall outside is part of the building: longer than
// maxLoose, exterior (on the very face the outline was drawn round), or
// with a door or window in it (a room left open). Then the outline is
// what's wrong, and leaving walls out would not mend it.
func outsideWalls(t *plan.Trace) (walls []int, ok bool) {
	p, err := plan.Convert(t, plan.Meta{})
	if err != nil {
		return nil, false
	}
	poly := make([]Point, len(p.Outline))
	for i, o := range p.Outline {
		poly[i] = Point{X: o[0], Y: o[1]}
	}
	in := func(o [2]float64, slack float64) bool {
		for _, i := range [3]float64{0, -1, 1} {
			for _, j := range [3]float64{0, -1, 1} {
				if inside(Point{X: o[0] + float64(i*slack), Y: o[1] + float64(j*slack)}, poly) {
					return true
				}
			}
		}
		return false
	}
	out := func(o [2]float64) bool { return !in(o, 0.15) }
	for _, r := range p.Rooms {
		if slices.ContainsFunc(r.Polygon, out) {
			return nil, false
		}
	}
	for _, v := range p.Voids {
		if slices.ContainsFunc(v, out) {
			return nil, false
		}
	}
	// Furniture outside stands on floor the outline leaves out: a terrace,
	// a room left open.
	for _, it := range p.Furniture {
		x, _ := it["x"].(float64)
		y, _ := it["y"].(float64)
		if out([2]float64{x, y}) {
			return nil, false
		}
	}
	for i, w := range p.Walls {
		if in(w.From, 0.2) && in(w.To, 0.2) {
			continue
		}
		if len(w.Openings) > 0 || w.Kind == exterior ||
			math.Hypot(w.To[0]-w.From[0], w.To[1]-w.From[1]) > maxLoose {
			return nil, false
		}
		walls = append(walls, i)
	}
	return walls, true
}

// keepOutline makes the trace fit its outline by leaving out the loose
// walls outside it, and reports whether the outline can stay.
//
// Leaving walls out moves the plan's origin to the bounds of those left,
// and Convert rounds every coordinate afresh to the millimetre: on a sliver
// of an outline that can move a wall that was inside by a probe's width
// out. So the test is run again on what is left, until nothing more
// leaves; a few rounds settle it, and an outline that won't is dropped.
func (b *builder) keepOutline(t *plan.Trace) bool {
	walls, left := t.Walls, 0
	for range 4 {
		try := *t
		try.Walls = walls
		out, ok := outsideWalls(&try)
		if !ok {
			return false
		}
		if len(out) == 0 {
			if !slices.ContainsFunc(walls, func(w plan.Wall) bool { return w.Kind == exterior }) {
				return false
			}
			t.Walls = walls
			b.outsideWalls = left
			return true
		}
		walls = slices.Clone(walls)
		for _, i := range slices.Backward(out) {
			walls = slices.Delete(walls, i, i+1)
		}
		left += len(out)
	}
	return false
}

// entrance is the middle of the widest exterior door (the one nearest the
// building's middle among equals), else of the longest exterior wall.
func (b *builder) entrance() Point {
	sx, sy := b.sc.sx, b.sc.sy
	var centre Point
	if b.outer >= 0 {
		centre = centroid(b.g.poly(b.fs[b.outer].verts))
	} else {
		centre = centroid(hull(b.g.pts))
	}
	best, found := Point{}, false
	var bestW, bestD float64
	for i := range b.chains {
		c := &b.chains[i]
		if c.kind != exterior {
			continue
		}
		ax := axis(c.from, c.to)
		for _, o := range c.ops {
			if o.kind != Door && o.kind != DoubleDoor {
				continue
			}
			pa, pb := c.at(o.a, ax), c.at(o.b, ax)
			m := qp(mid(pa, pb))
			w, d := mm(dist(pa, pb)*metresPerPx(c.dir(), sx, sy)), dist(m, centre)
			if !found || w > bestW || (w == bestW && d < bestD) {
				best, bestW, bestD, found = m, w, d, true
			}
		}
	}
	if found {
		return best
	}
	longest := -1
	for i, c := range b.chains {
		if c.kind == exterior && (longest < 0 || c.length > b.chains[longest].length) {
			longest = i
		}
	}
	return qp(mid(b.chains[longest].from, b.chains[longest].to))
}

// at is the point on the wall whose dominant-axis coordinate is v.
func (c *chain) at(v float64, ax int) Point {
	span := coord(c.to, ax) - coord(c.from, ax)
	return lerp(c.from, c.to, (v-coord(c.from, ax))/span)
}

func pt(p Point) [2]float64 { return [2]float64{p.X, p.Y} }

func polyOut(poly []Point) [][2]float64 {
	// Start at the top-left corner so the same room reads the same in
	// every trace.
	first := 0
	for i, p := range poly {
		if cmpPoint(p, poly[first]) < 0 {
			first = i
		}
	}
	out := make([][2]float64, 0, len(poly))
	for i := range poly {
		out = append(out, pt(poly[(first+i)%len(poly)]))
	}
	return out
}

func (b *builder) trace() *plan.Trace {
	sx, sy := b.sc.sx, b.sc.sy
	th := kindThickness(b.chains, sx, sy)
	t := &plan.Trace{
		Title:       strings.TrimSpace(b.in.Title),
		Calibration: plan.Calibration{PxPerMetre: [2]float64{sx, sy}, Evidence: b.sc.evidence},
		Thickness: plan.Thickness{
			Exterior: th[exterior],
			Interior: th[interior],
			Glass:    th[glass],
		},
		Entrance:  pt(b.entrance()),
		Walls:     make([]plan.Wall, 0, len(b.chains)),
		Rooms:     make([]plan.Room, 0, len(b.rs)),
		Furniture: slices.Clone(b.in.Furniture),
	}
	if t.Title == "" {
		t.Title = "Floor plan"
	}
	if t.Furniture == nil {
		t.Furniture = []plan.Item{}
	}
	if len(b.outline) > 0 {
		t.Outline = polyOut(b.outline)
	}
	for _, v := range b.voids {
		t.Voids = append(t.Voids, polyOut(v))
	}
	for i := range b.chains {
		c := &b.chains[i]
		w := plan.Wall{ID: fmt.Sprintf("w%d", i+1), Kind: c.kind, From: pt(c.from), To: pt(c.to)}
		def := th[c.kind]
		if m := mm(c.thicknessM(sx, sy)); c.t > 0 && m > 0 && math.Abs(m-def) > 0.2*def {
			w.Thickness = m
		}
		for _, o := range c.ops {
			w.Openings = append(
				w.Openings,
				plan.Opening{A: o.a, B: o.b, Kind: o.kind, Swing: o.swing, Hinge: o.hinge},
			)
		}
		t.Walls = append(t.Walls, w)
	}
	for i, r := range b.rs {
		zone, floor := classify(r.name)
		label := pt(r.label)
		kind := ""
		if r.open {
			kind = plan.KindZone
		}
		t.Rooms = append(t.Rooms, plan.Room{
			ID: fmt.Sprintf(
				"r%d",
				i+1,
			),
			Name:    r.name,
			Number:  r.number,
			Kind:    kind,
			Zone:    zone,
			Floor:   floor,
			Polygon: polyOut(r.poly),
			Label:   &label,
		})
	}
	return t
}

func (b *builder) confidence() float64 {
	scaleScore := [...]float64{1, 0.85, 0.6, 0.45, 0.3, 0.1}[b.sc.source]
	named := 0.0
	if len(b.rs) > 0 {
		named = float64(len(b.rs)-b.unnamed) / float64(len(b.rs))
	}
	closed, all := 0.0, 0.0
	for e := range b.g.edges {
		if b.g.edges[e].virtual {
			continue
		}
		l := b.g.length(e)
		all += l
		if b.alive[e] {
			closed += l
		}
	}
	doors := 1.0
	if b.pl.doors > 0 {
		doors = float64(b.pl.leafed) / float64(b.pl.doors)
	}
	scores := []float64{scaleScore, closed / all, b.inputWeight / b.inputLen, named, doors}
	weights := []float64{0.25, 0.2, 0.15, 0.15, 0.1}
	if c := b.in.Coverage; c >= 0 && finite(c) {
		scores, weights = append(scores, math.Min(c, 1)), append(weights, 0.15)
	}
	sum, wsum := 0.0, 0.0
	for i := range scores {
		sum += float64(scores[i] * weights[i])
		wsum += weights[i]
	}
	conf := sum / wsum
	if len(b.rs) == 0 {
		conf /= 2
	}
	return math.Round(conf*100) / 100
}

func (b *builder) summary(t *plan.Trace) string {
	ops := 0
	for _, w := range t.Walls {
		ops += len(w.Openings)
	}
	var how string
	switch b.sc.source {
	case scaleExact:
		how = "scale from the drawing's units"
	case scaleMeasured:
		how = "scale measured by the front end"
	default:
		how = "scale " + b.sc.evidence
	}
	return fmt.Sprintf("%s, %s, %s; %s",
		count(len(t.Rooms), "room", "rooms"), count(len(t.Walls), "wall", "walls"),
		count(ops, "opening", "openings"), how)
}

func (b *builder) caveats() []string {
	var out []string
	for _, n := range b.in.Notes {
		if n = strings.TrimSpace(n); n != "" {
			out = append(out, n)
		}
	}
	add := func(n int, one, many string) {
		if n > 0 {
			out = append(out, fmt.Sprintf(map[bool]string{true: one, false: many}[n == 1], n))
		}
	}
	add(b.badWalls, "%d wall piece had unusable coordinates and was ignored.",
		"%d wall pieces had unusable coordinates and were ignored.")
	add(b.badOps, "%d opening had unusable coordinates or no width and was ignored.",
		"%d openings had unusable coordinates or no width and were ignored.")
	add(b.oddOps, "%d opening of unknown kind was treated as a plain opening.",
		"%d openings of unknown kind were treated as plain openings.")
	if b.sc.source >= scaleDoors {
		out = append(
			out,
			fmt.Sprintf("The scale is %s; check it against a known dimension.", b.sc.evidence),
		)
	}
	if b.outer < 0 {
		out = append(
			out,
			"The walls don't enclose an area, so the exterior walls were guessed from the convex hull.",
		)
	}
	add(
		b.pl.unplaced,
		"%d opening matched no wall and was left out.",
		"%d openings matched no wall and were left out.",
	)
	add(b.pl.narrow, "%d opening was under 10 cm wide and was left out.",
		"%d openings were under 10 cm wide and were left out.")
	add(b.pl.overlapped, "%d opening overlapped another on the same wall and was left out.",
		"%d openings overlapped another on the same wall and were left out.")
	add(b.pl.guessed, "%d door swing guessed.", "%d door swings guessed.")
	add(
		b.unnamed,
		"%d room had no label and was numbered.",
		"%d rooms had no label and were numbered.",
	)
	add(b.strayLabels, "%d label fell outside every room.", "%d labels fell outside every room.")
	add(len(b.voids), "%d void (an atrium, a light well) was left open, not floored.",
		"%d voids (atriums, light wells) were left open, not floored.")
	add(b.voidMissed, "%d void's label is in no closed area, so the floor round it was kept.",
		"%d voids' labels are in no closed area, so the floor round them was kept.")
	add(b.strips, "%d narrow strip along the façade was taken as part of the wall, not a room.",
		"%d narrow strips along the façade were taken as part of the wall, not rooms.")
	add(b.openPlan, "%d large room with no name was called "+openPlanName+".",
		"%d large rooms with no name were called "+openPlanName+".")
	out = append(out, b.shared...)
	add(b.islands, "%d area enclosed by walls apart from the building was not made a room.",
		"%d areas enclosed by walls apart from the building were not made rooms.")
	add(b.overlapRooms, "%d overlapping room was dropped.", "%d overlapping rooms were dropped.")
	add(b.stub, "%d wall stub under 6 cm was dropped.", "%d wall stubs under 6 cm were dropped.")
	add(b.outsideWalls, "%d wall outside the building's outline was left out.",
		"%d walls outside the building's outline were left out.")
	if b.outlineDropped {
		out = append(
			out,
			"Some walls run outside the building's outline, so the outline was left out.",
		)
	}
	return out
}
