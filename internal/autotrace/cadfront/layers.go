package cadfront

import (
	"cmp"
	"slices"

	"github.com/weaveplatform/spaceweave/internal/autotrace/profile"
	"github.com/weaveplatform/spaceweave/internal/cad"
)

// facets are what the reader takes from a layer's role. A profile names
// one role per layer; each part of the reader asks its own question of
// it (may its lines be walls? may its text name a room?), and this is
// where the answers live.
type facets struct {
	// lines is what the layer's loose lines are, and what a block on it
	// is when the block's own name says nothing.
	lines role
	// text is what the layer's text says about rooms.
	text textClass
	// sanitary says furniture on it is sanitary fittings, which tells a
	// WC from a chair by shape.
	sanitary bool
	// desks says its text numbers desks.
	desks bool
	// area says its closed outlines are rooms (see areaRooms), and void
	// that they are holes in the floor (see findVoids).
	area, void bool
	// inert says its lines bound nothing, so they aren't offered as a
	// void's edge either: dimensions, grids, text.
	inert bool
}

// roleFacets must give every role in profile.Roles; TestRoleFacets
// checks. A layer no rule matches gets the zero facets: its lines are
// walls if nothing else is (roleNone) and its text names a room if it
// looks like a name.
var roleFacets = map[profile.Role]facets{
	profile.Wall:       {lines: roleWall},
	profile.Structure:  {lines: roleWall},
	profile.Glass:      {lines: roleGlass},
	profile.Door:       {lines: roleDoor, text: textNever},
	profile.Window:     {lines: roleWindow, text: textNever},
	profile.Furniture:  {lines: roleFurniture, text: textNever},
	profile.Sanitary:   {lines: roleFurniture, text: textNever, sanitary: true},
	profile.RoomName:   {lines: roleNotWall, text: textName, inert: true},
	profile.RoomNumber: {lines: roleNotWall, text: textNumber, inert: true},
	profile.DeskNumber: {lines: roleNotWall, text: textNever, desks: true, inert: true},
	profile.Area:       {lines: roleNotWall, area: true},
	profile.Void:       {lines: roleNotWall, void: true},
	profile.Stairs:     {lines: roleNotWall, text: textNever},
	profile.Lift:       {lines: roleNotWall, text: textNever},
	profile.Services:   {lines: roleNotWall, text: textNever},
	profile.Annotation: {lines: roleNotWall, inert: true},
	profile.Ignore:     {lines: roleNotWall, text: textNever, inert: true},
	profile.NotWall:    {lines: roleNotWall},
}

// layerInfo is one layer: its role, and how much is drawn on it.
type layerInfo struct {
	facets
	name     string
	match    profile.Match
	matched  bool
	entities int
}

// layer looks a layer's role up once per drawing; a drawing has tens of
// layers and hundreds of thousands of entities on them.
func (r *reader) layer(name string) *layerInfo {
	if li, ok := r.byLayer[name]; ok {
		return li
	}
	li := &layerInfo{name: name}
	li.match, li.matched = r.o.Profile.Resolve(name)
	if li.matched {
		li.facets = roleFacets[li.match.Role]
	}
	r.byLayer[name] = li
	return li
}

// guessOrder breaks ties between guesses, most telling first.
var guessOrder = []profile.Role{
	profile.Wall, profile.Door, profile.Window, profile.Furniture, profile.RoomName,
}

// layerReport says how each layer was read, sorted by name. A layer no
// rule matched is reported by what was made of it: a guess, which the
// reviewer can confirm or correct by giving it a role.
func (r *reader) layerReport() []profile.Layer {
	guesses := map[*layerInfo]map[profile.Role]int{}
	vote := func(li *layerInfo, role profile.Role) {
		if li.matched {
			return
		}
		if guesses[li] == nil {
			guesses[li] = map[profile.Role]int{}
		}
		guesses[li][role]++
	}
	for _, n := range r.nodes {
		if n.top != n {
			continue
		}
		switch n.role {
		case roleDoor:
			vote(n.info, profile.Door)
		case roleWindow:
			vote(n.info, profile.Window)
		case roleFurniture:
			vote(n.info, profile.Furniture)
		}
	}
	for i := range r.leaves {
		l := &r.leaves[i]
		if r.consumed(l) || !l.used {
			continue
		}
		// What uses a loose entity on a layer no rule names: a wall
		// pairing its lines, a loose door its arc and leaf, a label its
		// text. Text read but naming nothing is no evidence either way.
		switch l.e.(type) {
		case *cad.Arc:
			vote(l.info, profile.Door)
		case *cad.Line, *cad.Polyline, *cad.Circle, *cad.Hatch, *cad.Solid:
			if l.doorLeaf {
				vote(l.info, profile.Door)
			} else {
				vote(l.info, profile.Wall)
			}
		default:
			if l.labelled {
				vote(l.info, profile.RoomName)
			}
		}
	}
	out := make([]profile.Layer, 0, len(r.byLayer))
	for _, li := range r.byLayer {
		l := profile.Layer{
			Name:     li.name,
			Entities: li.entities,
			Role:     profile.Unknown,
			Source:   profile.Guess,
		}
		if li.matched {
			l.Role, l.Source = li.match.Role, li.match.Source
		} else if g := guesses[li]; len(g) > 0 {
			best := 0
			for _, role := range guessOrder {
				if g[role] > best {
					l.Role, best = role, g[role]
				}
			}
		}
		out = append(out, l)
	}
	slices.SortFunc(out, func(a, b profile.Layer) int { return cmp.Compare(a.Name, b.Name) })
	return out
}
