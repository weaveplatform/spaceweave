// Package profile says what each layer of a CAD drawing holds: walls,
// doors, room names, furniture and the rest. Drawings name their layers by
// a convention (Uniclass 2015 codes under BS 1192, AIA/NCS, a facilities
// system's export, or whatever words the office uses), and a profile maps
// layer names to roles under one.
//
// Four profiles are built in (uniclass, aia, cafm and generic, in
// builtin/) and a client's own profile, a YAML file the page writes when a
// reviewer corrects a layer's role, overrides them. A Set is the rules of
// a client profile followed by the built-ins', and resolves a layer name to
// the role of the first rule that matches it.
//
// # Matching
//
// A rule matches a layer name when every condition it gives holds:
//
//   - layer: a glob over the whole name, ignoring case. * is any run of
//     characters, ? any one, and \ escapes the next. A glob with no
//     wildcards is the layer's exact name, which is what the page writes.
//   - regex: a Go regular expression searched for in the name, ignoring
//     case (anchor it with ^ and $ to match the whole name).
//   - words: one of the name's words (see Words) is one of these; a
//     trailing * matches any word it starts. "WALL*" matches A-WALL-FULL
//     and Walls; "DR" matches DR but not DRAWING.
//   - with: as words, a second list that must match too ("a room word
//     with a number word").
//   - codes: the name carries a Uniclass code starting with one of these
//     (see hasCode): "EF_25_10" matches AR_EF_25_10-X_Walls.
//   - squashed: the name's words run together contain one of these, for
//     names written as one word ("RoomName", "RMNO").
//
// Globs and words are both offered because they suit different authors:
// a person correcting one layer means that layer exactly, while a built-in
// convention means a word wherever it falls in names it has never seen.
//
// # Precedence
//
// A client's rules come first, so a correction always wins. The built-ins'
// follow, highest priority first, then in the order uniclass, aia, cafm,
// generic, then in file order. Priorities let the conventions interleave
// the way a careful reader would: room lettering (300) before
// dimensions and text (200) before classification codes (100) before
// plain words (0), so "A-WALL-DIMS" is dimensions although it says WALL,
// and "AR_Zz_..._Room Name" names rooms although Zz is annotation.
package profile

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Role is what a layer holds.
type Role string

// The roles. The strings are what profiles and report.json say, and what
// js/workbench.js offers; changing one breaks every client profile that
// uses it.
const (
	Wall       Role = "wall"
	Structure  Role = "structure"
	Glass      Role = "glass"
	Door       Role = "door"
	Window     Role = "window"
	Furniture  Role = "furniture"
	Sanitary   Role = "sanitary"
	RoomName   Role = "room-name"
	RoomNumber Role = "room-number"
	DeskNumber Role = "desk-number"
	Area       Role = "area"
	Void       Role = "void"
	Stairs     Role = "stairs"
	Lift       Role = "lift"
	Services   Role = "services"
	Annotation Role = "annotation"
	Ignore     Role = "ignore"
	NotWall    Role = "not-wall"
	// Unknown is no role: no rule matched, and the tracer reads the layer
	// by what is drawn on it. It is never written in a profile.
	Unknown Role = "unknown"
)

// RoleInfo describes a role to someone choosing one.
type RoleInfo struct {
	Role  Role   `json:"role"`
	Label string `json:"label"`
	Help  string `json:"help"`
}

