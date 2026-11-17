package cadfront

import (
	"slices"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
	"github.com/weaveplatform/spaceweave/internal/floorgraph"
)

// These fixtures imitate how drawings letter their rooms and mark their
// voids, not any drawing: room names and numbers on their own layers
// (Uniclass Zz_20_10_70_65, AIA A-AREA-IDEN and A-ANNO-ROOM), equipment and
// furniture tagged beside them, and an atrium drawn as loose lines on the
// stairs layer with its name inside.

func TestTextLayers(t *testing.T) {
	for name, want := range map[string]textClass{
		"AR_Zz_20_10_70_65-T_Room Name":    textName,
		"AR_Zz_20_10_70_65-T_Room Number":  textNumber,
		"A-AREA-IDEN":                      textName,
		"A-ANNO-ROOM":                      textName,
		"ROOM-TEXT":                        textName,
		"ROOMNAME":                         textName,
		"RM_NO":                            textNumber,
		"RAUMNUMMER":                       textNumber,
		"A-AREA-NUMB":                      textNumber,
		"AR_EF_40_40-M_Equipment_IT":       textNever,
		"AR_Ss_40_15-X_Existing Furniture": textNever,
		"A-FURN-TEXT":                      textNever,
		"Desk Numbers":                     textNever,
		"A-ANNO-NOTE":                      textNever,
		"AR_Pr_40_20-X_Sanitary Fitting":   textNever,
		"EC1 Space Type Descs":             textAny,
		"0":                                textAny,
		"A-ANNO-TEXT":                      textAny,
	} {
		if got := textLayer(name); got != want {
			t.Errorf("%q: %d, want %d", name, got, want)
		}
	}
}

func TestLabelKind(t *testing.T) {
	for _, tc := range []struct {
		layer, text string
		named       bool
		kind        floorgraph.LabelKind
		ok          bool
	}{
		{"A-ANNO-ROOM", "Kitchen", false, floorgraph.LabelName, true},
		{"RM_NO", "1.01", true, floorgraph.LabelNumber, true},
		{"A-ANNO-ROOM", `85" TV`, false, 0, false},
		{"0", "1200x800 desk", false, 0, false},
		{"A-FURN", "Fridge", false, 0, false},
		{"0", "Meeting Room", false, floorgraph.LabelText, true},
		{"0", "Room 12-34", false, floorgraph.LabelText, true},
		{"0", "Meeting Room", true, 0, false}, // the drawing names its rooms elsewhere
		{"0", "4C-117", false, 0, false},      // a desk number
		{"0", "A", false, 0, false},
	} {
		kind, ok := labelKind(tc.layer, tc.text, tc.named)
		if kind != tc.kind || ok != tc.ok {
			t.Errorf("%q on %q (named %v): %v %v, want %v %v", tc.text, tc.layer, tc.named, kind, ok, tc.kind, tc.ok)
		}
	}
}

func TestVoidNames(t *testing.T) {
	for text, want := range map[string]bool{
		"ATRIUM": true, "Atrium 2": true, "VOID": true, "VOID OVER RECEPTION": true,
		"Open to below": true, "LIGHT WELL": true, "Lightwell B": true, "Luftraum": true,
		"ATRIUM CAFÉ": false, "Open to above": false, "Avoid": false, "Kitchen": false,
	} {
		if got := voidName.MatchString(text); got != want {
			t.Errorf("%q: %v, want %v", text, got, want)
		}
	}
}

// twoRooms is a 12 × 8 m box split in two, in millimetres.
func twoRooms() *dxftest.Doc {
	const wall = "A-WALL"
	d := dxftest.New(4).Layer(wall, 7, 0)
	d.Add(dxftest.Rect(wall, 0, 0, 12000, 8000), dxftest.Line(wall, 6000, 0, 6000, 8000))
	return d
}

// TestRoomNameText: on a drawing with a room-name layer only that and a
// room-number layer name rooms; furniture, equipment, desk numbers, notes
// and product sizes never do.
func TestRoomNameText(t *testing.T) {
	d := twoRooms()
	d.Add(
		dxftest.Text("A-ANNO-ROOM", 1000, 4000, 300, "Kitchen"),
		dxftest.Text("A-ANNO-ROOM", 1000, 2000, 500, `85" TV`),
		dxftest.Text("AR_EF_40_40-M_Equipment_IT", 1000, 6000, 600, "Fridge"),
		dxftest.Text("A-AREA-NUMB", 7000, 4000, 200, "R1-02"),
		dxftest.Text("Desk Numbers", 7000, 2000, 400, "Desk 12"),
		dxftest.Text("0", 7000, 6000, 400, "Notes here"),
	)
	in, _ := read(t, d)
	got := map[string]floorgraph.LabelKind{}
	for _, l := range in.Labels {
		got[l.Text] = l.Kind
	}
	if len(got) != 2 || got["Kitchen"] != floorgraph.LabelName || got["R1-02"] != floorgraph.LabelNumber {
		t.Errorf("labels %+v", in.Labels)
	}
	r, err := floorgraph.Build(in)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, rm := range r.Trace.Rooms {
		names = append(names, rm.Name)
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"Kitchen", "R1-02"}) {
		t.Errorf("rooms %v", names)
	}
}

