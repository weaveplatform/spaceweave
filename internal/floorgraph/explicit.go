package floorgraph

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/peterstace/simplefeatures/geom"

	"github.com/weaveplatform/spaceweave/internal/plan"
)

// packageSpaces overlays the registered outlines before the inferred floor.
// Differences, rather than dropping an overlapping inferred room, preserve
// the part of a partially documented floor that still needs inference.
func (b *builder) packageSpaces() error {
	if len(b.in.Spaces) == 0 {
		return nil
	}
	bounds, err := b.spaceBounds()
	if err != nil {
		return err
	}
	walls, _ := b.barriers()
	inputs := slices.Clone(b.in.Spaces)
	slices.SortFunc(inputs, func(a, c Space) int {
		area := func(sp Space) float64 {
			var sum float64
			for i, p := range sp.Polygon {
				q := sp.Polygon[(i+1)%len(sp.Polygon)]
				sum += p[0]*q[1] - q[0]*p[1]
			}
			return math.Abs(sum)
		}
		if n := cmpFloat(area(a), area(c)); n != 0 {
			return n
		}
		if n := strings.Compare(a.Number, c.Number); n != 0 {
			return n
		}
		if n := strings.Compare(a.Name, c.Name); n != 0 {
			return n
		}
		return slices.CompareFunc(a.Polygon, c.Polygon, func(p, q [2]float64) int {
			return cmpPoint(Point{X: p[0], Y: p[1]}, Point{X: q[0], Y: q[1]})
		})
	})
	remaining, reserved := bounds.floor, bounds.voids
	var explicit []room
	for _, sp := range inputs {
		ring, valid := spaceRing(sp.Polygon)
		if !valid {
			b.shared = append(
				b.shared,
				fmt.Sprintf("Space %q has an invalid outline and was not used.", sp.Name),
			)
			continue
		}
		if crosses(ring) {
			b.shared = append(
				b.shared,
				fmt.Sprintf(
					"Space %q has crossing edges; its outline was repaired before clipping.",
					sp.Number,
				),
			)
		}
		if b.in.SpaceSnap != SpaceSnapNone {
			ws := walls
			tol := snapReach * math.Min(b.sc.sx, b.sc.sy)
			if b.in.SpaceSnap == SpaceSnap300mm {
				ws = slices.Clone(walls)
				for i := range ws {
					ws[i].t = 0
				}
				tol = 0.3 * math.Min(b.sc.sx, b.sc.sy)
			}
			ring = snapRing(ring, ws, tol, 0, 0.3*math.Min(b.sc.sx, b.sc.sy), 0)
		}
		g, e := ringGeometry(ring)
		if e != nil {
			return fmt.Errorf("registered space: %w", e)
		}
		before := g.Area()
		if !bounds.image.IsEmpty() {
			g, e = geom.Intersection(g, bounds.image)
			if e != nil {
				return fmt.Errorf("bound registered space: %w", e)
			}
		}
		g, e = geom.Difference(g, reserved)
		if e != nil {
			return fmt.Errorf("clip registered space: %w", e)
		}
		g = polygonal(g)
		changed := (before - g.Area()) / (b.sc.sx * b.sc.sy)
		if changed > math.Max(0.25, 0.005*before/(b.sc.sx*b.sc.sy)) {
			b.shared = append(
				b.shared,
				fmt.Sprintf(
					"Space %q lost %.2f m² where it overlapped another space, a void or the image boundary; review its source outline.",
					sp.Number,
					changed,
				),
			)
		}
		r := room{
			name:     strings.TrimSpace(sp.Name),
			number:   sp.Number,
			explicit: true,
			seeded:   true,
			open:     sp.Kind == plan.KindZone,
		}
		if r.name == "" {
			r.name = strings.TrimSpace(sp.Number)
		}
		explicit = append(explicit, b.geometryRooms(g, r)...)
		reserved, e = geom.Union(reserved, g)
		if e != nil {
			return fmt.Errorf("reserve explicit space: %w", e)
		}
		remaining, e = geom.Difference(remaining, g)
		if e != nil {
			return fmt.Errorf("reserve registered space: %w", e)
		}
	}
	if len(explicit) == 0 {
		return nil
	}
	if err := b.packageFootprint(bounds, reserved); err != nil {
		return err
	}
	var residual []room
	for _, r := range b.rs {
		g, e := ringGeometry(r.poly)
		if e != nil {
			return fmt.Errorf("inferred space: %w", e)
		}
		g, e = geom.Intersection(g, remaining)
		g = polygonal(g)
		if e != nil {
			return fmt.Errorf("clip inferred space: %w", e)
		}
		residual = append(residual, b.geometryRooms(g, r)...)
	}
	explicit = append(explicit, residual...)
	b.rs = explicit
	return nil
}

