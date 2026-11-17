package floorgraph

import (
	"errors"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/plan"
)

func thick(s Segment, t float64) Segment {
	s.Thickness = t
	return s
}

// partitioned is an 8 × 6 m office at 50 px/m: exterior walls 12 px thick,
// a partition down the middle and one across the right half, each meeting
// the wall beyond in a T.
func partitioned() Input {
	var walls []Segment
	for _, w := range box(20, 20, 420, 320) {
		walls = append(walls, thick(w, 12))
	}
	walls = append(walls, wall(220, 20, 220, 320), wall(220, 170, 420, 170))
	return Input{
		Title: "Partitioned",
		Size:  [2]int{440, 340},
		Walls: walls,
		Labels: []Label{
			{At: P(120, 170), Text: "Open  Office", Height: 10},
			{At: P(320, 90), Text: "Meeting Room 2", Height: 10},
			{At: P(320, 250), Text: "Kitchen", Height: 10},
			{At: P(320, 270), Text: "3.60 x 2.40", Height: 14},
		},
		Scale:    scale50,
		Coverage: -1,
	}
}

// lShape is an L-shaped building with one partition.
func lShape() Input {
	return Input{
		Walls: []Segment{
			wall(0, 0, 400, 0), wall(400, 0, 400, 200), wall(400, 200, 200, 200),
			wall(200, 200, 200, 400), wall(200, 400, 0, 400), wall(0, 400, 0, 0),
			wall(0, 200, 200, 200),
		},
		Scale:    scale50,
		Coverage: 0.9,
	}
}

// slanted has a splayed facade drawn in two pieces a degree apart.
func slanted() Input {
	return Input{
		Walls: []Segment{
			wall(0, 0, 300, 0), wall(0, 300, 400, 300), wall(0, 0, 0, 300),
			wall(300, 0, 350, 150.5), wall(349, 149, 400, 300),
			wall(0, 150, 349, 150),
		},
		Scale:    scale50,
		Coverage: -1,
	}
}

// glazed is a box split by a glazed partition.
func glazed() Input {
	in := Input{Walls: box(0, 0, 400, 300), Scale: scale50, Coverage: -1}
	g := wall(200, 0, 200, 300)
	g.Glass, g.Thickness = true, 3
	in.Walls = append(in.Walls, g)
	return in
}

// unclosed is three sides of a building and a stray partition.
func unclosed() Input {
	return Input{
		Walls: []Segment{
			wall(0, 0, 400, 0),
			wall(400, 0, 400, 300),
			wall(0, 0, 0, 300),
			wall(200, 0, 200, 150),
		},
		Scale:    scale50,
		Coverage: -1,
	}
}

// cored is an open floor with a free-standing core of two rooms in it.
func cored() Input {
	walls := box(0, 0, 600, 400)
	walls = append(walls, box(200, 150, 400, 250)...)
	walls = append(walls, wall(300, 150, 300, 250))
	return Input{
		Walls: walls,
		Labels: []Label{
			{At: P(250, 200), Text: "Lift", Height: 9},
			{At: P(350, 200), Text: "Stair", Height: 9},
		},
		Scale:    scale50,
		Coverage: -1,
	}
}

// doors is the partitioned office with a door in every wall and the walls
// broken at them.
func doors() Input {
	in := partitioned()
	in.Walls = []Segment{
		// North wall, broken at an exterior door 100–145.
		thick(wall(20, 20, 100, 20), 12),
		thick(wall(145, 20, 420, 20), 12),
		thick(
			wall(420, 20, 420, 320),
			12,
		),
		thick(wall(420, 320, 20, 320), 12),
		thick(wall(20, 320, 20, 20), 12),
		// The middle partition, broken at a door 60–105 (y).
		wall(220, 20, 220, 60),
		wall(220, 105, 220, 320),
		// The cross partition, broken at a door 280–325 (x), nearer the
		// north wall so the rooms either side differ.
		wall(220, 150, 280, 150),
		wall(325, 150, 420, 150),
	}
	in.Openings = []Opening{
		// Exterior door, leaf inside (S), hinged at its west end given as B.
		{
			A:      P(145, 20),
			B:      P(100, 20),
			Kind:   Door,
			Leaf:   P(122, 40),
			Leafed: true,
			HingeA: false,
			Weight: 1,
		},
		// Vertical door, leaf to the east, hinged at the top (A).
		{
			A:      P(220, 60),
			B:      P(220, 105),
			Kind:   Door,
			Leaf:   P(240, 82),
			Leafed: true,
			HingeA: true,
			Weight: 1,
		},
		// Horizontal door with no leaf found.
		{A: P(280, 150), B: P(325, 150), Kind: Door, Weight: 1},
		// A window on the south wall, which isn't broken.
		{A: P(60, 320), B: P(160, 320), Kind: Window, Weight: 1},
	}
	return in
}