// Roles are the roles a rule can give, in the order the page lists them.
var Roles = []RoleInfo{
	{Wall, "Walls", "Lines in pairs a wall's thickness apart are walls."},
	{Structure, "Structure", "Structural walls and columns; read as walls."},
	{Glass, "Glazing", "Glazed partitions and curtain walling; read as glass walls."},
	{Door, "Doors", "Door blocks and swings."},
	{Window, "Windows", "Window blocks, and window runs in external walls."},
	{Furniture, "Furniture", "Every block on it is furniture, typed by name or shape."},
	{Sanitary, "Sanitary", "Furniture that is sanitary fittings: WCs, basins, urinals."},
	{RoomName, "Room names", "Its text names rooms; then no other text does."},
	{RoomNumber, "Room numbers", "Its text numbers rooms."},
	{DeskNumber, "Desk numbers", "Its text numbers desks, which marks what it sits on as one."},
	{Area, "Areas", "Closed outlines of rooms or spaces, used when a drawing has no walls."},
	{Void, "Voids", "Closed outlines of holes in the floor: atria, light wells."},
	{Stairs, "Stairs", "Stairs and ramps: drawn in parallel pairs, and not walls."},
	{Lift, "Lifts", "Lift cars and shafts: not walls."},
	{Services, "Services", "Fittings, equipment, lighting and power: neither walls nor names."},
	{
		Annotation,
		"Annotation",
		"Text and tags: lines bound nothing; text that looks like a name may still name a room.",
	},
	{
		Ignore,
		"Ignore",
		"Dimensions, grids, notes, hatching and title blocks: nothing on it is read.",
	},
	{
		NotWall,
		"Not walls",
		"Linework that is never a wall (overhead, hidden, ceilings, shading); it may still edge a void.",
	},
}

// Known reports whether r is a role a rule can give.
func Known(r Role) bool {
	return slices.ContainsFunc(Roles, func(i RoleInfo) bool { return i.Role == r })
}

func roleNames() string {
	names := make([]string, len(Roles))
	for i, r := range Roles {
		names[i] = string(r.Role)
	}
	return strings.Join(names, ", ")
}

// Profile is one profile file.
type Profile struct {
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Rules       []Rule `json:"rules"                 yaml:"rules"`
}

// Rule gives layers whose names match it a role. See the package
// documentation for what each condition means.
type Rule struct {
	Role     Role     `json:"role"               yaml:"role"`
	Layer    string   `json:"layer,omitempty"    yaml:"layer,omitempty"`
	Regex    string   `json:"regex,omitempty"    yaml:"regex,omitempty"`
	Words    []string `json:"words,omitempty"    yaml:"words,omitempty,flow"`
	With     []string `json:"with,omitempty"     yaml:"with,omitempty,flow"`
	Codes    []string `json:"codes,omitempty"    yaml:"codes,omitempty,flow"`
	Squashed []string `json:"squashed,omitempty" yaml:"squashed,omitempty,flow"`
	Priority int      `json:"priority,omitempty" yaml:"priority,omitempty"`
	// Note says why, for whoever reads the profile next.
	Note string `json:"note,omitempty" yaml:"note,omitempty"`
}

// Errors.
var (
	ErrInvalid  = errors.New("invalid profile")
	ErrName     = errors.New("invalid profile name")
	ErrNotFound = errors.New("no such profile")
	ErrBuiltin  = errors.New("built-in profiles can't be changed")
)

// unknownKey turns yaml's "field x not found in type profile.Rule" into
// a list of the keys that would have been accepted: a misspelt key
// otherwise reads as a Go type error to someone editing YAML.
var unknownKey = regexp.MustCompile(`field (\S+) not found in type profile\.(Rule|Profile)`)

// Parse reads a profile from YAML (or JSON, which is YAML too), rejecting
// unknown keys and invalid rules. name is only for messages.
func Parse(name string, data []byte) (*Profile, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var p Profile
	err := dec.Decode(&p)
	switch {
	case errors.Is(err, io.EOF):
		// An empty file is a profile with no rules yet.
	case err != nil:
		msg := unknownKey.ReplaceAllStringFunc(err.Error(), func(m string) string {
			sub := unknownKey.FindStringSubmatch(m)
			keys := "role, layer, regex, words, with, codes, squashed, priority, note"
			if sub[2] == "Profile" {
				keys = "description, rules"
			}
			return fmt.Sprintf(
				"unknown key %q (a %s's keys are %s)",
				sub[1],
				strings.ToLower(sub[2]),
				keys,
			)
		})
		return nil, fmt.Errorf("%w: %s: %s", ErrInvalid, name, msg)
	default:
		var extra any
		if dec.Decode(&extra) == nil {
			return nil, fmt.Errorf("%w: %s: one YAML document only", ErrInvalid, name)
		}
	}
	if _, err := p.compile(name, ""); err != nil {
		return nil, err
	}
	return &p, nil
}

