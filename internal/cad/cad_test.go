package cad

import (
	"math"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/vector"
)

// eps is loose enough for the Bézier and trigonometric round trips the
// tests compare against, and far tighter than any mistake they look for.
const eps = 1e-6

func near(a, b float64) bool { return math.Abs(a-b) < eps }

func nearPt(a, b Point) bool { return near(a.X, b.X) && near(a.Y, b.Y) }

func TestUnits(t *testing.T) {
	tests := []struct {
		u      Units
		metres float64
		name   string
	}{
		{u: Unitless, metres: 0, name: "unitless"},
		{u: Inches, metres: 0.0254, name: "inches"},
		{u: Feet, metres: 0.3048, name: "feet"},
		{u: Millimeters, metres: 0.001, name: "millimetres"},
		{u: Centimeters, metres: 0.01, name: "centimetres"},
		{u: Meters, metres: 1, name: "metres"},
		{u: Kilometers, metres: 1000, name: "kilometres"},
		{u: Yards, metres: 0.9144, name: "yards"},
		{u: Decimeters, metres: 0.1, name: "decimetres"},
		// Microinches and parsecs are real $INSUNITS codes a plan never
		// uses; they must read as unknown, not as some other unit.
		{u: 8, metres: 0, name: "unitless"},
		{u: 3, metres: 0, name: "unitless"},
		{u: -1, metres: 0, name: "unitless"},
		{u: 99, metres: 0, name: "unitless"},
	}
	for _, tt := range tests {
		if got := tt.u.Metres(); got != tt.metres {
			t.Errorf("Units(%d).Metres() = %v, want %v", int(tt.u), got, tt.metres)
		}
		if got := tt.u.String(); got != tt.name {
			t.Errorf("Units(%d).String() = %q, want %q", int(tt.u), got, tt.name)
		}
	}
}

func TestNewDocumentHasLayerZero(t *testing.T) {
	doc := NewDocument()
	l := doc.Layers["0"]
	if l == nil || l.Name != "0" || l.Color != 7 {
		t.Fatalf("layer 0 = %+v, want name 0 colour 7", l)
	}
	if doc.Blocks == nil {
		t.Fatal("Blocks is nil; readers add to it without checking")
	}
	// Layer 0's colour 7 must draw black on paper, or every default entity
	// would vanish into a white background.
	if got := ACI(l.Color); got != vector.Black {
		t.Errorf("ACI(7) = %v, want black", got)
	}
}

func TestBaseReturnsCommon(t *testing.T) {
	l := &Line{Common: Common{Layer: "walls"}}
	var e Entity = l
	if e.Base() != &l.Common {
		t.Fatal("Base does not return the entity's own Common")
	}
	e.Base().Color = 3
	if l.Color != 3 {
		t.Error("Base returned a copy, so readers' writes through it are lost")
	}
}
