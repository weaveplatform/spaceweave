package plan

import (
	"errors"
	"fmt"
	"math"
)

// ErrNoCalibration is returned when a trace has no usable scale.
var ErrNoCalibration = errors.New("calibration.pxPerMetre must be two positive numbers")

// ErrNoWalls is returned for a trace with nothing to anchor the origin to.
var ErrNoWalls = errors.New("trace has no walls")

// Plan is plan.json: exactly the exports listed in js/plans/kit.js. The
// upper-case keys are the JS module's export names.
//
//nolint:tagliatelle // keys must match the plan modules' export names
type Plan struct {
	Source       Source           `json:"SOURCE"`
	Width        float64          `json:"WIDTH"`
	Depth        float64          `json:"DEPTH"`
	Outline      [][2]float64     `json:"OUTLINE,omitempty"`
	Entrance     [2]float64       `json:"ENTRANCE"`
	Thickness    Thickness        `json:"THICKNESS"`
	Walls        []MWall          `json:"walls"`
	Rooms        []MRoom          `json:"rooms"`
	FloorPatches []Patch          `json:"floorPatches"`
	Furniture    []map[string]any `json:"furniture"`
}

// Source says where a plan came from and how its pixels map to metres.
// PxPerMetre is a number, or [x, y] when the two axes differ.
type Source struct {
	URL        string     `json:"url"`
	Credit     string     `json:"credit"`
	Title      string     `json:"title"`
	PxPerMetre any        `json:"pxPerMetre"`
	OriginPx   [2]float64 `json:"originPx"`
	SizePx     [2]int     `json:"sizePx"`
}

// MWall is a wall in metres.
type MWall struct {
	ID        string     `json:"id"`
	Kind      string     `json:"kind"`
	From      [2]float64 `json:"from"`
	To        [2]float64 `json:"to"`
	Thickness float64    `json:"thickness,omitempty"`
	Ext       *float64   `json:"ext,omitempty"`
	Openings  []Opening  `json:"openings"`
}

// MRoom is a room in metres.
type MRoom struct {
	ID      string       `json:"id"`
	Name    string       `json:"name"`
	Zone    string       `json:"zone"`
	Floor   string       `json:"floor"`
	Polygon [][2]float64 `json:"polygon"`
	Label   *[2]float64  `json:"label,omitempty"`
}

// Meta is what the worker knows about the image that the trace doesn't say.
type Meta struct {
	URL    string // relative to the site root
	Credit string
	SizePx [2]int
}

// Window sill and head heights when a trace leaves them out, as kit.js's win().
const (
	defaultSill = 0.9
	defaultHead = 2.1
)

// scale converts trace pixels to plan metres.
type scale struct {
	sx, sy, ox, oy float64
}

func (s scale) pt(p [2]float64) [2]float64 {
	return [2]float64{mm((p[0] - s.ox) / s.sx), mm((p[1] - s.oy) / s.sy)}
}

func (s scale) poly(ps [][2]float64) [][2]float64 {
	out := make([][2]float64, len(ps))
	for i, p := range ps {
		out[i] = s.pt(p)
	}
	return out
}

// angle converts a bearing measured in the image into plan space. They differ
// when the axes have different scales: on a squashed image a 45° line in the
// picture is not 45° on the ground.
func (s scale) angle(deg float64) float64 {
	r := deg * math.Pi / 180
	return math.Round(math.Atan2(math.Sin(r)/s.sy, math.Cos(r)/s.sx)*180/math.Pi*10) / 10
}

// mm rounds to the millimetre, as fromPx does, so plan.json stays readable.
func mm(v float64) float64 { return math.Round(v*1000) / 1000 }

