package cadfront

import (
	"slices"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/autotrace/profile"
	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
	"github.com/weaveplatform/spaceweave/internal/floorgraph"
)

// builtinFacets is what the built-in profiles make of a layer name, for
// the tests that check the conventions name by name.
func builtinFacets(name string) facets {
	m, ok := (*profile.Set)(nil).Resolve(name)
	if !ok {
		return facets{}
	}
	return roleFacets[m.Role]
}

func layerRole(name string) role           { return builtinFacets(name).lines }
func textLayer(name string) textClass      { return builtinFacets(name).text }
func sanitaryLayer(name string) bool       { return builtinFacets(name).sanitary }
func deskNumberLayer(name string) bool     { return builtinFacets(name).desks }
func layerBlockRole(block, layer string) role { return blockRole(block, layerRole(layer)) }

func analyse(t *testing.T, d *dxftest.Doc, set *profile.Set) (floorgraph.Input, map[string]profile.Layer, Options) {
	t.Helper()
	doc := parse(t, d)
	o, err := Frame(doc, testRaster)
	if err != nil {
		t.Fatal(err)
	}
	o.Profile = set
	in, layers, err := Analyse(doc, o)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.IsSortedFunc(layers, func(a, b profile.Layer) int { return strings.Compare(a.Name, b.Name) }) {
		t.Errorf("layers not sorted by name: %+v", layers)
	}
	by := map[string]profile.Layer{}
	for _, l := range layers {
		by[l.Name] = l
	}
	return in, by, o
}

// TestClientProfileFindsWalls is a partition on a layer no convention
// knows: not a wall in a drawing that names its wall layer, until a
// client profile says it is one.
func TestClientProfileFindsWalls(t *testing.T) {
	const odd = "Q-TRENNWAND"
	in, layers, o := analyse(t, dxftest.Partitioned(4, odd), nil)
	tol := 0.02 * o.View.Factor() * 1000
	if _, ok := findWall(in, mm(o, 5000, 1900), mm(o, 5000, 6000), tol); ok {
		t.Fatalf("the partition is a wall with no profile naming its layer: %+v", in.Walls)
	}
	want := map[string]profile.Layer{
		"A-WALL":      {Name: "A-WALL", Role: profile.Wall, Entities: 8, Source: "builtin:aia"},
		"A-DOOR":      {Name: "A-DOOR", Role: profile.Door, Entities: 4, Source: "builtin:aia"}, // the insert and its layer-0 lines
		"A-ANNO-TEXT": {Name: "A-ANNO-TEXT", Role: profile.Annotation, Entities: 2, Source: "builtin:generic"},
		odd:           {Name: odd, Role: profile.Unknown, Entities: 4, Source: profile.Guess},
	}
	for name, w := range want {
		if layers[name] != w {
			t.Errorf("%s: %+v, want %+v", name, layers[name], w)
		}
	}

	acme, err := profile.NewSet("acme", &profile.Profile{Rules: []profile.Rule{{Role: profile.Wall, Layer: odd}}})
	if err != nil {
		t.Fatal(err)
	}
	in, layers, o = analyse(t, dxftest.Partitioned(4, odd), acme)
	if _, ok := findWall(in, mm(o, 5000, 1900), mm(o, 5000, 6000), tol); !ok {
		t.Fatalf("the profile makes the partition a wall: %+v", in.Walls)
	}
	if l := layers[odd]; l.Role != profile.Wall || l.Source != "profile:acme" {
		t.Errorf("%+v", l)
	}
}

// TestGuessedRoles are layers no rule names, reported by what was made
// of them: walls in a drawing that names no wall layer, doors found by
// their swings, text taken as room names.
func TestGuessedRoles(t *testing.T) {
	_, layers, _ := analyse(t, dxftest.Unlayered(4), nil)
	for name, role := range map[string]profile.Role{"L1": profile.Wall, "L2": profile.RoomName, "0": profile.Door} {
		if l := layers[name]; l.Role != role || l.Source != profile.Guess {
			t.Errorf("%s: %+v, want a guessed %s", name, l, role)
		}
	}
	d := dxftest.New(4).Layer("BLOCKS", 7, 0).Layer("NOTHING", 7, 0).
		Block("WIN1200", 0, 0, dxftest.Line("0", 0, 0, 1200, 0)).
		Block("CHAIR", 0, 0, dxftest.Rect("0", 0, 0, 500, 500))
	d.Add(
		dxftest.Insert("BLOCKS", "WIN1200", 0, 0, 0),
		dxftest.Insert("BLOCKS", "WIN1200", 3000, 0, 0),
		dxftest.Insert("BLOCKS", "CHAIR", 1000, 1000, 0),
		dxftest.Text("NOTHING", 0, 0, 100, "1200x800"),
	)
	_, layers, _ = analyse(t, d, nil)
	if l := layers["BLOCKS"]; l.Role != profile.Window || l.Source != profile.Guess || l.Entities != 6 {
		t.Errorf("most of its blocks are windows: %+v", l)
	}
	if l := layers["NOTHING"]; l.Role != profile.Unknown {
		t.Errorf("text naming nothing is no evidence: %+v", l)
	}
}

func TestRoleFacets(t *testing.T) {
	for _, r := range profile.Roles {
		if _, ok := roleFacets[r.Role]; !ok {
			t.Errorf("role %q has no facets: the reader would treat it as no role at all", r.Role)
		}
	}
	if len(roleFacets) != len(profile.Roles) {
		t.Errorf("%d facets for %d roles", len(roleFacets), len(profile.Roles))
	}
}
