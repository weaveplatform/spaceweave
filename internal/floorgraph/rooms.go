package floorgraph

import (
	"math"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Room size limits, metres. minRoomArea matches the site validator's 0.5 m²
// (test/invariants.mjs), with a margin for millimetre rounding; a face
// narrower than minRoomWidth is the cavity between two wall lines or a
// sliver left by a near miss, not a room.
const (
	minRoomArea  = 0.5 * 1.04
	minRoomWidth = 0.4
)

// room is a face of the wall graph being made into a plan.Room.
type room struct {
	poly    []Point
	areaM   float64
	c       Point
	name    string
	label   Point
	labeled bool
}

// faceRoom turns a bounded face into a room, or reports it too small or
// thin to be one.
func faceRoom(poly []Point, sx, sy float64) (room, bool) {
	poly = simplify(poly, 0.25)
	if len(poly) < 3 {
		return room{}, false
	}
	areaM, width := faceSize(poly, sx, sy)
	if areaM <= minRoomArea || width < minRoomWidth {
		return room{}, false
	}
	return room{poly: poly, areaM: areaM, c: centroid(poly)}, true
}

// faceSize is a polygon's area in m² and its width in metres: the narrower
// side of the rectangle with the same area and perimeter, exact for a
// rectangle and an underestimate for anything less compact.
func faceSize(poly []Point, sx, sy float64) (areaM, width float64) {
	areaM = math.Abs(signedArea(poly)) / float64(sx*sy)
	perim := 0.0
	for i := range poly {
		d := sub(poly[(i+1)%len(poly)], poly[i])
		perim += float64(norm(d) * metresPerPx(d, sx, sy))
	}
	width = perim / 4
	if disc := float64(perim*perim)/16 - areaM; disc > 0 {
		width = perim/4 - math.Sqrt(disc)
	}
	return areaM, width
}

// unitWords are the words a dimension or area label is made of besides
// numbers: "12'-3\" x 18'-11\"", "3.60 m", "12.5 m²", "4.2 x 3.1", "Area 20 sq ft".
var unitWords = map[string]bool{
	"m": true, "mm": true, "cm": true, "sqm": true, "sq": true, "ft": true, "feet": true,
	"foot": true, "in": true, "inch": true, "inches": true, "sf": true, "sqft": true, "x": true,
	"by": true, "area": true, "metres": true, "meters": true, "metre": true, "meter": true,
	"approx": true, "ca": true,
}

// looksLikeDimension says a label is a measurement, not a name: it has a
// digit and every word in it is a unit.
func looksLikeDimension(s string) bool {
	digit := false
	for _, r := range s {
		if unicode.IsDigit(r) {
			digit = true
			break
		}
	}
	if !digit {
		return false
	}
	words := strings.FieldsFunc(
		strings.ToLower(s),
		func(r rune) bool { return !unicode.IsLetter(r) },
	)
	for _, w := range words {
		if !unitWords[w] {
			return false
		}
	}
	return true
}

// cleanLabel is the label's text on one line, or "" when it can't name a
// room.
func cleanLabel(l Label) string {
	if !finite(l.At.X, l.At.Y) {
		return ""
	}
	text := strings.Join(strings.Fields(l.Text), " ")
	if looksLikeDimension(text) {
		return ""
	}
	return text
}

// betterLabel orders a room's candidate names: by kind (a name, then text
// that may be one, then a number), then the tallest text (room names are
// lettered larger than notes), then the longest, then the topmost, then the
// leftmost, then the alphabetically first.
func betterLabel(x, y Label, xt, yt string) bool {
	if rx, ry := labelRank[x.Kind], labelRank[y.Kind]; rx != ry {
		return rx > ry
	}
	hx, hy := x.Height, y.Height
	if !finite(hx) {
		hx = 0
	}
	if !finite(hy) {
		hy = 0
	}
	switch {
	case hx != hy:
		return hx > hy
	case utf8.RuneCountInString(xt) != utf8.RuneCountInString(yt):
		return utf8.RuneCountInString(xt) > utf8.RuneCountInString(yt)
	case cmpPoint(x.At, y.At) != 0:
		return cmpPoint(x.At, y.At) < 0
	}
	return xt < yt
}

// An unnamed room this big, m², is open plan, not a numbered room: a
// meeting room or office is smaller, and a floor's unwalled workspace is
// what's left between its rooms and cores.
const openPlanArea = 100.0

// openPlanName is what an unnamed open-plan room is called.
const openPlanName = "Open plan"

// smallRoomWords name rooms never openPlanArea big. In an open-plan room
// such a label names a core, stair or WC drawn without walls round it, not
// the room.
var smallRoomWords = []string{
	"core", "stair", "staircase", "lift", "elevator", "riser", "shaft", "duct", "wc",
	"toilet", "shower", "cleaner", "cupboard",
}

// fitsRoom says a label can name a room of areaM m²: a label in an open-plan
// room names it only if it isn't a small room's name, and only if it is
// the only number there when it is a number (several are the numbers of
// areas it holds).
func fitsRoom(text string, kind LabelKind, areaM float64, numbers int) bool {
	if areaM < openPlanArea {
		return true
	}
	if kind == LabelNumber {
		return numbers == 1
	}
	return !hasWord(nameWords(text), smallRoomWords)
}

func nameWords(name string) []string {
	return strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// nameRooms gives each room the best label inside it. It returns how many
// usable labels fell in no room.
func nameRooms(rooms []room, labels []Label) (stray int) {
	best := make([]int, len(rooms))
	texts := make([]string, len(labels))
	for i := range best {
		best[i] = -1
	}
	in := make([]int, len(labels))
	numbers := make([]map[string]bool, len(rooms))
	for li, l := range labels {
		texts[li] = cleanLabel(l)
		in[li] = -1
		if texts[li] == "" {
			continue
		}
		in[li] = slices.IndexFunc(rooms, func(r room) bool { return inside(l.At, r.poly) })
		if in[li] < 0 {
			stray++
			continue
		}
		if l.Kind == LabelNumber {
			if numbers[in[li]] == nil {
				numbers[in[li]] = map[string]bool{}
			}
			numbers[in[li]][texts[li]] = true
		}
	}
	for li, l := range labels {
		ri := in[li]
		if ri < 0 || !fitsRoom(texts[li], l.Kind, rooms[ri].areaM, len(numbers[ri])) {
			continue
		}
		if b := best[ri]; b < 0 || betterLabel(l, labels[b], texts[li], texts[b]) {
			best[ri] = li
		}
	}
	for ri, li := range best {
		if li >= 0 {
			rooms[ri].name, rooms[ri].label, rooms[ri].labeled = texts[li], qp(labels[li].At), true
		}
	}
	return stray
}

// readingOrder sorts rooms top to bottom in rows rowTol px tall, then left
// to right, so ids and "Room N" numbers read like the drawing.
func readingOrder(rooms []room, rowTol float64) {
	slices.SortStableFunc(rooms, func(x, y room) int {
		if c := cmpFloat(x.c.Y, y.c.Y); c != 0 {
			return c
		}
		return cmpFloat(x.c.X, y.c.X)
	})
	row := make([]int, len(rooms))
	for i := 1; i < len(rooms); i++ {
		row[i] = row[i-1]
		start := slices.Index(row, row[i-1])
		if rooms[i].c.Y-rooms[start].c.Y > rowTol {
			row[i]++
		}
	}
	idx := make([]int, len(rooms))
	for i := range idx {
		idx[i] = i
	}
	slices.SortStableFunc(idx, func(i, j int) int {
		if row[i] != row[j] {
			return row[i] - row[j]
		}
		return cmpFloat(rooms[i].c.X, rooms[j].c.X)
	})
	sorted := make([]room, len(rooms))
	for k, i := range idx {
		sorted[k] = rooms[i]
	}
	copy(rooms, sorted)
}

// The room vocabulary, in one place. A room's zone comes from the first
// table with a word in its name (so "Meeting room store" is storage); its
// floor is tile for a wet room or kitchen, wood for any other social space,
// carpet otherwise. Words match whole, with plurals ("Lifts", "WCs").
var (
	zoneWords = []struct {
		zone  string
		words []string
	}{
		{"service", []string{
			"wc", "toilet", "restroom", "washroom", "lavatory", "bath", "bathroom", "shower",
			"store", "storage", "cupboard", "closet", "server", "comms", "plant", "riser",
			"shaft", "stair", "stairs", "staircase", "lift", "elevator", "corridor", "hall",
			"hallway", "lobby", "vestibule", "utility", "cleaner", "janitor", "men", "women",
			"ladies", "gents",
		}},
		{
			"meet",
			[]string{
				"meeting",
				"conference",
				"board",
				"boardroom",
				"interview",
				"huddle",
				"training",
			},
		},
		{"social", []string{
			"kitchen", "kitchenette", "break", "breakout", "lounge", "reception", "cafe", "café",
			"canteen", "dining", "living", "pantry", "waiting", "coffee",
		}},
	}
	tileWords = []string{
		"wc", "toilet", "restroom", "washroom", "lavatory", "bath", "bathroom", "shower",
		"kitchen", "kitchenette", "pantry", "utility", "cleaner", "men", "women", "ladies", "gents",
	}
)

func hasWord(words []string, vocab []string) bool {
	for _, w := range words {
		for _, v := range vocab {
			if w == v || w == v+"s" || w == v+"es" {
				return true
			}
		}
	}
	return false
}

// classify gives a room name's zone and floor.
func classify(name string) (zone, floor string) {
	words := nameWords(name)
	zone = "work"
	for _, z := range zoneWords {
		if hasWord(words, z.words) {
			zone = z.zone
			break
		}
	}
	switch {
	case hasWord(words, tileWords):
		floor = "tile"
	case zone == "social":
		floor = "wood"
	default:
		floor = "carpet"
	}
	return zone, floor
}