// Convert turns a checked trace into a plan. The origin moves to the top-left
// of the walls' bounding box, so WIDTH and DEPTH are its far corner.
func Convert(t *Trace, meta Meta) (*Plan, error) {
	sx, sy := t.Calibration.PxPerMetre[0], t.Calibration.PxPerMetre[1]
	if !(sx > 0 && sy > 0) || math.IsInf(sx, 0) || math.IsInf(sy, 0) {
		return nil, ErrNoCalibration
	}
	if len(t.Walls) == 0 {
		return nil, ErrNoWalls
	}
	ox, oy := math.Inf(1), math.Inf(1)
	mx, my := math.Inf(-1), math.Inf(-1)
	extend := func(p [2]float64) {
		ox, oy = math.Min(ox, p[0]), math.Min(oy, p[1])
		mx, my = math.Max(mx, p[0]), math.Max(my, p[1])
	}
	for _, w := range t.Walls {
		extend(w.From)
		extend(w.To)
	}
	for _, p := range t.Outline {
		extend(p)
	}
	s := scale{sx: sx, sy: sy, ox: ox, oy: oy}

	p := &Plan{
		Source: Source{
			URL:      meta.URL,
			Credit:   meta.Credit,
			Title:    t.Title,
			OriginPx: [2]float64{ox, oy},
			SizePx:   meta.SizePx,
		},
		Width:        mm((mx - ox) / sx),
		Depth:        mm((my - oy) / sy),
		Entrance:     s.pt(t.Entrance),
		Thickness:    t.Thickness,
		Walls:        make([]MWall, 0, len(t.Walls)),
		Rooms:        make([]MRoom, 0, len(t.Rooms)),
		FloorPatches: make([]Patch, 0, len(t.FloorPatches)),
		Furniture:    make([]map[string]any, 0, len(t.Furniture)),
	}
	if sx == sy {
		p.Source.PxPerMetre = sx
	} else {
		p.Source.PxPerMetre = []float64{sx, sy}
	}
	if len(t.Outline) > 0 {
		p.Outline = s.poly(t.Outline)
	}
	for _, w := range t.Walls {
		p.Walls = append(p.Walls, convertWall(s, w))
	}
	for _, r := range t.Rooms {
		m := MRoom{ID: r.ID, Name: r.Name, Zone: r.Zone, Floor: r.Floor, Polygon: s.poly(r.Polygon)}
		if r.Label != nil {
			l := s.pt(*r.Label)
			m.Label = &l
		}
		p.Rooms = append(p.Rooms, m)
	}
	for _, f := range t.FloorPatches {
		p.FloorPatches = append(
			p.FloorPatches,
			Patch{ID: f.ID, Name: f.Name, Floor: f.Floor, Polygon: s.poly(f.Polygon)},
		)
	}
	for _, it := range t.Furniture {
		p.Furniture = append(p.Furniture, convertItem(s, it))
	}
	return p, nil
}

func convertWall(s scale, w Wall) MWall {
	m := MWall{
		ID:        w.ID,
		Kind:      w.Kind,
		From:      s.pt(w.From),
		To:        s.pt(w.To),
		Thickness: w.Thickness,
		Ext:       w.Ext,
		Openings:  []Opening{},
	}
	pxAxis := dominant(w.From, w.To)
	mAxis := dominant(m.From, m.To)
	along := func(c float64) float64 {
		// Where the opening's pixel coordinate falls along the wall, then that
		// point's coordinate on the wall's dominant axis in metres. The two
		// axes can differ for a near-diagonal wall on a squashed image.
		span := w.To[pxAxis] - w.From[pxAxis]
		f := 0.0
		if span != 0 {
			f = (c - w.From[pxAxis]) / span
		}
		return mm(m.From[mAxis] + f*(m.To[mAxis]-m.From[mAxis]))
	}
	for _, o := range w.Openings {
		mo := o
		mo.A, mo.B = along(o.A), along(o.B)
		if mo.A > mo.B {
			// Switching axes reversed the wall's direction; keep a < b and the
			// hinge on the same physical end.
			mo.A, mo.B = mo.B, mo.A
			mo.Hinge = map[string]string{"a": "b", "b": "a", "": "b"}[o.Hinge]
		}
		if mo.Kind == "window" {
			if mo.Sill == 0 {
				mo.Sill = defaultSill
			}
			if mo.Head == 0 {
				mo.Head = defaultHead
			}
		}
		m.Openings = append(m.Openings, mo)
	}
	return m
}

func dominant(a, b [2]float64) int {
	if math.Abs(b[0]-a[0]) >= math.Abs(b[1]-a[1]) {
		return 0
	}
	return 1
}

func convertItem(s scale, it Item) map[string]any {
	spec := Types[it.Type]
	pt := s.pt([2]float64{it.X, it.Y})
	out := map[string]any{"type": it.Type, "x": pt[0], "y": pt[1]}
	for k, v := range spec.Defaults {
		out[k] = v
	}
	set := func(k string, v *float64) {
		if v != nil {
			out[k] = *v
		}
	}
	set("w", it.W)
	set("d", it.D)
	set("h", it.H)
	set("r", it.R)
	set("size", it.Size)
	set("z0", it.Z0)
	set("z1", it.Z1)
	set("rise", it.Rise)
	if it.Steps != nil {
		out["steps"] = *it.Steps
	}
	if it.Rot != nil {
		out["rot"] = s.angle(*it.Rot)
	}
	if it.Facing != nil {
		out["facing"] = s.angle(*it.Facing)
	}
	if spec.Oriented == "rot" && it.Rot == nil {
		out["rot"] = 0.0
	}
	if it.Label != "" {
		out["label"] = it.Label
	}
	if it.Mat != "" {
		out["mat"] = it.Mat
	}
	return out
}

// Describe summarises a plan for the tracer's tool result.
func Describe(p *Plan) string {
	doors := 0
	for _, w := range p.Walls {
		doors += len(w.Openings)
	}
	return fmt.Sprintf("%.2f × %.2f m; %d walls, %d openings, %d rooms, %d furniture items",
		p.Width, p.Depth, len(p.Walls), doors, len(p.Rooms), len(p.Furniture))
}
