package dxftest

import "fmt"

// Arc is an ARC; angles are degrees, as DXF stores them.
func Arc(layer string, cx, cy, r, a0, a1 float64) Ent {
	return E("ARC", 8, layer, 10, cx, 20, cy, 40, r, 50, a0, 51, a1)
}

// Insert is an INSERT of block at (x, y), turned rot degrees; extra pairs
// (scale 41/42, array 70/71/44/45, extrusion 230, ...) follow.
func Insert(layer, block string, x, y, rot float64, extra ...any) Ent {
	p := append([]any{8, layer, 2, block, 10, x, 20, y, 50, rot}, extra...)
	return E("INSERT", p...)
}

// Text is a left, baseline-aligned TEXT; extra pairs follow.
func Text(layer string, x, y, h float64, s string, extra ...any) Ent {
	p := append([]any{8, layer, 10, x, 20, y, 40, h, 1, s}, extra...)
	return E("TEXT", p...)
}

// faces draws a straight wall from a to b (centreline, along x when
// horizontal, else along y) as its two faces, thick apart, broken at the
// given gaps along it. Fixture walls are axis-aligned, which keeps their
// expected positions easy to read in tests.
func faces(layer string, horizontal bool, at, from, to, thick float64, gaps ...[2]float64) []Ent {
	var out []Ent
	for _, side := range []float64{-thick / 2, thick / 2} {
		start := from
		runs := append(append([][2]float64{}, gaps...), [2]float64{to, to})
		for _, g := range runs {
			if g[0] > start {
				if horizontal {
					out = append(out, Line(layer, start, at+side, g[0], at+side))
				} else {
					out = append(out, Line(layer, at+side, start, at+side, g[0]))
				}
			}
			start = g[1]
		}
	}
	return out
}

// The office's rooms, by name, with the corners of their wall centrelines
// in millimetres. See Office.
var OfficeRooms = map[string][4]float64{
	"Open office": {0, 0, 6000, 4000},
	"Meeting":     {0, 4000, 6000, 8000},
	"Kitchen":     {6000, 0, 12000, 4000},
	"Director":    {6000, 4000, 12000, 8000},
}