func synthetic() map[string]Input {
	return map[string]Input{
		"partitioned": partitioned(), "l-shape": lShape(), "slanted": slanted(), "glazed": glazed(),
		"unclosed": unclosed(), "cored": cored(), "doors": doors(),
	}
}

func wallsOf(r *Result, kind string) []plan.Wall {
	var out []plan.Wall
	for _, w := range r.Trace.Walls {
		if w.Kind == kind {
			out = append(out, w)
		}
	}
	return out
}

func roomNamed(t *testing.T, r *Result, name string) plan.Room {
	t.Helper()
	for _, rm := range r.Trace.Rooms {
		if rm.Name == name {
			return rm
		}
	}
	t.Fatalf("no room %q in %v", name, roomNames(r))
	return plan.Room{}
}

func TestPartitioned(t *testing.T) {
	r := mustBuild(t, partitioned())
	tr := r.Trace
	if n, m := kindCount(r, exterior), kindCount(r, interior); n != 4 || m != 2 {
		t.Fatalf("%d exterior, %d interior walls, want 4, 2: %+v", n, m, tr.Walls)
	}
	if tr.Outline != nil {
		t.Errorf("a rectangle needs no outline, got %v", tr.Outline)
	}
	if got := roomNames(
		r,
	); !slices.Equal(
		got,
		[]string{"Meeting Room 2", "Open Office", "Kitchen"},
	) {
		t.Errorf("rooms %v, want reading order Meeting Room 2, Open Office, Kitchen", got)
	}
	if rm := roomNamed(
		t,
		r,
		"Kitchen",
	); rm.Zone != "social" || rm.Floor != "tile" || rm.ID != "r3" ||
		*rm.Label != [2]float64{320, 250} {
		t.Errorf("kitchen %+v", rm)
	}
	if rm := roomNamed(
		t,
		r,
		"Meeting Room 2",
	); rm.Zone != "meet" || rm.Floor != "carpet" ||
		rm.ID != "r1" {
		t.Errorf("meeting room %+v", rm)
	}
	if tr.Thickness.Exterior != 0.24 || tr.Thickness.Interior != 0.12 ||
		tr.Thickness.Glass != defaultGlass {
		t.Errorf("thickness %+v", tr.Thickness)
	}
	for _, w := range tr.Walls {
		if w.Thickness != 0 {
			t.Errorf("wall %s has thickness %v; its kind's default fits", w.ID, w.Thickness)
		}
	}
	if tr.Title != "Partitioned" || tr.Calibration.Evidence != "test" || tr.Furniture == nil {
		t.Errorf("header %q %+v", tr.Title, tr.Calibration)
	}
	// No exterior door: the entrance is the middle of the longest exterior wall.
	if tr.Entrance != [2]float64{220, 20} {
		t.Errorf("entrance %v", tr.Entrance)
	}
	if r.Summary != "3 rooms, 6 walls, 0 openings; scale from the drawing's units" {
		t.Errorf("summary %q", r.Summary)
	}
	if len(r.Caveats) != 0 || r.Confidence < 0.9 {
		t.Errorf("caveats %v confidence %v", r.Caveats, r.Confidence)
	}
}

