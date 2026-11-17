package cadfront

import (
	"strconv"
	"strings"

	"github.com/weaveplatform/spaceweave/internal/autotrace/profile"
)

// tokens splits a block name into upper-case words; see profile.Words.
func tokens(name string) []string { return profile.Words(name) }

// matches reports whether any token fits any pattern: a word, or a prefix
// ending in *.
func matches(toks []string, patterns []string) bool { return profile.AnyWord(toks, patterns) }

// Block naming. What a layer holds is its profile's to say (see
// layers.go); a block's own name is read here, because block libraries
// name their symbols the same whatever the office's layer convention. Bare
// "DR" is a word on its own so that "DRAWING" isn't a door.
var (
	doorWords   = []string{"DOOR*", "DR", "DRS", "TÜR*", "TUER*", "PORTE*", "PUERTA*"}
	windowWords = []string{
		"WIND*", "WIN", "WDW*", "WNDW*", "FENSTER*", "FENETRE*", "FENÊTRE*", "VENTANA*",
	}
)

// role is what a layer's lines, or a block, are.
type role int

const (
	roleNone role = iota
	roleWall
	roleGlass
	roleDoor
	roleWindow
	roleFurniture
	roleNotWall
)

// blockRole classifies an insert by its block's name, then by what its
// layer's lines are.
func blockRole(block string, layer role) role {
	t := tokens(block)
	switch {
	case matches(t, doorWords):
		return roleDoor
	case matches(t, windowWords):
		return roleWindow
	case furnitureType(block) != "":
		return roleFurniture
	}
	switch layer {
	case roleDoor, roleWindow, roleFurniture:
		return layer
	}
	return roleNone
}

// furnitureKeys map block-name keywords to plan.Types, first match wins.
// Long keywords match anywhere in the name with separators removed
// ("Arm_Chair", "OfficeDesk01"); short ones ("WC", "TUB") only as whole
// words, so "TUBE" isn't a bath and "PANEL" isn't a pan. Earlier entries
// shadow later ones: CUPBOARD is a cabinet before BOARD is a whiteboard,
// ARMCHAIR an armchair before CHAIR a chair, a DESK before a TABLE, a
// KITCHEN sink a counter before SINK a sink. The words are English,
// German, French, Spanish, Italian and Dutch, and the abbreviations
// furniture libraries use whatever the vendor.
//
// "bench" is not a plan type: a bench is bench desking in an office plan
// and a seat everywhere else, so the caller decides by its depth and by
// desk numbers (see furnish).
var furnitureKeys = []struct {
	typ          string
	inside, word []string
}{
	{"cabinet", []string{
		"CUPBOARD", "CABINET", "CREDENZA", "PEDESTAL", "DRAWER", "SIDEBOARD", "SCHRANK",
		"ARMOIRE", "ARMARIO", "CASSETTIERA", "KAST",
	}, []string{"CUP", "CPD", "PED", "CAB"}},
	{"storage", []string{
		"STORAGE", "LOCKER", "SHELF", "SHELV", "BOOKCASE", "REGAL", "LIBRARY", "ETAGERE",
		"ESTANTE", "SCAFFALE", "SPIND", "CASIER", "TAQUILLA",
	}, []string{"LKR", "LKRS", "STOR"}},
	{"board", []string{"WHITEBOARD", "PINBOARD"}, []string{"BOARD"}},
	{"armchair", []string{
		"ARMCHAIR", "LOUNGECHAIR", "EASYCHAIR", "SESSEL", "FAUTEUIL", "POLTRONA", "SILLON",
		"RECLINER", "LOUNGER",
	}, nil},
	{"sofa", []string{"SOFA", "COUCH", "SETTEE", "DIVANO", "CANAPE", "LOVESEAT"}, nil},
	{"stool", []string{
		"STOOL", "HOCKER", "OTTOMAN", "POUF", "TABOURET", "TABURETE", "SGABELLO", "KRUK",
	}, nil},
	{"chair", []string{
		"CHAIR", "STUHL", "CHAISE", "SILLA", "SEDIA", "STOEL", "CADEIRA",
	}, []string{"SEAT", "CHR"}},
	{"desk", []string{
		"DESK", "WORKSTATION", "SCHREIBTISCH", "BUREAU", "ESCRITORIO", "SCRIVANIA",
		"WORKPLACE", "ARBEITSPLATZ",
	}, []string{"WS", "WKS", "WKST", "DSK"}},
	{"bench", []string{"BENCH"}, nil},
	{"table", []string{"TABLE", "TISCH", "MESA", "TAVOLO", "TAFEL"}, []string{"TBL"}},
	{"counter", []string{
		"COUNTER", "WORKTOP", "KITCHEN", "THEKE", "TRESEN", "FRIDGE", "REFRIGERATOR",
		"KUEHLSCHRANK", "KÜHLSCHRANK", "DISHWASHER", "PANTRY",
	}, nil},
	{"copier", []string{"COPIER", "PRINTER", "PHOTOCOP", "KOPIER"}, []string{"MFD", "MFP"}},
	{"sink", []string{"SINK", "SPUELE", "SPÜLE", "EVIER", "LAVELLO"}, nil},
	// A urinal is a basin in the plan: both are wall-hung bowls at about
	// the same height, where a toilet is a pan and cistern on the floor.
	{"basin", []string{
		"BASIN", "LAVATORY", "LAVABO", "WASCHBECKEN", "WASCHTISCH", "VASQUE", "URINAL",
		"URINOIR", "PISSOIR", "ORINATOIO",
	}, []string{"LAV", "WHB", "HWB"}},
	{"toilet", []string{"TOILET", "WATERCLOSET", "INODORO"}, []string{"WC", "PAN"}},
	{"tub", []string{"BATHTUB", "BADEWANNE"}, []string{"BATH", "TUB"}},
	{"plant", []string{"PLANT", "PFLANZE", "PIANTA"}, []string{"TREE", "TREES"}},
	{"stairs", []string{"STAIR", "TREPPE", "ESCALIER"}, nil},
}

