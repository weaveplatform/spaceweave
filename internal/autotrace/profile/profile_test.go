package profile

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestWords(t *testing.T) {
	for in, want := range map[string][]string{
		"A-WALL-FULL":   {"A", "WALL", "FULL"},
		"TaskChair02":   {"TASK", "CHAIR", "02"},
		"Ss40_15":       {"SS", "40", "15"},
		"xref|A-Glaz":   {"XREF", "A", "GLAZ"},
		"Tür_innen":     {"TÜR", "INNEN"},
		"  ":            nil,
		"MÖBEL Büro 1x": {"MÖBEL", "BÜRO", "1", "X"},
	} {
		if got := Words(in); !reflect.DeepEqual(got, want) {
			t.Errorf("Words(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCodes(t *testing.T) {
	for _, c := range []struct {
		name, code string
		want       bool
	}{
		{"AR_EF_25_10-X_Walls", "EF_25_10", true},
		{"AR_Ef_25_100_Walls", "EF_25_10", true}, // a prefix, as the code tables nest
		{"AR_EF_25_Walls", "EF_25_10", false},
		{"Ss40_15 Furniture", "SS_40_15", true},
		{"AR_Zz_20_10", "ZZ_", true},
		{"AR_ZZZ_20", "ZZ_", false},
		{"Notes ZZ", "ZZ_", false}, // a code needs a word after it
	} {
		if got := hasCode(Words(c.name), []string{c.code}); got != c.want {
			t.Errorf("hasCode(%q, %q) = %v", c.name, c.code, got)
		}
	}
}

// TestBuiltinsParse loads every built-in profile, which the program
// would otherwise only find broken on its first drawing.
func TestBuiltinsParse(t *testing.T) {
	for _, name := range BuiltinNames {
		p, ok := Builtin(name)
		if !ok || len(p.Rules) == 0 || p.Description == "" {
			t.Errorf("%s: %+v", name, p)
		}
	}
	if _, ok := Builtin("acme"); ok {
		t.Error("acme is no built-in")
	}
	if len(builtinRules()) < 40 {
		t.Errorf("only %d built-in rules", len(builtinRules()))
	}
}

// TestBuiltinConventions is the built-ins' precedence, name by name: the
// order a careful reader would apply.
func TestBuiltinConventions(t *testing.T) {
	for _, c := range []struct {
		layer  string
		role   Role
		source string
	}{
		{"A-WALL", Wall, "builtin:aia"},
		{"A-WALL-DIMS", Ignore, "builtin:generic"},      // dimensions, though it says WALL
		{"A-WALL-PATT", Wall, "builtin:aia"},            // a wall's poché is the wall
		{"A-GLAZ-WIND", Window, "builtin:aia"},          // windows before glazing
		{"A-FLOR-EVTR", Lift, "builtin:aia"},            // an AIA group the words don't know
		{"S-COLS", Structure, "builtin:aia"},            //
		{"Wände", Wall, "builtin:generic"},              //
		{"DRAWING", Unknown, ""},                        // DR is a word only on its own
		{"AR_EF_25_10-X_Existing Walls", Wall, "builtin:uniclass"},
		{"AR_EF_25_10_Glazed Partition", Glass, "builtin:uniclass"},
		{"AR_EF_25_30-X_Window", Window, "builtin:uniclass"},
		{"AR_EF_25_30-M_Internal Door", Door, "builtin:uniclass"},
		{"AR_Zz_20_10_70_65-T_Room Name", RoomName, "builtin:generic"}, // lettering before codes
		{"AR_Zz_20_10_90-T_Desk Number", DeskNumber, "builtin:generic"},
		{"AR_Zz_20_40-H_Hatching", Ignore, "builtin:uniclass"},
		{"AR_Zz_35-M_Setting Out", Annotation, "builtin:uniclass"},
		{"AR_Pr_40_20-X_Sanitary Fitting", Sanitary, "builtin:uniclass"},
		{"AR_Ss_40_15_Furniture Tags", Ignore, "builtin:uniclass"},
		{"AR_Ss_40_15_Text", Ignore, "builtin:uniclass"},
		{"EC1 Space Polygons", Area, "builtin:cafm"},
		{"EC1 Space Numbers", RoomNumber, "builtin:cafm"},
		{"EC1 Room Names", RoomName, "builtin:cafm"},
		{"RM$TXT", RoomName, "builtin:cafm"},
		{"GROS$", NotWall, "builtin:cafm"},
		{"EM$TXT", Ignore, "builtin:cafm"},
		{"I-WORKSTATION-TAG", DeskNumber, "builtin:generic"},
		{"LIGHTWELL", Void, "builtin:generic"},
		{"A-ANNO-TEXT", Annotation, "builtin:generic"},
		{"A-DOOR-TAG", Ignore, "builtin:generic"},
	} {
		m, ok := (*Set)(nil).Resolve(c.layer)
		if !ok {
			m.Role = Unknown
		}
		if m.Role != c.role || m.Source != c.source {
			t.Errorf("%q: %s from %q, want %s from %q", c.layer, m.Role, m.Source, c.role, c.source)
		}
	}
}

func TestClientOverrides(t *testing.T) {
	p, err := Parse("acme", []byte(`
description: Acme's drawings
rules:
  - layer: A220-Partitions*
    role: wall
  - words: [FITTINGS]
    role: furniture
  - layer: "A-WALL"
    role: glass
    priority: -1
  - layer: A-WALL
    role: not-wall
`))
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewSet("acme", p)
	if err != nil {
		t.Fatal(err)
	}
	for layer, want := range map[string]Match{
		"a220-partitions - new": {Wall, "profile:acme"},       // a glob, ignoring case
		"FITTINGS":              {Furniture, "profile:acme"},  // over the generic services
		"A-WALL":                {NotWall, "profile:acme"},    // priority orders a client's rules
		"A-DOOR":                {Door, "builtin:aia"},        // the rest falls through
		"X220-Partitions":       {},                           // no match
	} {
		got, _ := s.Resolve(layer)
		if got != want {
			t.Errorf("%q: %+v, want %+v", layer, got, want)
		}
	}
	if s, err := NewSet("none", nil); err != nil || len(s.client) != 0 {
		t.Errorf("no profile: %+v %v", s, err)
	}
	if _, err := NewSet("bad", &Profile{Rules: []Rule{{Role: "walls", Layer: "x"}}}); !errors.Is(err, ErrInvalid) {
		t.Errorf("a bad rule: %v", err)
	}
}

func TestStrictParsing(t *testing.T) {
	for doc, want := range map[string]string{
		"rules:\n  - role: wall\n    layr: A\n":             `unknown key "layr" (a rule's keys are role, layer`,
		"rule:\n  - role: wall\n":                           `unknown key "rule" (a profile's keys are description, rules)`,
		"rules:\n  - role: walls\n    layer: A\n":           `rule 1: unknown role "walls"; the roles are wall, structure`,
		"rules:\n  - role: wall\n":                          "rule 1: it matches nothing",
		"rules:\n  - role: wall\n    with: [NO]\n":          "with only narrows words",
		"rules:\n  - role: wall\n    words: ['*']\n":        "would match every layer",
		"rules:\n  - role: wall\n    words: ['']\n":         "would match every layer",
		"rules:\n  - role: wall\n    regex: '('\n":          "regex: error parsing regexp",
		"rules: []\n---\nrules: []\n":                       "one YAML document only",
		"rules: [":                                          "invalid profile",
		`{"rules": [{"role": "door", "layer": "x", "z": 1}]}`: `unknown key "z"`,
	} {
		_, err := Parse("acme.yaml", []byte(doc))
		if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, want %q", doc, err, want)
		}
	}
	p, err := Parse("empty", nil)
	if err != nil || len(p.Rules) != 0 {
		t.Errorf("an empty file is a profile with no rules yet: %+v %v", p, err)
	}
	// JSON, as the page sends it, is YAML too.
	p, err = Parse("page", []byte(`{"rules": [{"role": "wall", "layer": "Q-TRENN"}]}`))
	if err != nil || p.Rules[0].Layer != "Q-TRENN" {
		t.Errorf("JSON: %+v %v", p, err)
	}
}

func TestEncodeRoundTrip(t *testing.T) {
	in := &Profile{Description: "d", Rules: []Rule{
		{Role: Wall, Layer: `odd\*name`, Note: "why"},
		{Role: RoomName, Words: []string{"ROOM*"}, With: []string{"NAME"}, Priority: 5},
		{Role: Stairs, Codes: []string{"EF_35"}, Squashed: []string{"STAIRCORE"}, Regex: "^s"},
	}}
	b, err := in.Encode()
	if err != nil {
		t.Fatal(err)
	}
	out, err := Parse("again", b)
	if err != nil || !reflect.DeepEqual(in, out) {
		t.Fatalf("%s\n%+v %v", b, out, err)
	}
}

func TestGlob(t *testing.T) {
	for _, c := range []struct {
		glob, name string
		want       bool
	}{
		{"A-WALL*", "a-wall-full", true},
		{"A-WALL*", "XA-WALL", false},
		{"A?WALL", "A_WALL", true},
		{"Walls (new)", "WALLS (NEW)", true},
		{EscapeGlob("Odd*Name?"), "odd*name?", true},
		{EscapeGlob("Odd*Name?"), "OddXName!", false},
		{EscapeGlob(`back\slash`), `back\slash`, true},
		{`trailing\`, `trailing\`, true},
	} {
		p := &Profile{Rules: []Rule{{Role: Wall, Layer: c.glob}}}
		s, err := NewSet("g", p)
		if err != nil {
			t.Fatal(err)
		}
		m, _ := s.Resolve(c.name)
		if got := m.Source == "profile:g"; got != c.want {
			t.Errorf("%q vs %q: %v", c.glob, c.name, got)
		}
	}
}

func TestRolesAreKnown(t *testing.T) {
	if !Known(Wall) || Known(Unknown) || Known("walls") {
		t.Error("Known")
	}
	seen := map[Role]bool{}
	for _, r := range Roles {
		if seen[r.Role] || r.Label == "" || r.Help == "" {
			t.Errorf("%+v", r)
		}
		seen[r.Role] = true
	}
}

func TestLibrary(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "profiles")
	l := &Library{Dir: dir}
	if name, err := l.Active(); err != nil || name != "" {
		t.Fatalf("nothing in use yet: %q %v", name, err)
	}
	if s, name, err := l.Set(); err != nil || name != "" || len(s.client) != 0 {
		t.Fatalf("the built-ins alone: %v %q %v", s, name, err)
	}
	if _, err := l.Get("acme"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("not saved yet: %v", err)
	}
	if p, err := l.Get("uniclass"); err != nil || len(p.Rules) == 0 {
		t.Fatalf("a built-in reads by name: %v", err)
	}
	for _, bad := range []string{"../x", "Acme", "", ".hidden", strings.Repeat("a", 65)} {
		if _, err := l.Get(bad); !errors.Is(err, ErrName) {
			t.Errorf("Get(%q): %v", bad, err)
		}
	}
	if err := l.Put("generic", &Profile{}); !errors.Is(err, ErrBuiltin) {
		t.Fatalf("a built-in can't be overwritten: %v", err)
	}
	if err := l.Put("acme", &Profile{Rules: []Rule{{Role: "nope", Layer: "x"}}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("an invalid profile isn't saved: %v", err)
	}
	acme := &Profile{Rules: []Rule{{Role: Wall, Layer: "Q-TRENN"}}}
	if err := l.Put("acme", acme); err != nil {
		t.Fatal(err)
	}
	if err := l.Put("globex", &Profile{Rules: []Rule{}}); err != nil {
		t.Fatal(err)
	}
	if name, err := l.Active(); err != nil || name != "acme" {
		t.Fatalf("the first profile saved is the one in use: %q %v", name, err)
	}
	s, name, err := l.Set()
	if m, _ := s.Resolve("q-trenn"); err != nil || name != "acme" || m.Role != Wall {
		t.Fatalf("the saved rule applies: %+v %q %v", m, name, err)
	}
	if got, err := l.Get("acme"); err != nil || !reflect.DeepEqual(got, acme) {
		t.Fatalf("read back: %+v %v", got, err)
	}

	fixed := &Library{Dir: dir, Fixed: "initech"}
	if s, name, err := fixed.Set(); err != nil || name != "initech" || len(s.client) != 0 {
		t.Fatalf("a configured profile not saved yet adds nothing: %q %v", name, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "initech.yaml"), []byte("rules: [{role: wal, layer: x}]"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixed.Set(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("a broken file is an error, not ignored: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, activeFile), []byte("../etc\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Active(); !errors.Is(err, ErrName) {
		t.Fatalf("a tampered record: %v", err)
	}
	if _, _, err := l.Set(); !errors.Is(err, ErrName) {
		t.Fatalf("a tampered record: %v", err)
	}
	if err := l.Put("acme", acme); !errors.Is(err, ErrName) {
		t.Fatalf("saving checks the record too: %v", err)
	}
	if name, err := (*Library)(nil).Active(); err != nil || name != "" {
		t.Fatalf("no library: %q %v", name, err)
	}
}

func TestLibraryErrors(t *testing.T) {
	dir := t.TempDir()
	// A file where the folder should be.
	blocked := filepath.Join(dir, "file")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	l := &Library{Dir: blocked}
	if err := l.Put("acme", &Profile{}); err == nil {
		t.Error("the profiles folder can't be made")
	}
	// Unreadable record and profile: folders stand where the files should be.
	// Reading through a regular file instead would be "not a directory" on
	// Unix but "not found" on Windows, which is a missing profile, not an
	// error.
	if err := os.Mkdir(filepath.Join(dir, activeFile), 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := (&Library{Dir: dir}).Active(); err == nil {
		t.Error("the record can't be read")
	}
	if err := os.Mkdir(filepath.Join(dir, "acme.yaml"), 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := (&Library{Dir: dir}).Get("acme"); err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("the profile can't be read: %v", err)
	}
	l = &Library{Dir: dir, Fixed: "acme"}
	if err := l.Put("acme", &Profile{}); err == nil {
		t.Error("the profile can't replace a folder")
	}
	if _, _, err := l.Set(); err == nil {
		t.Error("the profile can't be read")
	}
	if err := writeFile(filepath.Join(dir, "missing", "x"), nil); err == nil {
		t.Error("no folder to write into")
	}
}