// TestUnlayeredRoomText: without a room-name layer, text that looks like a
// name names a room, and codes and sizes don't.
func TestUnlayeredRoomText(t *testing.T) {
	d := twoRooms()
	d.Add(
		dxftest.Text("0", 1000, 4000, 300, "Meeting Room"),
		dxftest.Text("0", 1000, 2000, 500, "1200x800"),
		dxftest.Text("0", 7000, 2000, 500, "4C-117"),
	)
	in, _ := read(t, d)
	if len(in.Labels) != 1 || in.Labels[0].Text != "Meeting Room" || in.Labels[0].Kind != floorgraph.LabelText {
		t.Errorf("labels %+v", in.Labels)
	}
}

// atriumPlan is a 24 × 18 m floor with a 6 × 5 m atrium drawn on the stairs
// layer as a slab edge (lines that stop 30 mm short of each other) and a
// balustrade 200 mm inside (an open polyline), with a lift shaft in it.
func atriumPlan() *dxftest.Doc {
	const wall, stair, names = "A-WALL", "AR_EF_35_10-M_Stairs and Ramps", "AR_Zz_20_10_70_65-T_Room Name"
	d := dxftest.New(4).Layer(wall, 7, 0).Layer(stair, 3, 0).Layer(names, 2, 0)
	d.Add(dxftest.Rect(wall, 0, 0, 24000, 18000), dxftest.Rect(wall, 10500, 7500, 12000, 8700))
	d.Add(
		dxftest.Line(stair, 9000, 6000, 15000, 6000), dxftest.Line(stair, 15000, 6030, 15000, 11000),
		dxftest.Line(stair, 15000, 11000, 9000, 11000), dxftest.Line(stair, 9000, 11000, 9000, 6030),
		dxftest.E("LWPOLYLINE", 8, stair, 90, 5, 70, 0,
			10, 9200.0, 20, 6200.0, 10, 14800.0, 20, 6200.0, 10, 14800.0, 20, 10800.0,
			10, 9200.0, 20, 10800.0, 10, 9200.0, 20, 6250.0),
		dxftest.Text(names, 12000, 10000, 300, "ATRIUM"),
		dxftest.Text(names, 3000, 3000, 300, "Office"),
		// A curved stair's string and a newel post's round: an edge, and too
		// small to be one. A dimension is none.
		dxftest.Arc(stair, 20000, 14000, 1500, 180, 270), dxftest.Arc(stair, 20000, 4000, 100, 0, 90),
		dxftest.Line("A-DIMS", 1000, 17000, 23000, 17000),
		dxftest.Rect(stair, 18000, 2000, 19000, 3000),
	)
	return d
}

func TestAtrium(t *testing.T) {
	in, o := read(t, atriumPlan())
	if len(in.Voids) != 1 || len(in.Edges) < 8 {
		t.Fatalf("voids %v, %d edges", in.Voids, len(in.Edges))
	}
	r, err := floorgraph.Build(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Trace.Voids) != 1 {
		t.Fatalf("voids %v caveats %v", r.Trace.Voids, r.Caveats)
	}
	var names []string
	for _, rm := range r.Trace.Rooms {
		names = append(names, rm.Name)
	}
	if slices.Contains(names, "ATRIUM") || !slices.Contains(names, "Office") || len(names) != 2 {
		t.Errorf("rooms %v", names)
	}
	// The void reaches the balustrade, 6200–10800 mm across.
	var lo, hi float64 = 1e18, -1e18
	for _, p := range r.Trace.Voids[0] {
		lo, hi = min(lo, p[1]), max(hi, p[1])
	}
	want := (mm(o, 0, 6200).Y - mm(o, 0, 10800).Y)
	if got := hi - lo; got < 0.97*want || got > 1.03*want {
		t.Errorf("void %.0f px deep, want %.0f", got, want)
	}
}

// TestVoidLayer: a closed outline on a void layer is a void without a
// label; one too small, or one its corners' mean is outside, is left be.
func TestVoidLayer(t *testing.T) {
	const void = "A-FLOR-VOID"
	d := twoRooms().Layer(void, 1, 0)
	d.Add(
		dxftest.Rect(void, 2000, 2000, 4000, 5000),
		dxftest.Rect(void, 8000, 2000, 8500, 2500),
		dxftest.E("LWPOLYLINE", 8, void, 90, 6, 70, 1,
			10, 7000.0, 20, 5000.0, 10, 11000.0, 20, 5000.0, 10, 11000.0, 20, 5500.0,
			10, 7500.0, 20, 5500.0, 10, 7500.0, 20, 7500.0, 10, 7000.0, 20, 7500.0),
	)
	in, _ := read(t, d)
	if len(in.Voids) != 1 {
		t.Fatalf("voids %v", in.Voids)
	}
	r, err := floorgraph.Build(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Trace.Voids) != 1 || len(r.Trace.Rooms) != 2 {
		t.Errorf("voids %v, %d rooms, caveats %v", r.Trace.Voids, len(r.Trace.Rooms), r.Caveats)
	}
}