// notFurnitureWords name things on furniture layers that the plan has no
// type for, so that their shape isn't read as one that it has: a table
// lamp is not a round table, a wheelchair's turning circle is not a
// chair, a coat stand is not a stool. Lamps and wheelchairs are checked
// before furnitureKeys, since TABLE and CHAIR are in their names; the
// rest after, so a "Living_Wall_Planter" is still a plant.
var (
	notFurnitureFirst = []string{"LAMP", "LUMINAIRE", "WHEELCHAIR", "TURNING"}
	notFurnitureWords = []string{
		"COAT*", "HANGER*", "BIN", "BINS", "RECYCL*", "DRYER*", "DISPENSER*", "TAP", "TAPS",
		"MIXER*", "SCREEN*", "ACOUSTIC*", "DIVIDER*", "PANEL*", "WALL", "WALLS", "RAIL*",
		"PHONE*", "KEYBOARD*", "MONITOR*", "MOUSE", "TV", "PC", "SERVER*", "BOOTH*", "POD",
		"PODS", "HINGE*", "PILASTER*", "BRACKET*", "INDICATOR*", "PIPE*", "MAT", "RUG",
		"CARPET*", "TROLLEY*", "CLOCK*", "MIRROR*", "VENDING", "BOILER", "RADIATOR*",
	}
)

// furnitureType is the furniture a block name names, or "". "table" is
// not a plan type: the caller makes it a roundTable or a desk by shape;
// nor is "bench" (see furnitureKeys).
func furnitureType(block string) string {
	t := tokens(block)
	squashed := strings.Join(t, "")
	for _, w := range notFurnitureFirst {
		if strings.Contains(squashed, w) {
			return ""
		}
	}
	for _, k := range furnitureKeys {
		for _, s := range k.inside {
			if strings.Contains(squashed, s) {
				return k.typ
			}
		}
		if matches(t, k.word) {
			return k.typ
		}
	}
	return ""
}

// notFurniture reports whether a block's name says it is something the
// plan can't draw, which its shape must not then be guessed as.
func notFurniture(block string) bool {
	t := tokens(block)
	squashed := strings.Join(t, "")
	for _, w := range notFurnitureFirst {
		if strings.Contains(squashed, w) {
			return true
		}
	}
	return matches(t, notFurnitureWords)
}

// nameNumbers reads the numbers a furniture block's name gives: sizes in
// millimetres ("1200x800", "BENCH_6_1600" both give 1.2 m or 1.6 m) as
// metres, and a small count ("_6_", "4P") of the workplaces a bench
// holds, 0 if none. Either is only a hint, used where the block's shape
// agrees with it: a model number reads the same.
func nameNumbers(block string) (sizes []float64, count int) {
	for _, t := range tokens(block) {
		v, err := strconv.Atoi(t)
		if err != nil || t[0] == '0' {
			continue
		}
		switch {
		case v >= 300 && v <= 6000 && len(t) >= 3:
			sizes = append(sizes, float64(v)/1000)
		case v >= 2 && v <= 16 && count == 0:
			count = v
		}
	}
	return sizes, count
}