func TestLShapeHasOutline(t *testing.T) {
	r := mustBuild(t, lShape())
	want := [][2]float64{{0, 0}, {400, 0}, {400, 200}, {200, 200}, {200, 400}, {0, 400}}
	got := r.Trace.Outline
	if len(got) != 6 {
		t.Fatalf("outline %v", got)
	}
	for _, p := range want {
		if !slices.Contains(got, p) {
			t.Errorf("outline %v lacks %v", got, p)
		}
	}
	if len(r.Trace.Rooms) != 2 || kindCount(r, exterior) != 6 || kindCount(r, interior) != 1 {
		t.Errorf("rooms %v, walls %+v", roomNames(r), r.Trace.Walls)
	}
	if !slices.Equal(roomNames(r), []string{"Room 1", "Room 2"}) ||
		!strings.Contains(strings.Join(r.Caveats, " "), "2 rooms had no label") {
		t.Errorf("names %v caveats %v", roomNames(r), r.Caveats)
	}
}

func TestSlantedFacade(t *testing.T) {
	r := mustBuild(t, slanted())
	var slants []plan.Wall
	for _, w := range r.Trace.Walls {
		if w.From[0] != w.To[0] && w.From[1] != w.To[1] {
			slants = append(slants, w)
		}
	}
	// The two pieces snap onto one direction and merge; the cross wall
	// meets it in a T without breaking it.
	if len(slants) != 1 || slants[0].Kind != exterior || slants[0].From != [2]float64{300, 0} ||
		slants[0].To != [2]float64{400, 300} {
		t.Fatalf("slanted walls %+v", slants)
	}
	if len(r.Trace.Outline) != 4 || len(r.Trace.Rooms) != 2 {
		t.Errorf("outline %v rooms %v", r.Trace.Outline, roomNames(r))
	}
}

func TestGlassPartition(t *testing.T) {
	r := mustBuild(t, glazed())
	g := wallsOf(r, glass)
	if len(g) != 1 || g[0].From != [2]float64{200, 0} || g[0].To != [2]float64{200, 300} {
		t.Fatalf("glass walls %+v", g)
	}
	// The glass kind's default is its own median; the box walls are 6 px.
	if r.Trace.Thickness.Glass != 0.06 || r.Trace.Thickness.Exterior != 0.12 ||
		r.Trace.Thickness.Interior != defaultInterior {
		t.Errorf("thickness %+v", r.Trace.Thickness)
	}
}

func TestUnclosedFallsBackToHull(t *testing.T) {
	r := mustBuild(t, unclosed())
	if len(r.Trace.Rooms) != 0 || r.Trace.Outline != nil {
		t.Fatalf("rooms %v outline %v", roomNames(r), r.Trace.Outline)
	}
	ext := wallsOf(r, exterior)
	if len(ext) != 3 {
		t.Errorf("exterior %+v, want the three hull walls", ext)
	}
	if !strings.Contains(strings.Join(r.Caveats, " "), "convex hull") || r.Confidence > 0.5 {
		t.Errorf("caveats %v confidence %v", r.Caveats, r.Confidence)
	}
}

// A single wall encloses nothing and its hull is the wall itself.
func TestSingleWall(t *testing.T) {
	r := mustBuild(t, Input{Walls: []Segment{wall(0, 0, 100, 30)}, Scale: scale50, Coverage: -1})
	if len(r.Trace.Walls) != 1 || r.Trace.Walls[0].Kind != exterior {
		t.Fatalf("walls %+v", r.Trace.Walls)
	}
}

// Two crossing walls: the hull touches only their ends, so the longest
// becomes exterior.
func TestCrossFallsBackToLongest(t *testing.T) {
	r := mustBuild(
		t,
		Input{
			Walls:    []Segment{wall(0, 50, 200, 50), wall(100, 0, 100, 100)},
			Scale:    scale50,
			Coverage: -1,
		},
	)
	ext := wallsOf(r, exterior)
	if len(ext) != 1 || ext[0].From != [2]float64{0, 50} {
		t.Fatalf("exterior %+v", ext)
	}
}