// Office is a four-room office drawn the way architects draw: walls as
// pairs of lines on A-WALL (200 mm outside, 100 mm partitions) broken at
// their openings, a glazed partition on A-GLAZ, a hatch in one wall, and
// doors, windows, furniture and room names as blocks and text.
//
//   - Doors (A-DOOR, block DOOR900, leaf open along the block's +y): the
//     entrance at x 2000–2900 in the south wall, hinged at 2000; one in the
//     x=6000 partition at y 2000–2900, turned −90° and hinged at 2900; one in
//     the glazed partition at x 4000–4900 by a mirrored insert (extrusion
//     −Z), hinged at 4900; and a double door (DOOR_DBL) at x 8000–9800 in
//     the y=4000 partition.
//   - Windows (A-GLAZ-WIND, WIN1200): x 1500–2700 and 7500–8700 in the north
//     wall, and y 1500–2700 in the east wall (turned 90°).
//   - Furniture (A-FURN): a 3 × 2 MINSERT of WORKSTATION blocks, each a
//     nested DESK and CHAIR; a round table with four chairs; a sink and a
//     counter; a fridge (no plan type); and a desk, mirrored chair, sofa and
//     plant. CHAIR's back is at its +y, so it faces -y; every chair is
//     turned to face its desk or table.
//   - Labels: MTEXT "Open office", TEXT "Meeting", a room-tag attribute
//     "Kitchen", MTEXT "Director" over its area. The entrance door has a
//     tag attribute and the drawing a dimension, neither of them a label.
//
// units is $INSUNITS (4 is millimetres; -1 leaves it out).
func Office(units int) *Doc {
	const part = 100.0
	door := []Ent{Line("0", 0, 0, 0, 900), Arc("0", 0, 0, 900, 0, 90)}
	double := []Ent{
		Line("0", 0, 0, 0, 900), Arc("0", 0, 0, 900, 0, 90),
		Line("0", 1800, 0, 1800, 900), Arc("0", 1800, 0, 900, 90, 180),
	}
	window := []Ent{Rect("0", -600, -100, 600, 100), Line("0", -600, 0, 600, 0)}
	desk := []Ent{Rect("0", 0, 0, 1600, 800)}
	chair := []Ent{Rect("0", -250, -250, 250, 250), Line("0", -250, 200, 250, 200)}
	station := []Ent{Insert("0", "DESK", 0, 0, 0), Insert("0", "CHAIR", 800, -400, 180)}
	table := []Ent{E("CIRCLE", 8, "0", 10, 0.0, 20, 0.0, 40, 600.0)}
	dim := []Ent{Line("0", 0, -1000, 12000, -1000), Text("0", 6000, -900, 200, "12000")}

	d := New(units).
		Layer("A-WALL", 7, 0).Layer("A-WALL-PATT", 8, 0).Layer("A-GLAZ", 4, 0).
		Layer("A-DOOR", 2, 0).Layer("A-GLAZ-WIND", 4, 0).Layer("A-FURN", 3, 0).
		Layer("A-ANNO", 7, 0).Layer("A-ANNO-DIMS", 1, 0).
		Block("DOOR900", 0, 0, door...).Block("DOOR_DBL", 0, 0, double...).
		Block("WIN1200", 0, 0, window...).
		Block("DESK", 0, 0, desk...).Block("CHAIR", 0, 0, chair...).
		Block("WORKSTATION", 0, 0, station...).Block("TABLE_ROUND", 0, 0, table...).
		Block("SINK", 0, 0, Rect("0", -400, -250, 400, 250)).
		Block("KITCHEN_COUNTER", 0, 0, Rect("0", 0, 0, 2400, 600)).
		Block("FRIDGE", 0, 0, Rect("0", 0, 0, 600, 600)).
		Block("SOFA", 0, 0, Rect("0", -1000, -400, 1000, 400)).
		Block("PLANT", 0, 0, E("CIRCLE", 8, "0", 10, 0.0, 20, 0.0, 40, 300.0)).
		Block("RMTAG", 0, 0, Rect("0", -800, -300, 800, 300)).
		Block("*D1", 0, 0, dim...)

	// Exterior walls: south (door), north (two windows), west, east
	// (window), each face broken where a partition meets it.
	d.Add(Line("A-WALL", -100, -100, 2000, -100), Line("A-WALL", 2900, -100, 12100, -100))
	d.Add(Line("A-WALL", 100, 100, 2000, 100), Line("A-WALL", 2900, 100, 5950, 100),
		Line("A-WALL", 6050, 100, 11900, 100))
	d.Add(Line("A-WALL", -100, 8100, 1500, 8100), Line("A-WALL", 2700, 8100, 7500, 8100),
		Line("A-WALL", 8700, 8100, 12100, 8100))
	d.Add(Line("A-WALL", 100, 7900, 1500, 7900), Line("A-WALL", 2700, 7900, 5950, 7900),
		Line("A-WALL", 6050, 7900, 7500, 7900), Line("A-WALL", 8700, 7900, 11900, 7900))
	d.Add(Line("A-WALL", -100, -100, -100, 8100), Line("A-WALL", 100, 100, 100, 7900))
	d.Add(Line("A-WALL", 12100, -100, 12100, 1500), Line("A-WALL", 12100, 2700, 12100, 8100))
	d.Add(Line("A-WALL", 11900, 100, 11900, 1500), Line("A-WALL", 11900, 2700, 11900, 3950),
		Line("A-WALL", 11900, 4050, 11900, 7900))
	// Partitions: x = 6000 the whole depth (door at y 2000–2900), y = 4000
	// east of it (double door at x 8000–9800).
	d.Add(Line("A-WALL", 5950, 100, 5950, 2000), Line("A-WALL", 5950, 2900, 5950, 7900))
	d.Add(Line("A-WALL", 6050, 100, 6050, 2000), Line("A-WALL", 6050, 2900, 6050, 3950),
		Line("A-WALL", 6050, 4050, 6050, 7900))
	d.Add(faces("A-WALL", true, 4000, 6050, 11900, part, [2]float64{8000, 9800})...)
	// The glazed partition, y = 4000 west of the partition, 20 mm panes.
	d.Add(faces("A-GLAZ", true, 4000, 100, 5950, 20, [2]float64{4000, 4900})...)
	// Poché in the west wall.
	d.Add(E(
		"HATCH",
		8,
		"A-WALL-PATT",
		2,
		"SOLID",
		70,
		1,
		91,
		1,
		92,
		2,
		72,
		0,
		73,
		1,
		93,
		4,
		10,
		-100.0,
		20,
		-100.0,
		10,
		100.0,
		20,
		-100.0,
		10,
		100.0,
		20,
		8100.0,
		10,
		-100.0,
		20,
		8100.0,
		97,
		0,
	))

	d.Add(
		Insert("A-DOOR", "DOOR900", 2000, 0, 0, 66, 1),
		E("ATTRIB", 8, "A-ANNO", 10, 2400.0, 20, -400.0, 40, 150.0, 1, "D01", 2, "TAG", 70, 0),
		E("SEQEND", 8, "A-DOOR"),
		Insert("A-DOOR", "DOOR900", 6000, 2900, -90),
		Insert("A-DOOR", "DOOR900", -4900, 4000, 0, 230, -1.0),
		Insert("A-DOOR", "DOOR_DBL", 8000, 4000, 0),
		Insert("A-GLAZ-WIND", "WIN1200", 2100, 8000, 0),
		Insert("A-GLAZ-WIND", "WIN1200", 8100, 8000, 0),
		Insert("A-GLAZ-WIND", "WIN1200", 12000, 2100, 90),
	)
	d.Add(
		Insert("A-FURN", "WORKSTATION", 600, 1000, 0, 70, 3, 71, 2, 44, 1800.0, 45, 1600.0),
		Insert("A-FURN", "TABLE_ROUND", 3000, 6000, 0),
		Insert("A-FURN", "CHAIR", 3000, 5100, 180),
		Insert("A-FURN", "CHAIR", 3900, 6000, 270),
		Insert("A-FURN", "CHAIR", 3000, 6900, 0),
		Insert("A-FURN", "CHAIR", 2100, 6000, 90),
		Insert("A-FURN", "SINK", 11500, 3000, 90),
		Insert("A-FURN", "KITCHEN_COUNTER", 11800, 300, 90),
		Insert("A-FURN", "FRIDGE", 6300, 300, 0),
		Insert("A-FURN", "DESK", 7000, 5500, 90),
		Insert("A-FURN", "CHAIR", 7900, 6300, 270, 41, -1.0),
		Insert("A-FURN", "SOFA", 10500, 7300, 180),
		Insert("A-FURN", "PLANT", 11500, 4500, 0),
	)
	d.Add(
		E("MTEXT", 8, "A-ANNO", 10, 3000.0, 20, 2600.0, 40, 250.0, 71, 5, 1, "Open office"),
		Text("A-ANNO", 1000, 7000, 250, "Meeting"),
		Insert("A-ANNO", "RMTAG", 9000, 2000, 0, 66, 1),
		E("ATTRIB", 8, "A-ANNO", 10, 9000.0, 20, 2000.0, 11, 9000.0, 21, 2000.0, 40, 250.0,
			72, 4, 1, "Kitchen", 2, "NAME", 70, 0),
		E("SEQEND", 8, "A-ANNO"),
		E(
			"MTEXT",
			8,
			"A-ANNO",
			10,
			9000.0,
			20,
			6000.0,
			40,
			250.0,
			71,
			5,
			1,
			`{\fArial|b1;Director}\P24 m²`,
		),
		E("DIMENSION", 8, "A-ANNO-DIMS", 2, "*D1"),
	)
	return d
}

