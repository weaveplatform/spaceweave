package cadfront

import (
	"strings"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/floorgraph"
)

// Voids. An atrium or light well is a hole through the floor, drawn as a
// slab edge or a balustrade with its name written inside, and the edge
// rarely sits on a wall layer: here it was on the stairs layer, elsewhere
// a slab-edge, handrail or plain line layer. So the evidence handed over
// is the point inside each void, from its label or a void layer's outline,
// and every line that might be its edge; floorgraph takes the area round
// the point that walls and those lines close, small gaps sealed.

var (
	// voidLayerWords name a layer whose closed outlines are voids.
	voidLayerWords = []string{"VOID*", "ATRIUM*", "LIGHTWELL*", "LUFTRAUM*"}
	// annotationWords are layers whose lines bound nothing: dimensions,
	// grids, help lines, title blocks and Uniclass's Zz annotation codes.
	annotationWords = []string{
		"DIM*", "GRID*", "AXIS", "AXES", "TEXT*", "TXT", "ANNO*", "NOTE*", "TAG*", "HELP*",
		"TITLE*", "TTLB", "BORDER*", "VIEWPORT*", "VPORT*", "DEFPOINTS", "HATCH*", "PATT*",
		"REV*", "ZZ",
	}
)

const (
	// minEdge is the shortest line, metres, offered as a void's edge: a
	// shorter one is a fitting's detail, and there are thousands.
	minEdge = 0.1
	// minEdgeArc is the smallest radius, metres, of an arc offered as an
	// edge: a curved balustrade, not a door swing or a basin.
	minEdgeArc = 1.0
	// minVoid is the smallest void outline taken from a void layer, m².
	minVoid = 1.0
)

// findVoids reads the voids' labels and outlines, and the lines that may
// be their edges.
func (r *reader) findVoids() {
	for i := range r.leaves {
		l := &r.leaves[i]
		if r.consumed(l) || l.owner != nil {
			continue
		}
		if lines := cad.TextLines(l.e, l.m); len(lines) > 0 {
			text := strings.Join(strings.Fields(lines[0].Content), " ")
			if voidName.MatchString(text) && textLayer(l.layer) != textNever {
				r.in.Voids = append(r.in.Voids, r.toPx(lines[0].Bounds().Center()))
			}
			continue
		}
		if p, ok := l.e.(*cad.Polyline); ok && matches(tokens(l.layer), voidLayerWords) {
			r.voidOutline(l, p)
		}
	}
	if len(r.in.Voids) == 0 {
		return
	}
	for i := range r.leaves {
		l := &r.leaves[i]
		if !r.edgeCandidate(l) {
			continue
		}
		for _, s := range r.edgeSegments(l) {
			a, b := l.m.Apply(s[0]), l.m.Apply(s[1])
			if a.Sub(b).Len() >= r.metres(minEdge) {
				r.in.Edges = append(r.in.Edges, floorgraph.Segment{A: r.toPx(a), B: r.toPx(b)})
			}
		}
	}
}

// voidOutline takes a closed polyline on a void layer as a void, by a
// point inside it; its lines are offered as edges with the rest.
func (r *reader) voidOutline(l *leaf, p *cad.Polyline) {
	if !p.Closed || len(p.Vertices) < 3 {
		return
	}
	var ring []point
	for _, q := range polylineSamples(p) {
		ring = append(ring, l.m.Apply(q))
	}
	if ringArea(ring) < r.metres(1)*r.metres(1)*minVoid {
		return
	}
	var c point
	for _, q := range ring {
		c = c.Add(q)
	}
	c = c.Mul(1 / float64(len(ring)))
	// The mean of the corners is inside anything convex; an outline it
	// misses (an L-shaped void) is left to its label.
	if inside([][]point{ring}, c) {
		r.in.Voids = append(r.in.Voids, r.toPx(c))
	}
}

// edgeCandidate says a leaf's lines may be a void's edge: loose linework
// that isn't a wall (walls are handed over already), a door, a window,
// furniture or annotation.
func (r *reader) edgeCandidate(l *leaf) bool {
	if r.consumed(l) || l.owner != nil {
		return false
	}
	switch l.role {
	case roleNone, roleNotWall:
	default:
		return false
	}
	t := tokens(l.layer)
	return matches(t, voidLayerWords) || !matches(t, annotationWords)
}

// edgeSegments are a leaf's straight pieces in its own coordinates: lines,
// polyline segments (arcs in them sampled) and large arcs.
func (r *reader) edgeSegments(l *leaf) [][2]point {
	chain := func(ps []point, closed bool) [][2]point {
		var out [][2]point
		for i := 1; i < len(ps); i++ {
			out = append(out, [2]point{ps[i-1], ps[i]})
		}
		if closed && len(ps) > 2 {
			out = append(out, [2]point{ps[len(ps)-1], ps[0]})
		}
		return out
	}
	switch e := l.e.(type) {
	case *cad.Line:
		return [][2]point{{e.A, e.B}}
	case *cad.Polyline:
		return chain(polylineSamples(e), e.Closed)
	case *cad.Arc:
		if e.Radius*l.m.Factor() >= r.metres(minEdgeArc) {
			return chain(samples(e), false)
		}
	}
	return nil
}