func TestCoreBecomesRooms(t *testing.T) {
	r := mustBuild(t, cored())
	names := roomNames(r)
	slices.Sort(names)
	if !slices.Equal(names, []string{"Lift", "Room 1", "Stair"}) {
		t.Fatalf("rooms %v", names)
	}
	// The open floor goes round the core: its area is the floor's less the
	// core's.
	floor := roomNamed(t, r, "Room 1")
	poly := make([]Point, len(floor.Polygon))
	for i, p := range floor.Polygon {
		poly[i] = pp(p)
	}
	if a := math.Abs(signedArea(poly)); a != 600*400-200*100 {
		t.Errorf("open floor area %v px²", a)
	}
	if kindCount(r, exterior) != 4 {
		t.Errorf("walls %+v", r.Trace.Walls)
	}
	if f := roomNamed(t, r, "Lift"); f.Zone != "service" || f.Floor != "carpet" {
		t.Errorf("lift %+v", f)
	}
}

func TestDoors(t *testing.T) {
	r := mustBuild(t, doors())
	type got struct {
		a, b         float64
		swing, hinge string
	}
	ops := map[string]got{}
	for _, w := range r.Trace.Walls {
		for _, o := range w.Openings {
			ops[o.Kind+"@"+w.Kind] = got{o.A, o.B, o.Swing, o.Hinge}
		}
	}
	want := map[string]got{
		"door@exterior":   {100, 145, "S", "a"},
		"window@exterior": {60, 160, "", ""},
	}
	for k, w := range want {
		if ops[k] != w {
			t.Errorf("%s: got %+v, want %+v", k, ops[k], w)
		}
	}
	var interiorDoors []got
	for _, w := range wallsOf(r, interior) {
		for _, o := range w.Openings {
			interiorDoors = append(interiorDoors, got{o.A, o.B, o.Swing, o.Hinge})
		}
	}
	// The vertical door swings east, hinged at the top; the leafless one
	// opens into the smaller room, the meeting room to the north.
	slices.SortFunc(interiorDoors, func(x, y got) int { return cmpFloat(x.a, y.a) })
	if want := []got{{60, 105, "E", "a"}, {280, 325, "N", ""}}; !slices.Equal(interiorDoors, want) {
		t.Errorf("interior doors %+v, want %+v", interiorDoors, want)
	}
	if r.Trace.Entrance != [2]float64{122.5, 20} {
		t.Errorf("entrance %v, want the exterior door", r.Trace.Entrance)
	}
	if !slices.Contains(r.Caveats, "1 door swing guessed.") {
		t.Errorf("caveats %v", r.Caveats)
	}
	if len(r.Trace.Rooms) != 3 {
		t.Errorf("rooms %v: the doors must join the walls back up", roomNames(r))
	}
}

// Swing and hinge on both kinds of wall, both ways round, with the opening's
// ends given either way.
func TestSwingAndHinge(t *testing.T) {
	cases := []struct {
		name         string
		a, b, leaf   Point
		hingeA       bool
		swing, hinge string
	}{
		{"north, hinge west", P(100, 0), P(150, 0), P(110, -20), true, "N", "a"},
		{"north, hinge east, reversed", P(150, 0), P(100, 0), P(110, -20), true, "N", "b"},
		{"south, hinge east", P(100, 0), P(150, 0), P(140, 20), false, "S", "b"},
		{"west of vertical, hinge top", P(0, 100), P(0, 150), P(-20, 110), true, "W", "a"},
		{
			"east of vertical, hinge bottom, reversed",
			P(0, 150),
			P(0, 100),
			P(20, 140),
			true,
			"E",
			"b",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := Input{Walls: box(-200, -200, 300, 300), Scale: scale50, Coverage: -1}
			ax := axis(tc.a, tc.b)
			if ax == 0 {
				in.Walls = append(in.Walls, wall(-200, 0, 300, 0))
			} else {
				in.Walls = append(in.Walls, wall(0, -200, 0, 300))
			}
			in.Openings = []Opening{
				{A: tc.a, B: tc.b, Kind: Door, Leaf: tc.leaf, Leafed: true, HingeA: tc.hingeA},
			}
			r := mustBuild(t, in)
			for _, w := range r.Trace.Walls {
				for _, o := range w.Openings {
					if o.A != 100 || o.B != 150 || o.Swing != tc.swing || o.Hinge != tc.hinge {
						t.Errorf("got %+v, want 100–150 swing %s hinge %s", o, tc.swing, tc.hinge)
					}
					return
				}
			}
			t.Fatal("door lost")
		})
	}
}