// Unlayered is a two-room plan that names nothing: walls, doors and text
// all on layer 0 or numbered layers, doors as anonymous blocks recognised
// only by their swings. Rooms "West" and "East" are 0–5000 and
// 5000–10000 across, 0–6000 up, in millimetres, with 200 mm walls and a
// door at x 5000, y 1000–1900.
func Unlayered(units int) *Doc {
	door := []Ent{Line("0", 0, 0, 0, 900), Arc("0", 0, 0, 900, 0, 90), Line("0", -50, 0, 950, 0)}
	d := New(units).Layer("L1", 7, 0).Layer("L2", 7, 0).Block("*U3", 0, 0, door...)
	d.Add(faces("L1", true, 0, -100, 10100, 200)...)
	d.Add(faces("L1", true, 6000, -100, 10100, 200)...)
	d.Add(faces("L1", false, 0, -100, 6100, 200)...)
	d.Add(faces("L1", false, 10000, -100, 6100, 200)...)
	d.Add(faces("L1", false, 5000, 100, 5900, 200, [2]float64{1000, 1900})...)
	d.Add(
		Insert("0", "*U3", 5000, 1900, -90),
		Text("L2", 2000, 3000, 250, "West"),
		Text("L2", 7000, 3000, 250, "East"),
	)
	return d
}

// Grid is an nx × ny grid of 4 m rooms with 200 mm walls in millimetres,
// each room with a door in its south wall and a name, every wall face
// broken where another wall crosses it: about 6·nx·ny lines, for timing.
func Grid(nx, ny int) *Doc {
	d := New(4).Layer("A-WALL", 7, 0).Layer("A-ANNO", 7, 0).Layer("A-DOOR", 2, 0).
		Block("DOOR", 0, 0, Line("0", 0, 0, 0, 900), Arc("0", 0, 0, 900, 0, 90))
	const room = 4000.0
	crossings := func(n int, door bool) [][2]float64 {
		var gaps [][2]float64
		for i := range n {
			if i > 0 {
				x := float64(i) * room
				gaps = append(gaps, [2]float64{x - 100, x + 100})
			}
			if door {
				x := float64(i)*room + 1000
				gaps = append(gaps, [2]float64{x, x + 900})
			}
		}
		return gaps
	}
	for j := 0; j <= ny; j++ {
		d.Add(
			faces(
				"A-WALL",
				true,
				float64(j)*room,
				-100,
				float64(nx)*room+100,
				200,
				crossings(nx, j < ny)...)...)
	}
	for i := 0; i <= nx; i++ {
		d.Add(
			faces(
				"A-WALL",
				false,
				float64(i)*room,
				-100,
				float64(ny)*room+100,
				200,
				crossings(ny, false)...)...)
	}
	for j := range ny {
		for i := range nx {
			x, y := float64(i)*room, float64(j)*room
			d.Add(
				Insert("A-DOOR", "DOOR", x+1000, y, 0),
				Text("A-ANNO", x+1500, y+2000, 250, fmt.Sprintf("Room %d-%d", i, j)),
			)
		}
	}
	return d
}
