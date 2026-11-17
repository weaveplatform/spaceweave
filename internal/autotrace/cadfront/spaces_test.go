package cadfront

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
	"github.com/weaveplatform/spaceweave/internal/floorgraph"
	"github.com/weaveplatform/spaceweave/internal/plan"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// writeDoc saves a fixture where ReadSpaces can open it.
func writeDoc(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func same(x, y float64) (float64, float64) { return x, y }

// checkOfficeSpaces checks every area of OfficeSpaceList came back named,
// numbered, sized and placed over the office's own rooms, within tol
// pixels.
func checkOfficeSpaces(t *testing.T, spaces []floorgraph.Space, o Options, tol float64) {
	t.Helper()
	if len(spaces) != len(dxftest.OfficeSpaceList) {
		t.Fatalf("%d spaces: %+v", len(spaces), spaces)
	}
	for i, want := range dxftest.OfficeSpaceList {
		got := spaces[i]
		if got.Number != want.Number || got.Name != want.Name {
			t.Errorf(
				"space %d is %q %q, want %q %q",
				i,
				got.Number,
				got.Name,
				want.Number,
				want.Name,
			)
		}
		if a := (want.X1 - want.X0) * (want.Y1 - want.Y0) / 1e6; math.Abs(got.Area-a) > 0.05 {
			t.Errorf("%s: area %.2f, want %.2f", want.Name, got.Area, a)
		}
		kind := plan.KindRoom
		if want.X1-want.X0 < 6000 {
			kind = plan.KindZone
		}
		if got.Kind != kind {
			t.Errorf("%s is a %s, want %s", want.Name, got.Kind, kind)
		}
		corners := []point{
			mm(o, want.X0, want.Y0),
			mm(o, want.X1, want.Y0),
			mm(o, want.X1, want.Y1),
			mm(o, want.X0, want.Y1),
		}
		for j, c := range corners {
			p := point{X: got.Polygon[j][0], Y: got.Polygon[j][1]}
			if !close2(p, c, tol) {
				t.Errorf("%s corner %d at %v, want %v", want.Name, j, p, c)
			}
		}
	}
}

// TestReadSpacesSharedCoordinates is the usual package: the space plan
// drawn in the drawing's own coordinates and units, which is believed as
// it is.
func TestReadSpacesSharedCoordinates(t *testing.T) {
	arch, o := read(t, dxftest.Office(4))
	path := writeDoc(t, "office spaces.dxf", dxftest.OfficeSpaces(4, same).Bytes())
	spaces, reg, err := ReadSpaces(path, o, arch)
	if err != nil {
		t.Fatal(err)
	}
	if reg.Method != Shared || reg.Poor || reg.Transform != vector.Identity ||
		reg.Layer != "EC1 Space Polygons" {
		t.Fatalf("%+v", reg)
	}
	if reg.Matched < 0.99 || reg.ResidualM > 0.01 {
		t.Fatalf("every corner is on a wall centreline: %+v", reg)
	}
	checkOfficeSpaces(t, spaces, o, 0.01)
}

// TestReadSpacesFitted is a space plan exported apart from the drawing:
// in centimetres from its own origin, turned a little, with a header that
// says inches. The written areas settle the units, and fitting its corners
// to the walls puts it back over the office.
func TestReadSpacesFitted(t *testing.T) {
	arch, o := read(t, dxftest.Office(4))
	turn := 1.5 * math.Pi / 180
	moved := vector.Scale(0.1, 0.1).Then(vector.Rotate(turn)).Then(vector.Translate(52000, -31000))
	place := func(x, y float64) (float64, float64) {
		p := moved.Apply(point{X: x, Y: y})
		return p.X, p.Y
	}
	path := writeDoc(t, "moved spaces.dxf", dxftest.OfficeSpaces(1, place).Bytes())
	spaces, reg, err := ReadSpaces(path, o, arch)
	if err != nil {
		t.Fatal(err)
	}
	if reg.Method != Fitted || reg.Poor || reg.Matched < 0.99 || reg.ResidualM > 0.02 {
		t.Fatalf("%+v", reg)
	}
	if !strings.Contains(strings.Join(reg.Notes, " "), "areas written") {
		t.Fatalf("the header's inches were overruled by the written areas: %q", reg.Notes)
	}
	// The transform found undoes the move, to a millimetre over the floor.
	back := moved.Then(reg.Transform)
	for _, c := range []point{{}, {X: 12000, Y: 8000}} {
		if p := back.Apply(c); !close2(p, c, 1) {
			t.Errorf("%v comes back to %v", c, p)
		}
	}
	checkOfficeSpaces(t, spaces, o, 0.2)
}

// TestReadSpacesPoor is space plans that don't fit the drawing: they are
// said to be poorly placed rather than handed over as if they were right.
func TestReadSpacesPoor(t *testing.T) {
	arch, o := read(t, dxftest.Office(4))
	// Stretched and turned on its side: however it is turned and moved at
	// the scale its written areas give, too few corners meet a wall.
	other := func(x, y float64) (float64, float64) { return 3*y + 40000, 2*x - 90000 }
	_, reg, err := ReadSpaces(
		writeDoc(t, "other.dxf", dxftest.OfficeSpaces(4, other).Bytes()),
		o,
		arch,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reg.Poor || reg.Method != Fitted || reg.Matched >= goodMatch {
		t.Fatalf("%+v", reg)
	}

	// The office's spaces over a drawing of another floor in the same
	// coordinates: most corners lie on no wall.
	part, po := read(t, dxftest.Partitioned(4, "A-WALL"))
	spaces := writeDoc(t, "office spaces.dxf", dxftest.OfficeSpaces(4, same).Bytes())
	if _, reg, err = ReadSpaces(
		spaces,
		po,
		part,
	); err != nil || !reg.Poor ||
		reg.Matched >= goodMatch {
		t.Fatalf("%+v %v", reg, err)
	}

	// No walls to check against: shared coordinates are believed as they
	// are, and anything else can't be placed.
	bare := arch
	bare.Walls, bare.Openings = nil, nil
	got, reg, err := ReadSpaces(spaces, o, bare)
	if err != nil || reg.Poor || reg.Method != Shared || reg.Matched != -1 || got[0].Kind != "" {
		t.Fatalf("%+v %v", reg, err)
	}
	away := func(x, y float64) (float64, float64) { return x + 1e6, y }
	_, reg, err = ReadSpaces(
		writeDoc(t, "away.dxf", dxftest.OfficeSpaces(4, away).Bytes()),
		o,
		bare,
	)
	if err != nil || !reg.Poor || reg.Method != "" || !strings.Contains(reg.Notes[0], "no walls") {
		t.Fatalf("%+v %v", reg, err)
	}
}

// TestReadSpacesUnits is a space plan that states no units and writes no
// areas: each common unit is tried, and the one whose outlines land on
// the walls is taken; and a drawing of unknown scale takes the space
// plan's.
func TestReadSpacesUnits(t *testing.T) {
	arch, o := read(t, dxftest.Office(4))
	d := dxftest.New(-1).Layer("A-AREA", 3, 0)
	for _, s := range dxftest.OfficeSpaceList {
		// In centimetres, 20 m off, without saying so.
		d.Add(dxftest.Rect("A-AREA", s.X0/10+2000, s.Y0/10, s.X1/10+2000, s.Y1/10))
	}
	got, reg, err := ReadSpaces(writeDoc(t, "unitless.dxf", d.Bytes()), o, arch)
	if err != nil {
		t.Fatal(err)
	}
	notes := strings.Join(reg.Notes, " ")
	if !strings.Contains(notes, "each common unit was tried") || reg.Method != Fitted || reg.Poor ||
		math.Abs(reg.Transform.Factor()-10) > 1e-9 {
		t.Fatalf("%+v", reg)
	}
	total := 0.0
	for _, sp := range got {
		total += sp.Area
		if sp.Name != "" || sp.Number != "" {
			t.Fatalf("nothing is written in it: %+v", sp)
		}
	}
	if math.Abs(total-96) > 0.01 {
		t.Fatalf("areas are measured in the units found: %.2f m² in all", total)
	}

	// Neither file's scale is known: the units are taken to agree.
	unknown := o
	unknown.PxPerMetre = 0
	bare := arch
	bare.Scale = floorgraph.Scale{}
	spaces := writeDoc(t, "s.dxf", dxftest.OfficeSpaces(4, same).Bytes())
	if _, reg, err = ReadSpaces(
		spaces,
		unknown,
		bare,
	); err != nil || reg.Method != Shared ||
		reg.Poor {
		t.Fatalf("%+v %v", reg, err)
	}
}

func TestFitTurn(t *testing.T) {
	want := vector.Scale(2, 2).Then(vector.Rotate(0.3)).Then(vector.Translate(5, -7))
	from := []point{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 0, Y: 5}}
	to := make([]point, len(from))
	for i, p := range from {
		to[i] = want.Apply(p)
	}
	got, ok := fitTurn(from, to, 2)
	if !ok || !close2(got.Apply(point{X: 3, Y: 4}), want.Apply(point{X: 3, Y: 4}), 1e-9) {
		t.Fatalf("%+v", got)
	}
	if got, ok = fitTurn(from, to, 1); !ok || math.Abs(got.Factor()-1) > 1e-12 {
		t.Fatalf("the scale is kept: %+v", got)
	}
	for _, bad := range [][2][]point{
		{from[:1], to[:1]},
		{from, to[:2]},
		{{{X: 1, Y: 1}, {X: 1, Y: 1}}, {{}, {X: 1}}},
	} {
		if _, ok := fitTurn(bad[0], bad[1], 1); ok {
			t.Errorf("%v fixes no transform", bad)
		}
	}
	if boxShare(vector.Rect{}, vector.Rect{}.Extend(point{})) != 0 {
		t.Error("an empty box shares nothing")
	}
}

func TestReadSpacesErrors(t *testing.T) {
	arch, o := read(t, dxftest.Office(4))
	if _, _, err := ReadSpaces(filepath.Join(t.TempDir(), "missing.dxf"), o, arch); err == nil {
		t.Fatal("a missing file")
	}
	if _, _, err := ReadSpaces(
		writeDoc(t, "bad.dxf", []byte("0\nSECTION\n")),
		o,
		arch,
	); err == nil {
		t.Fatal("a damaged file")
	}
	office := writeDoc(t, "office.dxf", dxftest.Office(4).Bytes())
	if _, _, err := ReadSpaces(office, o, arch); !errors.Is(err, ErrNoSpaces) {
		t.Fatalf("an architectural drawing has no space outlines: %v", err)
	}
	if _, _, err := ReadSpaces(office, Options{}, arch); !errors.Is(err, ErrView) {
		t.Fatalf("no view: %v", err)
	}
}
