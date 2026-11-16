package cadfront

import (
	"strings"
	"unicode"
)

// tokens splits a layer or block name into upper-case words: at anything
// that isn't a letter or digit, between letters and digits, and where a
// lower-case letter meets an upper-case one ("TaskChair", "DOOR900").
func tokens(name string) []string {
	var out []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			out = append(out, strings.ToUpper(string(cur)))
			cur = cur[:0]
		}
	}
	prev := rune(0)
	for _, r := range name {
		switch {
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			flush()
		case len(cur) > 0 && (unicode.IsDigit(r) != unicode.IsDigit(prev) ||
			unicode.IsUpper(r) && unicode.IsLower(prev)):
			flush()
			cur = append(cur, r)
		default:
			cur = append(cur, r)
		}
		prev = r
	}
	flush()
	return out
}

// matches reports whether any token fits any pattern: a word, or a prefix
// ending in *.
func matches(toks []string, patterns []string) bool {
	for _, t := range toks {
		for _, p := range patterns {
			if pre, ok := strings.CutSuffix(p, "*"); ok {
				if strings.HasPrefix(t, pre) {
					return true
				}
			} else if t == p {
				return true
			}
		}
	}
	return false
}

// Layer and block naming. AIA (A-WALL, A-DOOR, A-GLAZ, A-FURN) is the
// commonest scheme; the others are the plain English, German, French and
// Spanish words offices actually use. Bare "DR" is a word on its own so
// that "DRAWING" isn't a door.
var (
	wallWords = []string{
		"WALL*", "MUR", "MURS", "MURO", "MUROS", "WAND", "WÄNDE", "WAENDE", "PARED*",
	}
	glassWords  = []string{"GLAZ*", "GLASS*", "GLAS", "CURT*", "VERRE", "VITR*"}
	windowWords = []string{
		"WIND*", "WIN", "WDW*", "WNDW*", "FENSTER*", "FENETRE*", "FENÊTRE*", "VENTANA*",
	}
	doorWords = []string{"DOOR*", "DR", "DRS", "TÜR*", "TUER*", "PORTE*", "PUERTA*"}
	furnWords = []string{"FURN*", "MOBIL*", "MÖBEL*", "MOEBEL*", "SANITARY", "SANI*"}
	// structWords are the load-bearing layers: core walls drawn as line
	// pairs, and columns. They are read as walls, so a drawing whose shell
	// and cores live only there still encloses its rooms.
	structWords = []string{
		"STRUCT*", "COLUMN*", "COL", "COLS", "PILLAR*", "PIER*", "STUETZ*", "STÜTZ*",
		"POTEAU*", "PILAR*", "TRAGWERK*",
	}
	// vetoWords outrank everything: a door's tag, a wall's dimensions or
	// the structural grid is drawn on a layer named for walls, doors or
	// structure, and none of it is one. A wall's hatch layer (A-WALL-PATT)
	// is not vetoed: its poché marks the wall.
	vetoWords = []string{
		"GRID*", "AXIS", "AXES", "DIM*", "TEXT*", "TXT", "ANNO*", "IDEN", "NOTE*", "TAG*",
		"HELPLINE*",
	}
	// notWallWords are layers whose lines are never walls, for drawings
	// that don't name a wall layer: every other layer's lines are then
	// candidates. Stairs, overhead and hidden lines run parallel in pairs
	// like walls do.
	notWallWords = []string{
		"DIM*", "COTA*", "COTE*", "BEMASS*", "TEXT*", "TXT", "ANNO*", "NOTE*", "TAG*", "SYMB*",
		"LEAD*", "HATCH*", "PATT*", "GRID*", "AXIS", "AXES", "ACHS*", "TITLE*", "TITL*", "TTLB",
		"BORDER*", "VIEWPORT*", "VPORT*", "VP", "DEFPOINTS", "IDEN", "AREA*", "ROOM*", "SPACE*",
		"ELEC*", "LITE", "LIGHT*", "CLNG", "CEIL*", "PLUM*", "FIXT*", "EQPM*", "STAIR*", "STRS",
		"TREPPE*", "OVHD", "HIDD*", "HIDDEN", "CENTER*", "CNTR", "REV*", "SECT*", "ELEV*",
		"ROOF*", "LOGO", "NORTH", "BRISE", "SHADING", "LOUVRE*", "LOUVER*",
	}
)

// role is what a layer or block is for.
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

// layerRole classifies a layer name. Windows come before glass so that
// A-GLAZ-WIND is a window, and doors and windows before walls so that a
// wall-opening layer such as A-WALL-DOOR holds doors.
func layerRole(name string) role {
	t := tokens(name)
	if matches(t, vetoWords) {
		return roleNotWall
	}
	if r, ok := uniclassRole(t); ok {
		return r
	}
	switch {
	case matches(t, doorWords):
		return roleDoor
	case matches(t, windowWords):
		return roleWindow
	case matches(t, glassWords):
		return roleGlass
	case matches(t, wallWords), matches(t, structWords):
		return roleWall
	case matches(t, furnWords):
		return roleFurniture
	case matches(t, notWallWords):
		return roleNotWall
	}
	return roleNone
}