// Encode writes a profile as YAML.
func (p *Profile) Encode() ([]byte, error) {
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(p); err != nil {
		return nil, fmt.Errorf("encode profile: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("encode profile: %w", err)
	}
	return b.Bytes(), nil
}

// compiled is a rule ready to match, with where it came from.
type compiled struct {
	role     Role
	source   string
	priority int
	words    []string
	with     []string
	codes    []string
	squashed []string
	layer    *regexp.Regexp
	regex    *regexp.Regexp
}

// compile checks every rule and readies it. source is what a match
// reports ("profile:acme"); only the checks run when it's empty.
func (p *Profile) compile(name, source string) ([]compiled, error) {
	out := make([]compiled, 0, len(p.Rules))
	for i, r := range p.Rules {
		bad := func(format string, args ...any) error {
			return fmt.Errorf(
				"%w: %s: rule %d: %s",
				ErrInvalid,
				name,
				i+1,
				fmt.Sprintf(format, args...),
			)
		}
		if !Known(r.Role) {
			return nil, bad("unknown role %q; the roles are %s", r.Role, roleNames())
		}
		if r.Layer == "" && r.Regex == "" && len(r.Words) == 0 && len(r.With) == 0 &&
			len(r.Codes) == 0 && len(r.Squashed) == 0 {
			return nil, bad("it matches nothing: give a layer, regex, words, codes or squashed")
		}
		if len(r.With) > 0 && len(r.Words) == 0 {
			return nil, bad("with only narrows words; give words too")
		}
		c := compiled{
			role:     r.Role,
			source:   source,
			priority: r.Priority,
			words: upper(
				r.Words,
			),
			with:     upper(r.With),
			codes:    upper(r.Codes),
			squashed: upper(r.Squashed),
		}
		for _, list := range [][]string{c.words, c.with, c.codes, c.squashed} {
			if slices.Contains(list, "") || slices.Contains(list, "*") {
				return nil, bad("an empty word, or a lone *, would match every layer")
			}
		}
		var err error
		if r.Layer != "" {
			c.layer = regexp.MustCompile(Glob(r.Layer))
		}
		if r.Regex != "" {
			if c.regex, err = regexp.Compile("(?i)" + r.Regex); err != nil {
				return nil, bad("regex: %v", err)
			}
		}
		out = append(out, c)
	}
	return out, nil
}

func upper(ss []string) []string {
	if len(ss) == 0 {
		return nil
	}
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = strings.ToUpper(strings.TrimSpace(s))
	}
	return out
}

// Glob is the regular expression a layer glob stands for: anchored,
// ignoring case, with * any run of characters, ? any one and \ escaping
// the next. Every glob is valid, so a layer name pasted as one never
// fails to load; a name with *, ? or \ in it needs EscapeGlob to mean
// only itself.
func Glob(g string) string {
	var b strings.Builder
	b.WriteString(`(?is)^`)
	rs := []rune(g)
	for i := 0; i < len(rs); i++ {
		switch r := rs[i]; {
		case r == '*':
			b.WriteString(`.*`)
		case r == '?':
			b.WriteString(`.`)
		case r == '\\' && i+1 < len(rs):
			i++
			b.WriteString(regexp.QuoteMeta(string(rs[i])))
		default:
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	b.WriteString(`$`)
	return b.String()
}

// EscapeGlob is a glob matching exactly name (ignoring case). It must
// agree with escapeGlob in js/layers.js, which writes the page's rules.
func EscapeGlob(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r == '*' || r == '?' || r == '\\' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// matches reports whether the rule matches a name, given its words and
// those words run together.
func (c *compiled) matches(name string, words []string, squashed string) bool {
	if len(c.words) > 0 && !AnyWord(words, c.words) {
		return false
	}
	if len(c.with) > 0 && !AnyWord(words, c.with) {
		return false
	}
	if len(c.codes) > 0 && !hasCode(words, c.codes) {
		return false
	}
	if len(c.squashed) > 0 && !slices.ContainsFunc(c.squashed, func(s string) bool {
		return strings.Contains(squashed, s)
	}) {
		return false
	}
	if c.layer != nil && !c.layer.MatchString(name) {
		return false
	}
	return c.regex == nil || c.regex.MatchString(name)
}
