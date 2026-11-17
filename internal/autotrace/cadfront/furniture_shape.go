package cadfront

import (
	"cmp"
	"math"
	"slices"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// shape is what a furniture block draws, measured in metres in the
// block's own axes (x along the insert's x, y along its y), for telling
// a desk from a chair or a cupboard when the block's name says nothing.
type shape struct {
	box     vector.Rect
	segs    [][2]point // straight pieces
	circles []circle   // whole circles
	swings  []arcPart  // quarter arcs a cupboard door's width across
	rects   []vector.Rect
	// straight and curved are ink lengths: lines and polyline edges, and
	// arcs, circles, ellipses, splines and bulges.
	straight, curved float64
	// parts are the entities drawn; strokes the straight pieces and
	// curves among them, so a chair drawn as a few long polylines counts
	// its detail as one drawn in loose lines does.
	parts, strokes int
}

type circle struct {
	c point
	r float64
}

// arcPart is an arc's centre and radius with its middle point, which
// says which way a door drawn by it swings.
type arcPart struct {
	c, mid point
	r      float64
}

// blockFrame maps a block reference's own coordinates to metres: its
// coordinates first (the inverse of m), then each axis scaled by how much
// the insert stretches it, so a block scaled 1000 × reads true.
type blockFrame struct {
	m, inv vector.Affine
	sx, sy float64 // metres per block unit along x and y
}

func (r *reader) frameOf(m vector.Affine) (blockFrame, bool) {
	inv, ok := inverse(m)
	if !ok {
		return blockFrame{}, false
	}
	return blockFrame{
		m: m, inv: inv,
		sx: point{X: m.A, Y: m.B}.Len() * r.unit,
		sy: point{X: m.C, Y: m.D}.Len() * r.unit,
	}, true
}

// toM takes a drawing point into the frame's metres.
func (f blockFrame) toM(p point) point {
	q := f.inv.Apply(p)
	return point{X: q.X * f.sx, Y: q.Y * f.sy}
}

// fromM takes a point in the frame's metres back to the drawing.
func (f blockFrame) fromM(p point) point {
	return f.m.Apply(point{X: p.X / f.sx, Y: p.Y / f.sy})
}

// shapeOf measures the given leaves in a frame. Text is left out: a
// desk's number or a cupboard's label says nothing of its outline.
func (r *reader) shapeOf(f blockFrame, leaves []int) shape {
	var s shape
	for _, li := range leaves {
		l := &r.leaves[li]
		if cad.TextLines(l.e, l.m) != nil {
			continue
		}
		s.parts++
		at := func(p point) point { return f.toM(l.m.Apply(p)) }
		for _, p := range r.points(li) {
			s.box = s.box.Extend(f.toM(p))
		}
		switch e := l.e.(type) {
		case *cad.Line:
			s.addSeg(at(e.A), at(e.B))
		case *cad.Polyline:
			s.addPolyline(e, at)
		case *cad.Circle:
			c, edge := at(e.Center), at(e.Center.Add(point{X: e.Radius}))
			rad := edge.Sub(c).Len()
			s.circles = append(s.circles, circle{c: c, r: rad})
			s.curved += 2 * math.Pi * rad
			s.strokes++
		case *cad.Arc:
			end := ccwEnd(e.Start, e.End)
			c := at(e.Center)
			mid := at(arcPoint(e.Center, e.Radius, (e.Start+end)/2))
			rad := mid.Sub(c).Len()
			s.addArc(c, mid, rad, end-e.Start)
		case *cad.Solid:
			for i := range 4 {
				s.addSeg(at(e.Corners[i]), at(e.Corners[(i+1)%4]))
			}
		case *cad.Hatch:
			// A hatch fills what its outline already draws.
		default:
			s.curved += pathLen(r.points(li), f)
			s.strokes++
		}
	}
	return s
}

func (s *shape) addSeg(a, b point) {
	s.strokes++
	s.segs = append(s.segs, [2]point{a, b})
	s.straight += b.Sub(a).Len()
}

func (s *shape) addArc(c, mid point, rad, sweep float64) {
	s.strokes++
	s.curved += rad * sweep
	if sweep >= swingMin && sweep <= swingMax && rad >= doorMin && rad <= doorMax {
		s.swings = append(s.swings, arcPart{c: c, mid: mid, r: rad})
	}
}

func (s *shape) addPolyline(p *cad.Polyline, at func(point) point) {
	n := len(p.Vertices)
	edges := n - 1
	if p.Closed {
		edges = n
	}
	for i := range max(edges, 0) {
		a, b := p.Vertices[i], p.Vertices[(i+1)%n]
		if a.P == b.P {
			continue
		}
		if math.Abs(a.Bulge) < straightBulge {
			s.addSeg(at(a.P), at(b.P))
			continue
		}
		c, rad, start, sweep := bulgeArc(a.P, b.P, a.Bulge)
		mid := at(arcPoint(c, rad, start+sweep/2))
		cm := at(c)
		s.addArc(cm, mid, mid.Sub(cm).Len(), math.Abs(sweep))
	}
	if rc, ok := rectOutline(p, at); ok {
		s.rects = append(s.rects, rc)
	}
}

// pathLen is the length of a sampled curve, in the frame's metres.
func pathLen(ps []point, f blockFrame) float64 {
	n := 0.0
	for i := 1; i < len(ps); i++ {
		n += f.toM(ps[i]).Sub(f.toM(ps[i-1])).Len()
	}
	return n
}

// rectOutline reports whether a polyline is a rectangle square to the
// frame, closed by its flag or by coming back to its start: a desk top,
// one bay of a bench, a cupboard's carcass.
func rectOutline(p *cad.Polyline, at func(point) point) (vector.Rect, bool) {
	var ps []point
	for _, v := range p.Vertices {
		if math.Abs(v.Bulge) >= straightBulge {
			return vector.Rect{}, false
		}
		q := at(v.P)
		if len(ps) == 0 || q.Sub(ps[len(ps)-1]).Len() > 1e-6 {
			ps = append(ps, q)
		}
	}
	if len(ps) == 5 && ps[4].Sub(ps[0]).Len() < 0.01 {
		ps = ps[:4]
	} else if len(ps) != 4 || !p.Closed {
		return vector.Rect{}, false
	}
	var box vector.Rect
	for _, q := range ps {
		box = box.Extend(q)
	}
	sz := box.Size()
	tol := 0.01 + 0.02*math.Max(sz.X, sz.Y)
	for _, q := range ps {
		onX := math.Abs(q.X-box.Min.X) < tol || math.Abs(q.X-box.Max.X) < tol
		onY := math.Abs(q.Y-box.Min.Y) < tol || math.Abs(q.Y-box.Max.Y) < tol
		if !onX || !onY {
			return vector.Rect{}, false
		}
	}
	return box, sz.X > tol && sz.Y > tol
}

// Door swings in furniture: a cupboard's or a pedestal's doors drawn open.
// A room door (0.7 m and up) on a furniture layer is not one.
const (
	swingMin = 75 * math.Pi / 180
	swingMax = 105 * math.Pi / 180
	doorMin  = 0.2
	doorMax  = 0.65
)

// edgeTol is how far from a box's side ink may lie and still draw it:
// a few centimetres, more for big boxes whose outlines are drawn doubled.
func edgeTol(b vector.Rect) float64 {
	sz := b.Size()
	return math.Max(0.02, 0.03*math.Min(sz.X, sz.Y))
}

// cover is how much of the straight line from a to b, along x (vertical
// false) or y, the shape's straight ink draws: 0 to 1.
func (s *shape) cover(a, b point, tol float64) float64 {
	vertical := math.Abs(b.X-a.X) < math.Abs(b.Y-a.Y)
	along := func(p point) float64 {
		if vertical {
			return p.Y
		}
		return p.X
	}
	across := func(p point) float64 {
		if vertical {
			return p.X
		}
		return p.Y
	}
	lo, hi := math.Min(along(a), along(b)), math.Max(along(a), along(b))
	if hi-lo <= 0 {
		return 0
	}
	var spans [][2]float64
	for _, g := range s.segs {
		if math.Abs(across(g[0])-across(a)) > tol || math.Abs(across(g[1])-across(a)) > tol {
			continue
		}
		x0, x1 := math.Min(along(g[0]), along(g[1])), math.Max(along(g[0]), along(g[1]))
		x0, x1 = math.Max(x0, lo), math.Min(x1, hi)
		if x1 > x0 {
			spans = append(spans, [2]float64{x0, x1})
		}
	}
	slices.SortFunc(spans, func(p, q [2]float64) int { return cmp.Compare(p[0], q[0]) })
	got, end := 0.0, lo
	for _, sp := range spans {
		if sp[1] <= end {
			continue
		}
		got += sp[1] - math.Max(sp[0], end)
		end = sp[1]
	}
	return got / (hi - lo)
}

// sides is how much of each side of b straight ink draws: bottom, right,
// top, left.
func (s *shape) sides(b vector.Rect) [4]float64 {
	tol := edgeTol(b)
	lo, hi := b.Min, b.Max
	return [4]float64{
		s.cover(lo, point{X: hi.X, Y: lo.Y}, tol),
		s.cover(point{X: hi.X, Y: lo.Y}, hi, tol),
		s.cover(point{X: lo.X, Y: hi.Y}, hi, tol),
		s.cover(lo, point{X: lo.X, Y: hi.Y}, tol),
	}
}

// boxy reports whether straight ink outlines b: at least three sides
// drawn for most of their length. Rounded corners and a missing back
// (a desk against a screen) still count; the curves of a chair don't.
func (s *shape) boxy(b vector.Rect) bool {
	n := 0
	for _, c := range s.sides(b) {
		if c >= boxySide {
			n++
		}
	}
	return n >= 3
}

const boxySide = 0.6

// crossed reports whether two long diagonals of opposite slope cross
// inside the box: the drafting mark of a full-height cupboard or locker.
func (s *shape) crossed() bool {
	sz := s.box.Size()
	var up, down []([2]point)
	for _, g := range s.segs {
		d := g[1].Sub(g[0])
		if math.Abs(d.X) < 0.3*sz.X || math.Abs(d.Y) < 0.2*sz.Y {
			continue
		}
		if d.X*d.Y > 0 {
			up = append(up, g)
		} else {
			down = append(down, g)
		}
	}
	for _, a := range up {
		for _, b := range down {
			if crosses(a, b) {
				return true
			}
		}
	}
	return false
}

// crosses reports whether two segments cross or one ends on the other:
// some libraries draw the cross as one diagonal and half of the other.
func crosses(a, b [2]point) bool {
	side := func(p, q, x point) float64 {
		d := q.Sub(p)
		return cross(d, x.Sub(p)) / d.Len() // the distance, signed, so a touch has a tolerance
	}
	const touch = 0.01
	straddles := func(s1, s2 float64) bool {
		return s1*s2 < 0 || math.Abs(s1) < touch || math.Abs(s2) < touch
	}
	return straddles(side(a[0], a[1], b[0]), side(a[0], a[1], b[1])) &&
		straddles(side(b[0], b[1], a[0]), side(b[0], b[1], a[1]))
}

// roundness says whether the shape is one round thing (a table top, a
// stool's seat): a circle nearly as big as the box, or an outline that is
// almost all curve; either way simply drawn, not much more ink than one
// loop round the box in a few strokes. A big circle may hold more: a
// meeting table's power modules. It returns the diameter.
func (s *shape) roundness() (float64, bool) {
	sz := s.box.Size()
	long, short := math.Max(sz.X, sz.Y), math.Min(sz.X, sz.Y)
	if short <= 0 || long/short > 1.2 {
		return 0, false
	}
	ink := s.straight + s.curved
	simple := ink <= roundInk*math.Pi*(long+short)/2 && s.strokes <= roundStrokes
	for _, c := range s.circles {
		if 2*c.r >= 0.9*long && c.c.Sub(s.box.Center()).Len() < 0.1*long &&
			(simple || 2*c.r >= bigRound) {
			return 2 * c.r, true
		}
	}
	if simple && s.curved >= 0.8*ink {
		return (long + short) / 2, true
	}
	return 0, false
}

// roundInk is how many loops round its box a round piece's ink may make
// (a table's top and its edge, not a chair's seat, back and arms), and
// roundStrokes in how many strokes: an upholstered chair drawn in curves
// alone takes dozens. A circle of bigRound or more is a table whatever
// is drawn on it.
const (
	roundInk     = 2.3
	roundStrokes = 30
	bigRound     = 0.9
)