// A double door with no leaf in a wall between two equal rooms falls back
// to opening N; on a vertical wall, E.
func TestGuessedSwingDefaults(t *testing.T) {
	for _, tc := range []struct {
		in    Input
		swing string
	}{
		{Input{Walls: []Segment{wall(0, 0, 300, 0), wall(0, 200, 300, 200)}, Openings: []Opening{{A: P(100, 0), B: P(190, 0), Kind: DoubleDoor}}}, "N"},
		{Input{Walls: []Segment{wall(0, 0, 0, 300), wall(200, 0, 200, 300)}, Openings: []Opening{{A: P(0, 100), B: P(0, 190), Kind: DoubleDoor}}}, "E"},
	} {
		tc.in.Scale, tc.in.Coverage = scale50, -1
		r := mustBuild(t, tc.in)
		if o := r.Trace.Walls[0].Openings; len(o) != 1 || o[0].Swing != tc.swing ||
			o[0].Hinge != "" ||
			o[0].A != 100 {
			t.Errorf("openings %+v, want swing %s", o, tc.swing)
		}
	}
}

func TestOpeningsAtWallEndsAndAstray(t *testing.T) {
	in := Input{
		Walls: append(box(0, 0, 400, 300), wall(200, 0, 200, 250)),
		Openings: []Opening{
			// At the free end of the partition: the partition is stretched
			// over it, to meet the south wall.
			{A: P(200, 250), B: P(200, 295), Kind: Gap},
			// Nowhere near a wall.
			{A: P(50, 150), B: P(90, 150), Kind: Door},
			// Unusable.
			{A: P(math.NaN(), 0), B: P(1, 1), Kind: Door},
			{A: P(10, 0), B: P(10.5, 0), Kind: Door},
			// Unknown kind, on the north wall.
			{A: P(50, 0), B: P(90, 0), Kind: "portal"},
			// Overlapping the one before.
			{A: P(60, 0), B: P(100, 0), Kind: Window},
			// Under 10 cm.
			{A: P(150, 300), B: P(153, 300), Kind: Window},
		},
		Scale:    scale50,
		Coverage: -1,
	}
	r := mustBuild(t, in)
	if len(r.Trace.Rooms) != 2 {
		t.Errorf("rooms %v: the gap should close the partition", roomNames(r))
	}
	kinds := openingKinds(r)
	if kinds[Gap] != 2 || kinds[Window] != 0 || kinds[Door] != 0 {
		t.Errorf("openings %v", kinds)
	}
	all := strings.Join(r.Caveats, " | ")
	for _, want := range []string{
		"1 opening matched no wall", "2 openings had unusable coordinates", "1 opening of unknown kind",
		"1 opening overlapped another", "1 opening was under 10 cm wide",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("caveats %q lack %q", all, want)
		}
	}
}

func TestDimensionLabelsSkipped(t *testing.T) {
	in := Input{Walls: box(0, 0, 400, 300), Scale: scale50, Coverage: -1}
	for i, s := range []string{`12'-3"`, "3600", "12.5 m²", "4.2 x 3.1", "3.60", "Area 20 sq ft", "12m2"} {
		in.Labels = append(in.Labels, Label{At: P(50+float64(i)*20, 100), Text: s, Height: 40})
	}
	in.Labels = append(in.Labels,
		Label{At: P(200, 150), Text: "Office", Height: 12},
		Label{At: P(200, 170), Text: "Desk A", Height: 12},
		Label{At: P(200, 190), Text: "note", Height: math.NaN()},
		Label{At: P(500, 500), Text: "Outside", Height: 50},
		Label{At: P(200, 210), Text: "   ", Height: 50},
	)
	r := mustBuild(t, in)
	if got := roomNames(r); !slices.Equal(got, []string{"Office"}) {
		t.Errorf("names %v", got)
	}
	if !slices.Contains(r.Caveats, "1 label fell outside every room.") {
		t.Errorf("caveats %v", r.Caveats)
	}
}

