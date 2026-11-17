package cadfront

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/weaveplatform/spaceweave/internal/floorgraph"
)

// Which text names rooms. A plan is lettered with far more than room
// names: desk numbers, equipment tags ("85\" TV"), fitting codes, notes.
// The layer says which is which wherever the drawing has a layer for room
// names, and then nothing else names a room. Without one, text on a layer
// that says it holds something else is dropped, and the rest must look
// like a name.

// textClass is what a text layer holds, as far as naming rooms goes.
type textClass int

const (
	textAny    textClass = iota // says nothing
	textName                    // room names
	textNumber                  // room numbers
	textNever                   // something that is never a room's name
)

var (
	// roomWords and the qualifier words make a room-name or room-number
	// layer: Uniclass "Zz_20_10_70_65-T_Room Name", AIA "A-AREA-IDEN",
	// "ROOM-NAME", "RM_NO", German "RAUMNAME" and "RAUMNUMMER".
	roomWords = []string{"ROOM*", "RM", "RMS", "SPACE*", "AREA*", "RAUM*", "LOCAL*"}
	// Not DESC: a CAFM export's "Space Type Descs" are codes ("OP",
	// "MEETP") for the kind of space, not its name.
	nameWords   = []string{"NAME*", "TITLE*", "LABEL*", "IDEN*", "BEZEICHNUNG*", "NOM"}
	numberWords = []string{"NUM*", "NO", "NR", "NOS", "NBR", "NUMMER*", "NUMERO*"}
	// squashedRoom catches the names written as one word: "ROOMNAME",
	// "RmNo", "RAUMNUMMER".
	squashedName   = []string{"ROOMNAME", "RMNAME", "RAUMNAME", "ROOMTAG", "ROOMTITLE"}
	squashedNumber = []string{"ROOMNO", "ROOMNUM", "RMNO", "RMNUM", "RAUMNR", "RAUMNUMMER"}
	// neverWords are layers whose text is never a room's name: furniture,
	// equipment, fittings and services tags, desk and seat numbers, notes,
	// dimensions and the title block.
	neverWords = []string{
		"FURN*", "MOBIL*", "EQUIP*", "EQPM*", "FITT*", "FIXT*", "SANI*", "DESK*", "SEAT*",
		"WORKSTATION*", "WS", "NOTE*", "HELP*", "DIM*", "GRID*", "AXIS", "AXES", "TITLE*",
		"TITL*", "TTLB", "BORDER*", "REV*", "ELEC*", "POWER", "LIGHT*", "DATA", "MECH*",
		"PLUM*", "HVAC", "FIRE", "SIGN*", "KEY*", "LEGEND*", "DOOR*", "WIND*", "STAIR*",
		"LIFT*", "ELEV*", "HATCH*", "PATT*", "SYMB*", "CRS",
	}
)

// textLayer classifies a layer by what its text says about rooms.
func textLayer(name string) textClass {
	t := tokens(name)
	squashed := strings.Join(t, "")
	for _, w := range squashedNumber {
		if strings.Contains(squashed, w) {
			return textNumber
		}
	}
	for _, w := range squashedName {
		if strings.Contains(squashed, w) {
			return textName
		}
	}
	if matches(t, roomWords) {
		switch {
		case matches(t, numberWords):
			return textNumber
		case matches(t, nameWords), matches(t, []string{"ANNO*", "TEXT*", "TXT"}):
			// A-ANNO-ROOM, ROOM-TEXT: the room's own lettering.
			return textName
		}
	}
	if matches(t, neverWords) {
		return textNever
	}
	if r, ok := uniclassRole(t); ok && r == roleFurniture {
		return textNever
	}
	return textAny
}

var (
	// productSize is a size in inches or a W × D: "85\" TV", "1200x800",
	// "1600 X 800 DESK". Equipment and furniture are labelled so; rooms are
	// not.
	productSize = regexp.MustCompile(`(?i)\d\s*(?:"|''|”|″|inch\b|in\b)|\d\s*[x×*]\s*\d`)
	// voidName is how a plan labels a hole in the floor. "Open to above"
	// is not one: the floor is there, the ceiling isn't.
	// "VOID" leads whatever follows ("VOID OVER RECEPTION"); an "ATRIUM
	// CAFÉ" is a room in one, so the other words stand alone or with an id.
	voidName = regexp.MustCompile(
		`(?i)^\s*(?:void\b.*|open\s+to\s+below\b.*|` +
			`(?:atrium|light\s*well|luftraum|vide|tr[eé]mie)(?:\s+(?:\d+[a-z]?|[a-z]))?)\s*$`,
	)
)

// looksLikeName says text from a layer that doesn't say what it holds
// could be a room's name: it has a word of two letters or more, is no
// product size, and has as many letters as digits ("Room 12-34" does; a
// desk number "4C-117" is a code).
func looksLikeName(s string) bool {
	if productSize.MatchString(s) {
		return false
	}
	letters, digits, run, word := 0, 0, 0, false
	for _, r := range s {
		switch {
		case unicode.IsLetter(r):
			letters++
			run++
			word = word || run >= 2
		case unicode.IsDigit(r):
			digits++
			run = 0
		default:
			run = 0
		}
	}
	return word && letters >= digits
}

// labelKind says how a text names rooms, or that it doesn't. named says
// the drawing has a room-name layer: then only that and room-number layers
// name rooms.
func labelKind(layer, text string, named bool) (floorgraph.LabelKind, bool) {
	if productSize.MatchString(text) {
		return 0, false
	}
	switch textLayer(layer) {
	case textName:
		return floorgraph.LabelName, true
	case textNumber:
		return floorgraph.LabelNumber, true
	case textNever:
		return 0, false
	}
	if named || !looksLikeName(text) {
		return 0, false
	}
	return floorgraph.LabelText, true
}