func spaceRing(poly [][2]float64) ([]Point, bool) {
	if len(poly) < 3 {
		return nil, false
	}
	ring := make([]Point, len(poly))
	for i, p := range poly {
		if !finite(p[0], p[1]) || math.Abs(p[0]) > maxCoord || math.Abs(p[1]) > maxCoord {
			return nil, false
		}
		ring[i] = Point{X: p[0], Y: p[1]}
	}
	return ring, true
}

func (b *builder) packageFootprint(bounds spaceBounds, reserved geom.Geometry) error {
	g, err := geom.Union(bounds.floor, reserved)
	if err != nil {
		return fmt.Errorf("assemble slab footprint: %w", err)
	}
	// Buffer in metres, so an anisotropic image does not distort the wall
	// allowance. This is the existing half-exterior-thickness slab offset,
	// applied robustly to the supported floor instead of a bounding box.
	g = g.TransformXY(func(p geom.XY) geom.XY {
		return geom.XY{X: p.X / b.sc.sx, Y: p.Y / b.sc.sy}
	})
	thickness := kindThickness(b.chains, b.sc.sx, b.sc.sy)[exterior]
	g, err = geom.Buffer(g, thickness/2, geom.BufferJoinStyleMitre(5))
	if err != nil {
		return fmt.Errorf("offset slab footprint: %w", err)
	}
	voids := bounds.voids.TransformXY(func(p geom.XY) geom.XY {
		return geom.XY{X: p.X / b.sc.sx, Y: p.Y / b.sc.sy}
	})
	g, err = geom.Difference(g, voids)
	if err != nil {
		return fmt.Errorf("cut slab voids: %w", err)
	}
	for _, part := range g.Dump() {
		p, ok := part.AsPolygon()
		if !ok || p.IsEmpty() {
			continue
		}
		var polygon [][][2]float64
		for _, seq := range p.ForceCCW().Coordinates() {
			var ring [][2]float64
			for i := 0; i < seq.Length()-1; i++ {
				xy := seq.GetXY(i)
				ring = append(ring, pt(Point{X: xy.X * b.sc.sx, Y: xy.Y * b.sc.sy}))
			}
			polygon = append(polygon, ring)
		}
		b.footprint = append(b.footprint, polygon)
	}
	return nil
}

type spaceBounds struct {
	floor, voids, image geom.Geometry
}

func (b *builder) spaceBounds() (spaceBounds, error) {
	var bounds spaceBounds
	var err error
	if b.outer >= 0 {
		bounds.floor, err = ringGeometry(b.g.poly(b.fs[b.outer].verts))
		if err != nil {
			return bounds, fmt.Errorf("space outline: %w", err)
		}
	}
	for _, v := range b.voids {
		hole, e := ringGeometry(v)
		if e != nil {
			return bounds, fmt.Errorf("space void: %w", e)
		}
		bounds.floor, err = geom.Difference(bounds.floor, hole)
		if err != nil {
			return bounds, fmt.Errorf("subtract space void: %w", err)
		}
		bounds.voids, err = geom.Union(bounds.voids, hole)
		if err != nil {
			return bounds, fmt.Errorf("reserve space void: %w", err)
		}
	}
	// A pruned or unclosed wall bay can lie outside the inferred outer
	// face. Registered spaces supply floor evidence there; the image, not
	// that incomplete face, bounds their coordinates.
	if b.in.Size[0] > 0 && b.in.Size[1] > 0 {
		w, h := float64(b.in.Size[0]), float64(b.in.Size[1])
		bounds.image, err = ringGeometry([]Point{{}, {X: w}, {X: w, Y: h}, {Y: h}})
		if err != nil {
			return bounds, fmt.Errorf("space image bounds: %w", err)
		}
	}
	return bounds, nil
}