func TestLooksLikeDimension(t *testing.T) {
	for s, want := range map[string]bool{
		`12'-3"`: true, "3600": true, "12.5 m²": true, "4.2 x 3.1": true, `12'-3" x 18'-11"`: true,
		"Office 06": false, "x": false, "WC": false, "Room": false, "101": true, "2F": false,
	} {
		if got := looksLikeDimension(s); got != want {
			t.Errorf("looksLikeDimension(%q) = %v", s, got)
		}
	}
}

func TestClassify(t *testing.T) {
	for name, want := range map[string][2]string{
		"Men's WC":                {"service", "tile"},
		"Lifts":                   {"service", "carpet"},
		"Boardroom":               {"meet", "carpet"},
		"Conference / Lab":        {"meet", "carpet"},
		"Reception":               {"social", "wood"},
		"Coffee, Kitchen & Stuff": {"social", "tile"},
		"Open office & Break":     {"social", "wood"},
		"Office 06":               {"work", "carpet"},
		"Restrooms":               {"service", "tile"},
		"Hallway":                 {"service", "carpet"},
		"Meeting room store":      {"service", "carpet"},
		"Café":                    {"social", "wood"},
	} {
		if z, f := classify(name); z != want[0] || f != want[1] {
			t.Errorf("classify(%q) = %s, %s; want %v", name, z, f, want)
		}
	}
}

func TestLabelTieBreaks(t *testing.T) {
	in := Input{Walls: box(0, 0, 400, 300), Scale: scale50, Coverage: -1}
	in.Labels = []Label{
		{At: P(100, 100), Text: "Bb", Height: 10},
		{At: P(100, 100), Text: "Aa", Height: 10},
		{At: P(50, 100), Text: "Cc", Height: 10},
		{At: P(50, 50), Text: "Dd", Height: 10},
		{At: P(60, 60), Text: "Ee", Height: 10},
	}
	if got := roomNames(mustBuild(t, in)); got[0] != "Dd" {
		t.Errorf("topmost of equals should win, got %v", got)
	}
	in.Labels = append(in.Labels, Label{At: P(300, 200), Text: "Longer", Height: 10})
	if got := roomNames(mustBuild(t, in)); got[0] != "Longer" {
		t.Errorf("longest should win, got %v", got)
	}
	in.Labels = in.Labels[:2]
	if got := roomNames(mustBuild(t, in)); got[0] != "Aa" {
		t.Errorf("alphabetical last resort, got %v", got)
	}
}

func TestScaleEstimates(t *testing.T) {
	base := func() Input {
		return Input{Walls: append(box(0, 0, 500, 400), wall(250, 0, 250, 400)), Coverage: -1}
	}
	door := func(x0, x1 float64, kind string) Opening {
		return Opening{A: P(x0, 0), B: P(x1, 0), Kind: kind, Leafed: true, Leaf: P(x0, 10)}
	}
	cases := []struct {
		name     string
		mod      func(*Input)
		px       float64
		evidence string
		source   int
	}{
		{"doors", func(in *Input) {
			in.Openings = []Opening{
				door(20, 65, Door),
				door(100, 190, DoubleDoor),
				door(300, 345, Door),
			}
		}, 52.326, "assumed from 3 door widths (0.86 m per leaf)", scaleDoors},
		{"doorways", func(in *Input) {
			in.Openings = []Opening{door(20, 65, Gap), door(100, 145, Gap), door(300, 460, Gap)}
		}, 52.326, "assumed from 2 doorway widths (0.86 m each)", scaleGaps},
		{"thickness", func(in *Input) {
			in.Walls[4].Thickness = 7.5
		}, 50, "assumed from the median interior wall thickness (0.15 m)", scaleThickness},
		{"guess", func(in *Input) {
			for i := range in.Walls {
				in.Walls[i].Thickness = 0
			}
		}, 25, "guessed: the walls taken to span 20 m", scaleGuess},
		{"measured", func(in *Input) {
			in.Scale = Scale{PxPerMetre: [2]float64{40, 44}}
		}, 40, "measured by the front end", scaleMeasured},
		{"exact without evidence", func(in *Input) {
			in.Scale = Scale{PxPerMetre: [2]float64{40, 40}, Exact: true}
		}, 40, "the drawing's units", scaleExact},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := base()
			tc.mod(&in)
			r := mustBuild(t, in)
			c := r.Trace.Calibration
			if c.PxPerMetre[0] != tc.px || c.Evidence != tc.evidence {
				t.Fatalf("calibration %+v, want %v from %q", c, tc.px, tc.evidence)
			}
			assumed := strings.Contains(
				strings.Join(r.Caveats, " "),
				"check it against a known dimension",
			)
			if assumed != (tc.source >= scaleDoors) {
				t.Errorf("caveats %v", r.Caveats)
			}
			if tc.source >= scaleDoors && !strings.Contains(r.Summary, "scale "+tc.evidence) {
				t.Errorf("summary %q", r.Summary)
			}
		})
	}
}

