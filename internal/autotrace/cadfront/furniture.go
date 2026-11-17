package cadfront

import (
	"math"
	"strings"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/plan"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// findFurniture places furniture blocks: by the type their name gives
// where it gives one, otherwise by what they draw (see guess), split into
// one desk per workplace where a block is a bench of them. It needs real
// sizes, so it does nothing when the drawing's units are unknown.
//
// A block holding other furniture blocks (a workstation of a desk and a
// chair, a meeting set) is placed as its pieces; what it draws itself, a
// meeting set's table, is read as one more piece.
func (r *reader) findFurniture() {
	r.findDesks()
	var tops []*node
	for _, n := range r.nodes {
		if n.role == roleFurniture &&
			(n.top == n || n.top == nil && n.holdsFurniture && len(n.leaves) > 0) {
			tops = append(tops, n)
		}
	}
	if len(tops) == 0 {
		return
	}
	if !r.known {
		r.note(
			"%d furniture blocks were left out: without the drawing's units their sizes are unknown.",
			len(tops),
		)
		return
	}
	nums := r.deskNumbers()
	var tally furnTally
	first := len(r.in.Furniture)
	for _, n := range tops {
		group := n.top != n
		leaves := r.subtree(n, nil)
		if group {
			leaves = n.leaves
		}
		items, v := r.furnish(n, leaves, nums)
		if group && v != placedByName && v != placedByShape {
			// Its pieces are placed on their own; a frame or a label
			// drawn round them is nothing to report.
			continue
		}
		tally[v]++
		if len(items) == 0 {
			continue
		}
		for _, li := range leaves {
			r.leaves[li].used = true
		}
		if !group {
			r.use(n)
		}
		r.in.Furniture = append(r.in.Furniture, items...)
	}
	r.faceDesks(r.in.Furniture[first:])
	tally.notes(r)
}

// verdict is what became of a furniture block.
type verdict int

const (
	placedByName verdict = iota
	placedByShape
	unknownBlock // nothing says what it is
	untypedBlock // its name says it is something the plan has no type for
	oddBlock     // the size it is drawn at is no piece of furniture's
	verdicts
)

type furnTally [verdicts]int

func (t *furnTally) notes(r *reader) {
	if n := t[placedByShape]; n > 0 {
		r.note("%d furniture blocks were typed by their shape, "+
			"their names not saying what they are.", n)
	}
	if n := t[unknownBlock]; n > 0 {
		r.note(
			"%d furniture blocks had names that don't say what they are and shapes that don't "+
				"either, and were left out.",
			n,
		)
	}
	if n := t[untypedBlock]; n > 0 {
		r.note(
			"%d furniture blocks were left out as things the plan has no type for "+
				"(lamps, screens, bins, coat stands and the like).",
			n,
		)
	}
	if n := t[oddBlock]; n > 0 {
		r.note("%d furniture blocks were left out for implausible sizes.", n)
	}
}

// furnPiece is one item a block makes, in its frame's metres: a box, which
// way its width runs (along the frame's y when alongY, so a desk in a
// bench split across x lies the right way), and the type.
type furnPiece struct {
	typ    string
	box    vector.Rect
	alongY bool
	round  bool
}

// furnish turns a furniture block (its given leaves) into plan items.
func (r *reader) furnish(n *node, leaves []int, nums []point) ([]plan.Item, verdict) {
	f, ok := r.frameOf(n.m)
	if !ok {
		return nil, oddBlock
	}
	s := r.shapeOf(f, leaves)
	if s.box.Empty() {
		return nil, oddBlock
	}
	var in []point
	for _, p := range nums {
		if q := f.toM(p); inBox(s.box, q, 0.02) {
			in = append(in, q)
		}
	}
	sizes, count := nameNumbers(n.ins.Block)
	v := placedByName
	var ps []furnPiece
	switch n.furn {
	case "":
		if notFurniture(n.ins.Block) {
			return nil, untypedBlock
		}
		v = placedByShape
		ps = r.guess(&s, n.info.sanitary, in, sizes, count)
		if ps == nil {
			return nil, unknownBlock
		}
	case "desk":
		ps = deskCells(&s, in, sizes, count, false)
	case "bench":
		// Bench desking is back to back, or numbered as desks; a bench
		// too shallow for either is a seat, which the plan has no type
		// for.
		sz := s.box.Size()
		if len(in) == 0 && math.Min(sz.X, sz.Y) < 2*minDeskD+0.2 {
			return nil, untypedBlock
		}
		ps = deskCells(&s, in, sizes, count, false)
	default:
		// A table drawn mostly in curves is round even without a circle:
		// a pebble-shaped coffee table, a top drawn as a spline.
		round := len(s.circles) > 0 || s.curved >= 0.5*(s.straight+s.curved)
		ps = []furnPiece{{typ: n.furn, box: s.box, round: round}}
	}
	var out []plan.Item
	for _, p := range ps {
		if it, ok := r.emit(f, p); ok {
			out = append(out, it)
		}
	}
	if len(out) == 0 {
		return nil, oddBlock
	}
	return out, v
}

// sizedName reports whether a name's sizes give both sides of a box, to
// 6%.
func sizedName(sizes []float64, long, short float64) bool {
	near := func(v float64) bool {
		for _, x := range sizes {
			if math.Abs(x-v) <= 0.06*v {
				return true
			}
		}
		return false
	}
	return near(long) && near(short)
}

func area(b vector.Rect) float64 {
	sz := b.Size()
	return math.Max(sz.X, 0) * math.Max(sz.Y, 0)
}

func inBox(b vector.Rect, p point, tol float64) bool {
	return p.X >= b.Min.X-tol && p.X <= b.Max.X+tol && p.Y >= b.Min.Y-tol && p.Y <= b.Max.Y+tol
}

// deskNumbers are where desks are numbered: text on a layer named for
// desks, short and holding a digit ("4C-117", "WS 14"), so a note about
// the desks on the same layer isn't one.
func (r *reader) deskNumbers() []point {
	var out []point
	for i := range r.leaves {
		l := &r.leaves[i]
		if !l.info.desks {
			continue
		}
		lines := cad.TextLines(l.e, l.m)
		if len(lines) == 0 {
			continue
		}
		text := strings.TrimSpace(lines[0].Content)
		if len(text) > 12 || !strings.ContainsAny(text, "0123456789") {
			continue
		}
		out = append(out, l.box.Center())
	}
	return out
}

// Sizes, in metres, that say what an unnamed piece of furniture is. They
// are the ranges office furniture is made in, wide enough for the
// variety and narrow enough that one kind doesn't pass for another.
const (
	// A desk or a table: a workplace is 0.9–2.0 m wide and 0.5–1.0 m
	// deep; a lone table may run to 2.4 m by 1.2 m (and a little, for
	// how it is drawn), and is at least 1 m long, so a 0.9 m square is
	// an armchair's box as often as a table's.
	minDeskW, maxDeskW, maxTableW = 0.9, 2.0, 2.4
	minDeskD, maxDeskD, maxTableD = 0.5, 1.0, 1.25
	minTableW                     = 1.0
	// A workplace in a bench is at least 0.6 m deep: a 1 m table split
	// down its middle is not two desks.
	minCellD = 0.6
	// A meeting table, a bench of desks with nothing to split it by: up
	// to 5 m by 2 m.
	maxBigTable, maxBigTableD = 5.0, 2.0
	// A chair is 0.38–0.76 m across, an armchair up to 1 m.
	minChair, maxChair, maxArmchair = 0.38, 0.76, 1.0
	// A stool's seat or a side table: 0.3–0.55 m. Round, from there to
	// 2 m, it is a table; square, to 0.65 m, a pouffe.
	minStool, maxStool, maxRound, maxPouffe = 0.3, 0.55, 2.0, 0.65
	stoolAspect                             = 1.25
	// A cupboard's carcass, behind its doors.
	minCabinet, maxCabinet = 0.25, 3.0
)

// guess types an unnamed furniture block by what it draws, or returns
// nil. Each rule asks for the evidence a draughtsman's symbol always
// has, and a block that fits none is left out: a wrongly typed piece in
// the plan is worse than a missing one.
func (r *reader) guess(
	s *shape, sanitary bool, in []point, sizes []float64, count int,
) []furnPiece {
	sz := s.box.Size()
	long, short := math.Max(sz.X, sz.Y), math.Min(sz.X, sz.Y)
	if short <= 0 {
		return nil
	}
	one := func(typ string) []furnPiece { return []furnPiece{{typ: typ, box: s.box}} }
	if sanitary {
		return sanitaryGuess(s, long, short)
	}
	// loops is how many times the ink would go round the box: one for an
	// outline, more for a chair's seat, back and arms.
	loops := (s.straight + s.curved) / (2 * (sz.X + sz.Y))
	if d, ok := s.roundness(); ok {
		switch {
		case d < minStool || d > maxRound:
			return nil
		case d < maxStool:
			return one("stool")
		}
		return []furnPiece{{typ: "roundTable", box: s.box, round: true}}
	}
	if len(s.swings) > 0 {
		if ps := cabinet(s); ps != nil {
			return ps
		}
	}
	boxy := s.boxy(s.box)
	if boxy && s.crossed() && short >= minCabinet && long <= maxCabinet {
		return one("storage")
	}
	// Doors drawn open say it is no desk, even when they don't say what
	// it is.
	if (boxy || len(in) > 0) && len(s.swings) == 0 {
		if ps := deskCells(s, in, sizes, count, true); ps != nil {
			return ps
		}
	}
	// A block named for its size ("..._850x850") is a made-to-size top too
	// small for a desk: a table, round or square, never a chair.
	if sizedName(sizes, long, short) && short >= minDeskD && long <= maxTableW {
		round := s.curved >= 0.5*(s.straight+s.curved)
		return []furnPiece{{typ: "table", box: s.box, alongY: sz.Y > sz.X, round: round}}
	}
	if long/short <= chairAspect && short >= minChair && long <= maxArmchair &&
		loops >= chairLoops && s.strokes >= chairStrokes {
		if long > maxChair {
			return one("armchair")
		}
		return one("chair")
	}
	// A stool is an outline or two; a door swing in a box that small is a
	// cupboard drawn too sparely to find its carcass in.
	if long/short <= stoolAspect && short >= minStool && long <= maxPouffe && len(s.swings) == 0 {
		return one("stool")
	}
	return nil
}

// A chair is near square and drawn in detail: seat, back, arms and
// castors make its ink go round its box one and a half times or more, in
// dozens of strokes, where a stool, a pouffe or a base unit is an outline
// or two in a handful.
const (
	chairAspect  = 1.35
	chairLoops   = 1.4
	chairStrokes = 40
)

// sanitaryGuess types a fitting on a sanitary layer: a WC is a long bowl
// (0.5–1.2 m by 0.3–0.6 m), a basin or urinal a shorter one; anything
// smaller is a tap, a dryer or a dispenser, and left out.
func sanitaryGuess(s *shape, long, short float64) []furnPiece {
	if s.curved < 0.15*(s.straight+s.curved) {
		return nil // a bowl is round; a box is a dispenser or a duct
	}
	switch {
	case long >= 0.55 && long <= 1.2 && short >= 0.3 && short <= 0.6 && long/short >= 1.3:
		return []furnPiece{{typ: "toilet", box: s.box}}
	case long >= 0.3 && long <= 0.75 && short >= 0.25:
		return []furnPiece{{typ: "basin", box: s.box}}
	}
	return nil
}

// cabinet is a cupboard drawn with its doors open: the carcass is the
// box behind the doors' hinges, on the side away from their swing. An
// arc's middle lies at 45° to both the closed and the open leaf, so it
// can't say which side of the hinge is the front; of the two cuts, the
// one that leaves more carcass is.
func cabinet(s *shape) []furnPiece {
	b := s.box
	for _, a := range s.swings {
		out := a.mid.Sub(a.c)
		byX, byY := b, b
		if out.X > 0 {
			byX.Max.X = math.Min(b.Max.X, a.c.X)
		} else {
			byX.Min.X = math.Max(b.Min.X, a.c.X)
		}
		if out.Y > 0 {
			byY.Max.Y = math.Min(b.Max.Y, a.c.Y)
		} else {
			byY.Min.Y = math.Max(b.Min.Y, a.c.Y)
		}
		b = byY
		if area(byX) > area(byY) {
			b = byX
		}
	}
	sz := b.Size()
	if math.Min(sz.X, sz.Y) < minCabinet || math.Max(sz.X, sz.Y) > maxCabinet {
		return nil
	}
	// The carcass is drawn: a lounge chair's curved arm is a quarter arc
	// too, with nothing square behind it.
	drawn := 0
	for _, c := range s.sides(b) {
		if c >= boxySide {
			drawn++
		}
	}
	if drawn < 2 {
		return nil
	}
	return []furnPiece{{typ: "cabinet", box: b}}
}

// deskCells splits a desk block into its workplaces: one row, or two
// back to back, of equal desks. Of the layouts whose desks are desk
// sized, the one the evidence best supports wins: desk numbers, one in
// each desk; the count and module the name gives; lines drawn along the
// joins; closed outlines the size of one desk. A layout of several desks
// needs some of that evidence; with none the block is one desk or table,
// if it is the size of one. guessing asks the stricter size of an
// unnamed block, which can't be a desk the size of a room.
func deskCells(s *shape, in []point, sizes []float64, count int, guessing bool) []furnPiece {
	sz := s.box.Size()
	type layout struct {
		depthY     bool // the rows stack along y, so desks are wide along x
		rows, cols int
		score      float64
	}
	best := layout{score: -1}
	perim := 2 * (sz.X + sz.Y)
	loops := (s.straight + s.curved) / perim
	if guessing && float64(s.strokes) > tableStrokes*perim {
		return nil
	}
	for _, depthY := range []bool{true, false} {
		lw, ld := sz.X, sz.Y
		if !depthY {
			lw, ld = sz.Y, sz.X
		}
		for rows := 1; rows <= 2; rows++ {
			for cols := 1; cols <= maxCols; cols++ {
				w, d := lw/float64(cols), ld/float64(rows)
				single := rows*cols == 1
				if !deskSized(w, d, single, guessing && loops > deskLoops) {
					continue
				}
				sc := layoutScore(s, depthY, rows, cols, in, sizes, count)
				if !single && sc < minEvidence {
					continue
				}
				// Ties go to fewer desks: splitting needs a reason.
				if sc > best.score || sc == best.score && rows*cols < best.rows*best.cols {
					best = layout{depthY: depthY, rows: rows, cols: cols, score: sc}
				}
			}
		}
	}
	if best.score < 0 {
		return oneTable(s.box, guessing, loops)
	}
	out := make([]furnPiece, 0, best.rows*best.cols)
	for row := range best.rows {
		for col := range best.cols {
			out = append(out, furnPiece{
				typ: "desk", box: cell(s.box, best.depthY, best.rows, best.cols, row, col),
				alongY: !best.depthY,
			})
		}
	}
	return out
}

// deskSized reports whether a layout's desk is the size of one: a lone
// desk or table (not too busy to be one, when guessing) or one desk of
// several.
func deskSized(w, d float64, single, busy bool) bool {
	if single {
		return !busy && w >= d && w >= minTableW && w <= maxTableW &&
			d >= minDeskD && d <= maxTableD
	}
	return w >= minDeskW && w <= maxDeskW && d >= minCellD && d <= maxDeskD
}

// oneTable is a block no layout fits as one table, its width along its
// longer side; guessing, only one of a meeting table's size and simply
// drawn.
func oneTable(b vector.Rect, guessing bool, loops float64) []furnPiece {
	sz := b.Size()
	long, short := math.Max(sz.X, sz.Y), math.Min(sz.X, sz.Y)
	if guessing && (long < minTableW || long > maxBigTable || short < minDeskD ||
		short > maxBigTableD || loops > bigTableLoops) {
		return nil
	}
	return []furnPiece{{typ: "desk", box: b, alongY: sz.Y > sz.X}}
}

// Desk layouts: at most twelve workplaces a row, and a split needs a desk
// number in each desk, or the joins drawn and the name's count or module
// besides: a booth's table and benches are drawn with a line down the
// middle too. An unnamed desk or table must be simply drawn: a pod's or
// booth's walls, seats and screens, or a sofa's stitching, go round its
// box many times.
const (
	maxCols       = 12
	minEvidence   = 3.0
	bigTableLoops = 3.0
	deskLoops     = 4.0
	// tableStrokes is the most strokes a metre of an unnamed desk's
	// outline may carry: a desk with its frame and cable tray has a few
	// dozen, an upholstered bench or sofa hundreds.
	tableStrokes = 60
)

// cell is one desk of a layout, in the block's metres.
func cell(b vector.Rect, depthY bool, rows, cols, row, col int) vector.Rect {
	sz := b.Size()
	if depthY {
		w, d := sz.X/float64(cols), sz.Y/float64(rows)
		lo := point{X: b.Min.X + float64(col)*w, Y: b.Min.Y + float64(row)*d}
		return vector.Rect{}.Extend(lo).Extend(lo.Add(point{X: w, Y: d}))
	}
	w, d := sz.Y/float64(cols), sz.X/float64(rows)
	lo := point{X: b.Min.X + float64(row)*d, Y: b.Min.Y + float64(col)*w}
	return vector.Rect{}.Extend(lo).Extend(lo.Add(point{X: d, Y: w}))
}

// layoutScore weighs the evidence for a layout of desks.
func layoutScore(s *shape, depthY bool, rows, cols int, in []point, sizes []float64,
	count int,
) float64 {
	n := rows * cols
	sc := 0.0
	if k := len(in); k > 0 {
		if k == n {
			sc += 4
		}
		seen := map[int]bool{}
		distinct := true
		for _, p := range in {
			c := whichCell(s.box, depthY, rows, cols, p)
			distinct = distinct && !seen[c]
			seen[c] = true
		}
		if distinct && k == n {
			sc += 2
		}
	}
	if count > 0 && count == n && n > 1 {
		sc += 2
	}
	sz := s.box.Size()
	w, d := sz.X/float64(cols), sz.Y/float64(rows)
	if !depthY {
		w, d = sz.Y/float64(cols), sz.X/float64(rows)
	}
	for _, v := range sizes {
		if math.Abs(v-w) <= 0.06*w {
			sc++
			break
		}
	}
	for _, v := range sizes {
		if math.Abs(v-d) <= 0.06*d {
			sc++
			break
		}
	}
	if n > 1 {
		sc += 2 * joins(s, depthY, rows, cols)
		matched := 0
		for _, rc := range s.rects {
			q := rc.Size()
			if !depthY {
				q = point{X: q.Y, Y: q.X}
			}
			if math.Abs(q.X-w) <= 0.1*w && math.Abs(q.Y-d) <= 0.1*d {
				matched++
			}
		}
		if matched >= n {
			sc += 2
		}
	}
	return sc
}

// joins is how well lines are drawn along a layout's joins between desks,
// 0 to 1: the mean share of each join drawn, counting only joins drawn
// for at least half their length.
func joins(s *shape, depthY bool, rows, cols int) float64 {
	b := s.box
	tol := edgeTol(b)
	var got []float64
	line := func(a, c point) { got = append(got, s.cover(a, c, tol)) }
	sz := b.Size()
	if depthY {
		for i := 1; i < cols; i++ {
			x := b.Min.X + sz.X*float64(i)/float64(cols)
			line(point{X: x, Y: b.Min.Y}, point{X: x, Y: b.Max.Y})
		}
		if rows == 2 {
			y := b.Min.Y + sz.Y/2
			line(point{X: b.Min.X, Y: y}, point{X: b.Max.X, Y: y})
		}
	} else {
		for i := 1; i < cols; i++ {
			y := b.Min.Y + sz.Y*float64(i)/float64(cols)
			line(point{X: b.Min.X, Y: y}, point{X: b.Max.X, Y: y})
		}
		if rows == 2 {
			x := b.Min.X + sz.X/2
			line(point{X: x, Y: b.Min.Y}, point{X: x, Y: b.Max.Y})
		}
	}
	if len(got) == 0 {
		return 0
	}
	sum := 0.0
	for _, g := range got {
		if g >= 0.5 {
			sum += g
		}
	}
	return sum / float64(len(got))
}

// whichCell numbers the desk of a layout a point falls in.
func whichCell(b vector.Rect, depthY bool, rows, cols int, p point) int {
	sz := b.Size()
	u, v := (p.X-b.Min.X)/sz.X, (p.Y-b.Min.Y)/sz.Y
	if !depthY {
		u, v = v, u
	}
	col := min(max(int(u*float64(cols)), 0), cols-1)
	row := min(max(int(v*float64(rows)), 0), rows-1)
	return row*cols + col
}

// emit makes a plan item of a piece: its width along the frame's x (or y,
// alongY) and its depth across, its turn from the insert's, and a seat
// or fitting facing the block's -y, the way symbol libraries draw chairs,
// toilets and basins (back to the wall at the top).
func (r *reader) emit(f blockFrame, p furnPiece) (plan.Item, bool) {
	sz := p.box.Size()
	w, d := round3(sz.X), round3(sz.Y)
	xAxis, yAxis := point{X: f.m.A, Y: f.m.B}, point{X: f.m.C, Y: f.m.D}
	along := xAxis
	if p.alongY {
		w, d, along = d, w, yAxis
	}
	if w < minItem || d < minItem || w > maxItem || d > maxItem {
		return plan.Item{}, false
	}
	at := r.toPx(f.fromM(p.box.Center()))
	it := plan.Item{Type: p.typ, X: at.X, Y: at.Y}
	if it.Type == "table" {
		it.Type = "desk"
		if p.round && math.Abs(w-d) <= 0.15*math.Max(w, d) {
			it.Type = "roundTable"
		}
	}
	spec := plan.Types[it.Type]
	if spec.Oriented == "" {
		// Without an angle field the item is laid out along the image's
		// axes, so a piece turned nearer upright than flat swaps sides.
		if a := imageAngle(r.o.View, along); math.Abs(math.Sin(a*math.Pi/180)) > math.Sqrt2/2 {
			w, d = d, w
		}
	}
	for _, field := range spec.Required {
		switch field {
		case "w":
			it.W = ptr(w)
		case "d":
			it.D = ptr(d)
		case "r":
			it.R = ptr(round3((w + d) / 4))
		}
	}
	switch it.Type {
	case "chair":
		it.Size = ptr(round3((w + d) / 2))
	case "plant":
		it.R = ptr(round3((w + d) / 4))
	}
	switch spec.Oriented {
	case "rot":
		it.Rot = ptr(imageAngle(r.o.View, along))
	case "facing":
		it.Facing = ptr(imageAngle(r.o.View, yAxis.Mul(-1)))
	}
	return it, true
}

// faceDesks turns a chair drawn at a desk to face it. Symbol libraries
// mostly draw a chair facing -y, but not all, and a chair seen from the
// wrong side of its desk is the first thing a viewer notices. A chair
// whose drawn facing is already within 60° of its desk keeps it, so a
// chair swung a little aside stays so.
func (r *reader) faceDesks(items []plan.Item) {
	pxm := r.px / r.unit // image pixels per metre
	var desks []plan.Item
	for _, it := range items {
		if it.Type == "desk" && it.W != nil && it.D != nil && it.Rot != nil {
			desks = append(desks, it)
		}
	}
	for i := range items {
		c := &items[i]
		if c.Type != "chair" || c.Facing == nil {
			continue
		}
		best, want := math.Inf(1), 0.0
		for _, dk := range desks {
			s, co := math.Sincos(*dk.Rot * math.Pi / 180)
			rel := point{X: c.X - dk.X, Y: c.Y - dk.Y}.Mul(1 / pxm)
			lx, ly := rel.X*co+rel.Y*s, -rel.X*s+rel.Y*co
			gap := math.Abs(ly) - *dk.D/2
			if math.Abs(lx) > *dk.W/2+0.1 || gap < -chairTuck || gap > chairReach || gap >= best {
				continue
			}
			best, want = gap, *dk.Rot+90
			if ly > 0 {
				want = *dk.Rot + 270
			}
		}
		if math.IsInf(best, 1) {
			continue
		}
		want = math.Mod(want, 360)
		off := math.Abs(math.Mod(*c.Facing-want+540, 360) - 180)
		if off > 60 {
			c.Facing = ptr(math.Round(want*100) / 100)
		}
	}
}

// A chair at a desk sits within 0.8 m of its edge, or tucked up to 0.25 m
// under it.
const chairReach, chairTuck = 0.8, 0.25