func (b *builder) geometryRooms(g geom.Geometry, template room) []room {
	var out []room
	for _, part := range g.Dump() {
		p, ok := part.AsPolygon()
		if !ok || p.IsEmpty() {
			continue
		}
		p = p.ForceCCW()
		seqs := p.Coordinates()
		rings := make([][]Point, len(seqs))
		for i, seq := range seqs {
			for j := 0; j < seq.Length()-1; j++ {
				xy := seq.GetXY(j)
				rings[i] = append(rings[i], Point{X: xy.X, Y: xy.Y})
			}
		}
		r := template
		r.poly = bridge(rings[0], rings[1:])
		var width float64
		r.areaM, width = faceSize(r.poly, b.sc.sx, b.sc.sy)
		// Clipping can turn an otherwise usable inferred room into a thin
		// wall band. Apply the same size rule as initial room inference;
		// supplied spaces have identity evidence and keep their width.
		if r.areaM <= minRoomArea || !r.explicit && width < minRoomWidth {
			continue
		}
		r.c = centroid(r.poly)
		if !r.labeled || !inside(r.label, r.poly) {
			r.label, r.labeled = qp(pole(r.poly, 1)), true
		}
		out = append(out, r)
	}
	return out
}

// The plan format joins holes to the outer ring with a doubled bridge.
// Split at repeated vertices and XOR the simple loops to recover OGC rings.
func ringGeometry(poly []Point) (geom.Geometry, error) {
	var result geom.Geometry
	var path []Point
	positions := map[Point]int{}
	if len(poly) == 0 {
		return result, nil
	}
	poly = nodeRing(poly)
	if len(poly) < 3 {
		return result, nil
	}
	for _, p := range append(slices.Clone(poly), poly[0]) {
		if at, ok := positions[p]; ok {
			loop := path[at:]
			if len(loop) >= 3 && math.Abs(signedArea(loop)) > 0 {
				coords := make([]float64, 0, 2*(len(loop)+1))
				for _, q := range loop {
					coords = append(coords, q.X, q.Y)
				}
				coords = append(coords, p.X, p.Y)
				g := geom.NewPolygon([]geom.LineString{geom.NewLineString(geom.NewSequence(coords, geom.DimXY))}).
					AsGeometry()
				if err := g.Validate(); err != nil {
					repaired, e := geom.Buffer(g, 0)
					if e != nil {
						return result, fmt.Errorf("repair polygon: %w", e)
					}
					g = repaired
				}
				var err error
				result, err = geom.SymmetricDifference(result, g)
				if err != nil {
					return result, fmt.Errorf("assemble rings: %w", err)
				}
			}
			for _, q := range path[at:] {
				delete(positions, q)
			}
			path = path[:at]
		}
		positions[p] = len(path)
		path = append(path, p)
	}
	return result, nil
}

// Simplification may remove a bridge endpoint from the middle of the
// reverse edge. Reinsert collinear vertices before extracting its loops.
func nodeRing(poly []Point) []Point {
	var out []Point
	for i, a := range poly {
		b := poly[(i+1)%len(poly)]
		d := sub(b, a)
		ll := dot(d, d)
		if ll == 0 {
			continue
		}
		points := []Point{a}
		for _, p := range poly {
			t := dot(sub(p, a), d) / ll
			if t > 0 && t < 1 && segDist(p, a, b) < 1e-7 {
				points = append(points, p)
			}
		}
		slices.SortFunc(
			points,
			func(p, q Point) int { return cmpFloat(dot(sub(p, a), d), dot(sub(q, a), d)) },
		)
		out = append(out, slices.Compact(points)...)
	}
	return out
}

// Overlay intersections may also contain shared edges or isolated vertices.
// They carry no floor and must not reach the next polygon-only operation.
func polygonal(g geom.Geometry) geom.Geometry {
	var polys []geom.Polygon
	for _, part := range g.Dump() {
		if p, ok := part.AsPolygon(); ok && !p.IsEmpty() {
			polys = append(polys, p)
		}
	}
	return geom.NewMultiPolygon(polys).AsGeometry()
}