// Walls that stop short of a corner, or run past it, are closed up: the
// building still has its room.
func TestCornersClose(t *testing.T) {
	in := Input{
		Walls: []Segment{
			wall(4, 0, 396, 0),      // short of both corners
			wall(400, -5, 400, 300), // past the top corner
			wall(400, 300, 0, 300),
			wall(0, 297, 0, 3),
			wall(200, 4, 200, 296),   // a partition short of both walls
			wall(300, 150, 305, 150), // a stub of less than its thickness: a column
		},
		Scale:    scale50,
		Coverage: -1,
	}
	r := mustBuild(t, in)
	if len(r.Trace.Rooms) != 2 {
		t.Fatalf("rooms %v walls %+v", roomNames(r), r.Trace.Walls)
	}
	for _, w := range wallsOf(r, exterior) {
		for _, p := range [][2]float64{w.From, w.To} {
			if (p[0] != 0 && p[0] != 400) || (p[1] != 0 && p[1] != 300) {
				t.Errorf("exterior wall %s end %v not at a corner", w.ID, p)
			}
		}
	}
}

// Pieces of one wall found separately, overlapping or with small gaps and a
// little off line, come out as one wall.
func TestCollinearMerge(t *testing.T) {
	in := Input{
		Walls: append(box(0, 0, 400, 300),
			wall(200, 0, 200, 100), wall(201, 98, 201, 200), wall(200, 203, 200, 300),
			Segment{A: P(199.5, 50), B: P(199.5, 150), Thickness: 6, Weight: 0.5, Glass: true}),
		Scale:    scale50,
		Coverage: -1,
	}
	r := mustBuild(t, in)
	if n := kindCount(r, interior); n != 1 || kindCount(r, glass) != 0 {
		t.Fatalf("walls %+v", r.Trace.Walls)
	}
}

// A wall drawn a degree off vertical is snapped about its middle.
func TestSnapNearAxis(t *testing.T) {
	in := Input{
		Walls:    append(box(0, 0, 400, 300), wall(197, 0, 203, 300)),
		Scale:    scale50,
		Coverage: -1,
	}
	r := mustBuild(t, in)
	w := wallsOf(r, interior)
	if len(w) != 1 || w[0].From != [2]float64{200, 0} || w[0].To != [2]float64{200, 300} {
		t.Fatalf("interior %+v", w)
	}
}

func TestNoWalls(t *testing.T) {
	for name, in := range map[string]Input{
		"empty":    {},
		"unusable": {Walls: []Segment{{A: P(math.Inf(1), 0), B: P(1, 1)}, {A: P(2e6, 0), B: P(1, 1)}}},
		"points":   {Walls: []Segment{wall(5, 5, 5, 5), wall(10, 10, 10.01, 10)}},
		"tiny":     {Walls: []Segment{wall(0, 0, 1, 0)}, Scale: scale50},
		"merged":   {Walls: []Segment{wall(0, 0, 0.3, 0.3)}, Scale: scale50},
	} {
		if _, err := Build(in); !errors.Is(err, ErrNoWalls) {
			t.Errorf("%s: err %v, want ErrNoWalls", name, err)
		}
	}
	// Walls on one line enclose no floor: the site's validator can't place
	// them inside a building of no depth.
	for _, in := range []Input{
		{Walls: []Segment{wall(0, 0, 300, 0), wall(400, 0, 500, 0)}, Scale: scale50},
		{Walls: []Segment{wall(0, 0, 0, 300), wall(5, 0, 5, 300)}, Scale: scale50},
	} {
		if _, err := Build(in); !errors.Is(err, ErrFlat) {
			t.Errorf("walls %v: err %v, want ErrFlat", in.Walls, err)
		}
	}
}