// uniclassRole reads a Uniclass 2015 classification code in a layer name,
// as BS 1192 and ISO 19650 layer naming put one ("AR_EF_25_10-X_Existing
// Walls", "Ss_40_15_Furniture"). The code says what the layer holds even
// where its description is missing or in another language. Only the codes
// a floor plan's evidence depends on are read; anything else falls through
// to the words. Doors and windows share EF_25_30, so the words split them
// (a window layer says so; anything else there is a door).
func uniclassRole(t []string) (role, bool) {
	for i := 0; i+1 < len(t); i++ {
		code := t[i] + "_" + t[i+1]
		if i+2 < len(t) {
			code += "_" + t[i+2]
		}
		switch {
		case strings.HasPrefix(code, "EF_25_10"):
			if matches(t, glassWords) {
				return roleGlass, true
			}
			return roleWall, true
		case strings.HasPrefix(code, "EF_25_30"):
			if matches(t, windowWords) {
				return roleWindow, true
			}
			return roleDoor, true
		case strings.HasPrefix(code, "SS_25_50"):
			// Solar shading: brise soleil and louvres stand off the
			// façade in long parallel runs, and enclose nothing.
			return roleNotWall, true
		case strings.HasPrefix(code, "SS_25"):
			// Wall and barrier systems: partitions and glazed screens.
			if matches(t, glassWords) {
				return roleGlass, true
			}
			return roleWall, true
		case strings.HasPrefix(code, "EF_20"):
			return roleWall, true
		case strings.HasPrefix(code, "EF_35"), strings.HasPrefix(code, "ZZ_"),
			strings.HasPrefix(code, "SL_90"):
			// Stairs and ramps, annotation, and lift installations: drawn
			// in parallel pairs like walls, and none of them is one.
			return roleNotWall, true
		case strings.HasPrefix(code, "SS_40_15"), strings.HasPrefix(code, "PR_40_20"),
			strings.HasPrefix(code, "PR_40_70"):
			return roleFurniture, true
		}
	}
	return roleNone, false
}

// blockRole classifies an insert by its block's name, then its layer.
func blockRole(block, layer string) role {
	t := tokens(block)
	switch {
	case matches(t, doorWords):
		return roleDoor
	case matches(t, windowWords):
		return roleWindow
	case furnitureType(block) != "":
		return roleFurniture
	}
	switch r := layerRole(layer); r {
	case roleDoor, roleWindow, roleFurniture:
		return r
	}
	return roleNone
}

// furnitureKeys map block-name keywords to plan.Types, first match wins.
// Long keywords match anywhere in the name with separators removed
// ("Arm_Chair", "OfficeDesk01"); short ones ("WC", "TUB") only as whole
// words, so "TUBE" isn't a bath. Earlier entries shadow later ones:
// CUPBOARD is a cabinet before BOARD is a whiteboard, ARMCHAIR an armchair
// before CHAIR a chair, and a DESK before a TABLE.
var furnitureKeys = []struct {
	typ          string
	inside, word []string
}{
	{"cabinet", []string{
		"CUPBOARD", "CABINET", "CREDENZA", "PEDESTAL", "DRAWER", "SIDEBOARD", "SCHRANK",
	}, nil},
	{"storage", []string{"STORAGE", "LOCKER", "SHELF", "SHELV", "BOOKCASE", "REGAL"}, nil},
	{"board", []string{"WHITEBOARD", "PINBOARD"}, []string{"BOARD"}},
	{"armchair", []string{"ARMCHAIR", "LOUNGECHAIR", "EASYCHAIR", "SESSEL", "FAUTEUIL"}, nil},
	{"sofa", []string{"SOFA", "COUCH", "SETTEE"}, nil},
	{"stool", []string{"STOOL", "HOCKER"}, nil},
	{"chair", []string{"CHAIR", "STUHL", "CHAISE", "SILLA"}, []string{"SEAT"}},
	{"desk", []string{
		"DESK", "WORKSTATION", "SCHREIBTISCH", "BUREAU", "ESCRITORIO",
	}, []string{"WS"}},
	{"table", []string{"TABLE", "TISCH", "MESA"}, nil},
	{"counter", []string{"COUNTER", "WORKTOP", "KITCHEN", "THEKE", "TRESEN"}, nil},
	{"copier", []string{"COPIER", "PRINTER", "PHOTOCOP", "KOPIER"}, []string{"MFD", "MFP"}},
	{"sink", []string{"SINK", "SPUELE", "SPÜLE", "EVIER"}, nil},
	{"basin", []string{
		"BASIN", "LAVATORY", "LAVABO", "WASCHBECKEN",
	}, []string{"LAV", "WHB", "HWB"}},
	{"toilet", []string{"TOILET", "WATERCLOSET"}, []string{"WC"}},
	{"tub", []string{"BATHTUB", "BADEWANNE"}, []string{"BATH", "TUB"}},
	{"plant", []string{"PLANT", "PFLANZE"}, nil},
	{"stairs", []string{"STAIR", "TREPPE", "ESCALIER"}, nil},
}

// furnitureType is the furniture a block name names, or "". "table" is
// not a plan type: the caller makes it a roundTable or a desk by shape.
func furnitureType(block string) string {
	t := tokens(block)
	squashed := strings.Join(t, "")
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
