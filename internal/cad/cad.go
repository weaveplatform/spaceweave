// Package cad is the drawing model the DXF and DWG readers share, and Draw,
// which turns it into a vector.Drawing. It holds what a floor plan needs to
// be seen (geometry, text, blocks, layers and units), not everything a CAD
// file can say.
//
// Angles are radians and points are 2D: plans are drawn flat, and Z is
// dropped. Entity semantics follow the DXF reference; ezdxf (MIT) was a guide
// to the corners of it.
package cad

import "github.com/weaveplatform/spaceweave/internal/vector"

// Point is a 2D point in drawing units.
type Point = vector.Point

// Units is a drawing's $INSUNITS code.
type Units int

// The units a floor plan is likely to be drawn in. Codes not listed have no
// sensible length and count as unitless.
const (
	Unitless    Units = 0
	Inches      Units = 1
	Feet        Units = 2
	Millimeters Units = 4
	Centimeters Units = 5
	Meters      Units = 6
	Kilometers  Units = 7
	Yards       Units = 10
	Decimeters  Units = 14
)

var metres = map[Units]float64{
	Inches: 0.0254, Feet: 0.3048, Millimeters: 0.001, Centimeters: 0.01, Meters: 1,
	Kilometers: 1000, Yards: 0.9144, Decimeters: 0.1,
}

var unitNames = map[Units]string{
	Inches: "inches", Feet: "feet", Millimeters: "millimetres", Centimeters: "centimetres",
	Meters: "metres", Kilometers: "kilometres", Yards: "yards", Decimeters: "decimetres",
}

// Metres is the length of one unit, or 0 when it isn't known.
func (u Units) Metres() float64 { return metres[u] }

func (u Units) String() string {
	if n, ok := unitNames[u]; ok {
		return n
	}
	return "unitless"
}

// Colour numbers with special meanings (the AutoCAD Color Index).
const (
	ByBlock = 0
	ByLayer = 256
)

// Layer is a drawing layer.
type Layer struct {
	Name   string
	Color  int // ACI
	RGB    *vector.Color
	Off    bool
	Frozen bool
}

// Common is what every entity has.
type Common struct {
	Layer     string
	Color     int // ACI; ByLayer or ByBlock
	RGB       *vector.Color
	Invisible bool
	// Mirrored is set when the entity's extrusion points down the Z axis:
	// its object coordinates then run right to left. This is how CAD stores
	// a mirrored arc, circle or block.
	Mirrored bool
}

// Base returns the common data.
func (c *Common) Base() *Common { return c }

// Entity is a drawable thing.
type Entity interface{ Base() *Common }

// Line is a straight line.
type Line struct {
	Common
	A, B Point
}

// Circle is a full circle.
type Circle struct {
	Common
	Center Point
	Radius float64
}

// Arc runs counter-clockwise from Start to End.
type Arc struct {
	Common
	Center     Point
	Radius     float64
	Start, End float64
}

// Ellipse is a full or partial ellipse. Major is the semi-major axis from the
// centre; the minor axis is Ratio times as long, a quarter turn
// counter-clockwise. Start and End are parameters, not angles.
type Ellipse struct {
	Common
	Center     Point
	Major      Point
	Ratio      float64
	Start, End float64
}

// Vertex is a polyline vertex. Bulge is the tangent of a quarter of the
// included angle of the arc to the next vertex; 0 is a straight segment and
// a positive bulge turns counter-clockwise.
type Vertex struct {
	P     Point
	Bulge float64
}

// Polyline is an LWPOLYLINE or a 2D POLYLINE.
type Polyline struct {
	Common
	Vertices []Vertex
	Closed   bool
	Width    float64 // constant width in drawing units, 0 for a thin line
}

// Spline is a B-spline, by control points and knots, or, when a file only
// has them, by fit points.
type Spline struct {
	Common
	Degree  int
	Knots   []float64
	Control []Point
	Weights []float64
	Fit     []Point
	Closed  bool
}

// Text is a single-line TEXT or ATTRIB. Align is DXF group 72 and VAlign
// group 73; At is the point they anchor (the insertion point when the
// text is left and baseline aligned, the alignment point otherwise).
type Text struct {
	Common
	At          Point
	Height      float64
	Rotation    float64
	WidthFactor float64
	Align       int
	VAlign      int
	Content     string
}

// MText is multi-line text. Attach is DXF group 71: 1 top left to 9 bottom
// right, row by row.
type MText struct {
	Common
	At       Point
	Height   float64
	Rotation float64
	Attach   int
	Content  string // with MTEXT formatting codes
}

// Insert places a block. Columns and Rows above 1 make an MINSERT array.
type Insert struct {
	Common
	Block             string
	At                Point
	Scale             Point
	Rotation          float64
	Columns, Rows     int
	ColumnGap, RowGap float64
	Attribs           []*Text
}

// Solid is a filled triangle or quadrilateral. Its corners are in DXF order:
// the third and fourth are swapped relative to going round the shape.
type Solid struct {
	Common
	Corners [4]Point
}

// Hatch is a filled area. A pattern hatch (cross-hatching, tiles) is shown
// as a light fill: drawing every pattern line costs more than it tells the
// tracer.
type Hatch struct {
	Common
	Solid    bool
	Boundary vector.Path // closed loops; holes are told apart by nesting
}

// Dimension is drawn through its block, which CAD regenerates on every edit
// and stores with the arrows, lines and text in place.
type Dimension struct {
	Common
	Block string
}

// Block is a named group of entities. Base is the point that lands on an
// insert's position.
type Block struct {
	Name     string
	Base     Point
	Entities []Entity
}

// Document is a whole drawing.
type Document struct {
	Version string // AC1015 etc.
	Units   Units
	Layers  map[string]*Layer
	Blocks  map[string]*Block
	// Model is model space, where the building is drawn; Paper is the
	// active layout's paper space, drawn only when model space is empty.
	Model, Paper []Entity
	// Skipped counts what the reader couldn't decode and left out.
	Skipped int
	// Notes are things a reader should know.
	Notes []string
}

// NewDocument returns an empty drawing with layer 0, which every drawing has.
func NewDocument() *Document {
	return &Document{
		Layers: map[string]*Layer{"0": {Name: "0", Color: 7}},
		Blocks: map[string]*Block{},
	}
}