func TestPassThrough(t *testing.T) {
	w, d := 1.6, 0.8
	in := partitioned()
	in.Title = "  "
	in.Furniture = []plan.Item{{Type: "desk", X: 100, Y: 100, W: &w, D: &d}}
	in.Notes = []string{"front end note", " "}
	in.Coverage = 0.5
	in.Walls = append(in.Walls, Segment{A: P(math.NaN(), 0), B: P(0, 0)})
	in.Walls[0].Weight = math.NaN()
	in.Walls[1].Thickness = math.Inf(1)
	r := mustBuild(t, in)
	if r.Trace.Title != "Floor plan" || len(r.Trace.Furniture) != 1 ||
		r.Trace.Furniture[0].W != &w {
		t.Errorf("title %q furniture %+v", r.Trace.Title, r.Trace.Furniture)
	}
	if r.Caveats[0] != "front end note" ||
		!slices.Contains(r.Caveats, "1 wall piece had unusable coordinates and was ignored.") {
		t.Errorf("caveats %v", r.Caveats)
	}
	if full := mustBuild(t, partitioned()); r.Confidence >= full.Confidence {
		t.Errorf(
			"confidence %v with half coverage and a weightless wall, %v without",
			r.Confidence,
			full.Confidence,
		)
	}
}

// A wall much thicker than its kind's default carries its own thickness.
func TestThicknessOverride(t *testing.T) {
	in := partitioned()
	in.Walls[5].Thickness = 15
	r := mustBuild(t, in)
	n := 0
	for _, w := range r.Trace.Walls {
		if w.Thickness != 0 {
			n++
			if w.Thickness != 0.3 {
				t.Errorf("wall %s thickness %v", w.ID, w.Thickness)
			}
		}
	}
	if n != 1 {
		t.Errorf("%d walls with their own thickness, want 1", n)
	}
}

// A wall sticking out of the building leaves its end outside the outline,
// so an L-shaped outline can't be given.
func TestOutlineDroppedForStrayWall(t *testing.T) {
	in := lShape()
	in.Walls = append(in.Walls, wall(400, 100, 500, 100), wall(300, 300, 350, 350))
	r := mustBuild(t, in)
	if r.Trace.Outline != nil ||
		!slices.Contains(
			r.Caveats,
			"Some walls run outside the building's outline, so the outline was left out.",
		) {
		t.Errorf("outline %v caveats %v", r.Trace.Outline, r.Caveats)
	}
}

// A second building apart from the first isn't made into rooms.
func TestSeparateBuilding(t *testing.T) {
	in := Input{
		Walls:    append(box(0, 0, 400, 300), box(500, 0, 600, 100)...),
		Scale:    scale50,
		Coverage: -1,
	}
	r := mustBuild(t, in)
	if len(r.Trace.Rooms) != 1 ||
		!slices.Contains(
			r.Caveats,
			"1 area enclosed by walls apart from the building was not made a room.",
		) {
		t.Errorf("rooms %v caveats %v", roomNames(r), r.Caveats)
	}
}

// Slivers between double wall lines and rooms under 0.5 m² aren't rooms.
func TestSliversAreNotRooms(t *testing.T) {
	in := Input{
		Walls: append(
			box(0, 0, 400, 300),
			wall(0, 10, 400, 10),
			wall(370, 270, 400, 270),
			wall(370, 270, 370, 300),
		),
		Scale:    scale50,
		Coverage: -1,
	}
	r := mustBuild(t, in)
	if len(r.Trace.Rooms) != 1 {
		t.Errorf("rooms %v", roomNames(r))
	}
}
