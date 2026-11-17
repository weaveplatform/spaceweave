package profile

import (
	"cmp"
	"embed"
	"slices"
	"strings"
	"sync"
)

// BuiltinNames are the built-in profiles, in the order their rules are
// tried at equal priority: the conventions that say most precisely what a
// layer is come before the plain words that guess.
var BuiltinNames = []string{"uniclass", "aia", "cafm", "generic"}

//go:embed builtin/*.yaml
var builtinFS embed.FS

// builtins parses the embedded profiles once. They are part of the
// program, so a broken one is a bug that TestBuiltinsParse catches before
// it ships; panicking here only stops a broken build tracing anything.
var builtins = sync.OnceValue(func() map[string]*Profile {
	out := map[string]*Profile{}
	for _, name := range BuiltinNames {
		b, err := builtinFS.ReadFile("builtin/" + name + ".yaml")
		if err != nil {
			panic(err)
		}
		p, err := Parse(name, b)
		if err != nil {
			panic(err)
		}
		out[name] = p
	}
	return out
})

// Builtin returns a built-in profile, or false for any other name.
func Builtin(name string) (*Profile, bool) {
	p, ok := builtins()[name]
	return p, ok
}

// builtinRules are every built-in rule in the order they are tried.
var builtinRules = sync.OnceValue(func() []compiled {
	var all []compiled
	for _, name := range BuiltinNames {
		p, _ := Builtin(name)
		rules, err := p.compile(name, "builtin:"+name)
		if err != nil {
			panic(err) // Parse compiled them already
		}
		all = append(all, rules...)
	}
	// Stable, so profile order and then file order break ties.
	slices.SortStableFunc(
		all,
		func(a, b compiled) int { return cmp.Compare(b.priority, a.priority) },
	)
	return all
})

// Match is how a layer's role was decided.
type Match struct {
	Role Role
	// Source is "profile:<name>" for a client profile's rule and
	// "builtin:<name>" for a built-in's.
	Source string
}

// Set resolves layer names to roles. The zero Set, and a nil one, hold
// the built-ins only. A Set never changes, so it is safe to share.
type Set struct {
	client []compiled
}

// NewSet is a client profile's rules ahead of the built-ins'. A nil
// profile gives the built-ins alone.
func NewSet(name string, p *Profile) (*Set, error) {
	if p == nil {
		return &Set{}, nil
	}
	rules, err := p.compile(name, "profile:"+name)
	if err != nil {
		return nil, err
	}
	// A client's own priorities order its rules among themselves; all of
	// them still come before every built-in, which is what overriding
	// means.
	slices.SortStableFunc(
		rules,
		func(a, b compiled) int { return cmp.Compare(b.priority, a.priority) },
	)
	return &Set{client: rules}, nil
}

// Resolve finds a layer's role: the first rule that matches its name, or
// false when none does.
func (s *Set) Resolve(layer string) (Match, bool) {
	words := Words(layer)
	squashed := strings.Join(words, "")
	var client []compiled
	if s != nil {
		client = s.client
	}
	for _, rules := range [][]compiled{client, builtinRules()} {
		for i := range rules {
			if rules[i].matches(layer, words, squashed) {
				return Match{Role: rules[i].role, Source: rules[i].source}, true
			}
		}
	}
	return Match{}, false
}

// Guess is a Layer's Source when no rule matched it.
const Guess = "guess"

// Layer is how a drawing's layer was read, as report.json lists it: its
// role, how many entities are drawn on it (block references, and the
// layer-0 entities inside them, which take their reference's layer), and
// where the role came from: Match.Source, or Guess for a layer no
// rule matched, whose role is then what the tracer made of it (Unknown
// when nothing).
type Layer struct {
	Name     string `json:"name"`
	Role     Role   `json:"role"`
	Entities int    `json:"entities"`
	Source   string `json:"source"`
}
