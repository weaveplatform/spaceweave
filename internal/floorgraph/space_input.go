package floorgraph

// Space is a named area read from a floor package's space plan: a
// facilities team's outline of a room or an open-plan zone, with the
// number and name written in it. A space
// plan's outlines are what the rooms are, before anything is inferred
// from the walls.
type Space struct {
	// Polygon is the outline in source-image pixels, placed on the
	// drawing traced.
	Polygon [][2]float64
	Name    string
	Number  string
	// Area is in square metres: the area written in the outline when
	// there is one, else the outline's own.
	Area float64
	// Kind is plan.KindRoom when walls or openings run along nearly all
	// of the outline, and plan.KindZone for an open-plan area that has
	// none of its own.
	Kind string
}

// SpaceSnap selects the registered-outline trial. Source boundaries are
// preserved by default: snapping reduced strict polygon recall in real trials.
type SpaceSnap int

const (
	SpaceSnapNone SpaceSnap = iota
	SpaceSnapWalls
	SpaceSnap300mm
)
