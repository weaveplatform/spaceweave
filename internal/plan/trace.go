// Package plan holds the trace a model writes while reading a floor-plan
// image and turns it into the plan.json the microsite renders.
//
// A trace is in source-image pixels for everything you can point at (wall
// ends, opening ends, room corners, furniture centres) and in metres for
// sizes and heights, which are easier to state from experience than to
// measure. Conversion owns the calibration, so a model never does arithmetic
// it could get wrong, and a squashed image (different px/m across and down)
// is handled in one place.
package plan

// Trace is what the tracer submits.
type Trace struct {
	Title        string       `json:"title"`
	Calibration  Calibration  `json:"calibration"`
	Thickness    Thickness    `json:"thickness"`
	Outline      [][2]float64 `json:"outline,omitempty"`
	Entrance     [2]float64   `json:"entrance"`
	Walls        []Wall       `json:"walls"`
	Rooms        []Room       `json:"rooms"`
	FloorPatches []Patch      `json:"floorPatches,omitempty"`
	Furniture    []Item       `json:"furniture"`
}

// Calibration maps pixels to metres. PxPerMetre is [across, down]; equal
// values are the usual case.
type Calibration struct {
	PxPerMetre [2]float64 `json:"pxPerMetre"`
	Evidence   string     `json:"evidence"`
}

// Thickness is the default wall thickness per kind, metres.
type Thickness struct {
	Exterior float64 `json:"exterior"`
	Interior float64 `json:"interior"`
	Glass    float64 `json:"glass"`
}

// Wall is a wall centreline in pixels. Thickness, when set, overrides the
// kind's default; Ext is how far the wall runs past each end point (the
// renderer's default is half its thickness, which closes corners; 0 stops two
// walls meeting at a corner from overlapping). Both are metres.
type Wall struct {
	ID        string     `json:"id"`
	Kind      string     `json:"kind"`
	From      [2]float64 `json:"from"`
	To        [2]float64 `json:"to"`
	Thickness float64    `json:"thickness,omitempty"`
	Ext       *float64   `json:"ext,omitempty"`
	Openings  []Opening  `json:"openings,omitempty"`
}

// Opening is a gap in a wall. A and B are pixel coordinates along the wall's
// dominant axis (x for a mostly horizontal wall, y for a mostly vertical one),
// the same convention as js/plans/kit.js. Sill and Head are metres.
type Opening struct {
	A     float64 `json:"a"`
	B     float64 `json:"b"`
	Kind  string  `json:"kind"`
	Swing string  `json:"swing,omitempty"`
	Hinge string  `json:"hinge,omitempty"`
	Sill  float64 `json:"sill,omitempty"`
	Head  float64 `json:"head,omitempty"`
}

// Room is a named floor area; Polygon and Label are pixels.
type Room struct {
	ID      string       `json:"id"`
	Name    string       `json:"name"`
	Zone    string       `json:"zone"`
	Floor   string       `json:"floor"`
	Polygon [][2]float64 `json:"polygon"`
	Label   *[2]float64  `json:"label,omitempty"`
}

// Patch is a floor finish laid over part of a room, e.g. kitchen tiles.
type Patch struct {
	ID      string       `json:"id"`
	Name    string       `json:"name"`
	Floor   string       `json:"floor"`
	Polygon [][2]float64 `json:"polygon"`
}

// Item is a piece of furniture. X and Y are its centre in pixels; Rot and
// Facing are degrees in the image (0 = right, 90 = down); every size and
// height is metres. Which fields an item needs depends on Type (see Types).
type Item struct {
	Type   string   `json:"type"`
	X      float64  `json:"x"`
	Y      float64  `json:"y"`
	W      *float64 `json:"w,omitempty"`
	D      *float64 `json:"d,omitempty"`
	H      *float64 `json:"h,omitempty"`
	R      *float64 `json:"r,omitempty"`
	Rot    *float64 `json:"rot,omitempty"`
	Facing *float64 `json:"facing,omitempty"`
	Size   *float64 `json:"size,omitempty"`
	Label  string   `json:"label,omitempty"`
	Mat    string   `json:"mat,omitempty"`
	Z0     *float64 `json:"z0,omitempty"`
	Z1     *float64 `json:"z1,omitempty"`
	Steps  *int     `json:"steps,omitempty"`
	Rise   *float64 `json:"rise,omitempty"`
}

// Spec says what a furniture type needs. Required fields must be given;
// Defaults fill fields the tracer may leave out.
type Spec struct {
	Required []string
	Defaults map[string]float64
	Oriented string // "rot", "facing" or "" — which angle field it takes
}

// Types is the furniture vocabulary of furniturePrisms in js/geometry.js.
// A type added there needs an entry here, or traces using it are rejected.
var Types = map[string]Spec{
	"desk": {
		Required: []string{"w", "d"},
		Defaults: map[string]float64{"h": 0.75},
		Oriented: "rot",
	},
	"counter": {
		Required: []string{"w", "d"},
		Defaults: map[string]float64{"h": 0.9},
		Oriented: "rot",
	},
	"cabinet": {
		Required: []string{"w", "d"},
		Defaults: map[string]float64{"h": 0.9},
		Oriented: "rot",
	},
	"storage": {
		Required: []string{"w", "d"},
		Defaults: map[string]float64{"h": 1.8},
		Oriented: "rot",
	},
	"copier": {
		Required: []string{"w", "d"},
		Defaults: map[string]float64{"h": 1.1},
		Oriented: "rot",
	},
	"sink": {Required: []string{"w", "d"}, Defaults: map[string]float64{"h": 0.9}},
	"board": {
		Required: []string{"w", "d"},
		Defaults: map[string]float64{"z0": 0.8, "z1": 2.0},
		Oriented: "rot",
	},
	"roundTable": {Required: []string{"r"}, Defaults: map[string]float64{"h": 0.74}},
	"chair":      {Defaults: map[string]float64{"size": 0.5}, Oriented: "facing"},
	"armchair":   {Oriented: "facing"},
	"tub":        {Oriented: "facing"},
	"stool":      {},
	"plant":      {Defaults: map[string]float64{"r": 0.3}},
	"toilet":     {Oriented: "facing"},
	"basin":      {Oriented: "facing"},
	"block": {
		Required: []string{"w", "d", "h", "mat"},
		Defaults: map[string]float64{"z0": 0},
		Oriented: "rot",
	},
	"sofa": {Required: []string{"w"}, Oriented: "facing"},
	"stairs": {
		Required: []string{"w", "d"},
		Defaults: map[string]float64{"rise": 1.5, "steps": 9},
		Oriented: "facing",
	},
}

// Materials a block may use: the palette in js/render.js.
var Materials = []string{
	"wall", "wood", "metal", "fabric", "fabricB", "cabinet", "counter", "steel",
	"appliance", "board", "ceramic", "pot", "plant", "door",
}

// Zones and Floors are the room classifications the renderer colours.
var (
	Zones  = []string{"meet", "work", "social", "service"}
	Floors = []string{"carpet", "wood", "tile"}
)
