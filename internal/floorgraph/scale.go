package floorgraph

import (
	"fmt"
	"math"
	"slices"
)

// Where the scale came from, best first.
const (
	scaleExact = iota
	scaleMeasured
	scaleDoors
	scaleGaps
	scaleThickness
	scaleGuess
)

// Sizes the estimate assumes, metres: a door leaf, a plain doorway, an
// interior partition, and the span of a building with nothing else to go on.
const (
	leafM      = 0.9
	partitionM = 0.15
	guessSpanM = 20.0
)

type scaleInfo struct {
	sx, sy   float64
	source   int
	evidence string
}

func median(vs []float64) float64 {
	s := slices.Clone(vs)
	slices.Sort(s)
	if n := len(s); n%2 == 1 {
		return s[n/2]
	}
	return (s[len(s)/2-1] + s[len(s)/2]) / 2
}

// estimateScale uses the front end's scale when it has one. Otherwise, in
// order: door leaves are 0.9 m; plain doorways of a door-like width are 0.9
// m; the median interior wall (thickness px, weighted by length) is 0.15 m;
// the walls span 20 m.
func estimateScale(
	s Scale,
	ops []opening,
	interiorT, interiorL []float64,
	extent float64,
) scaleInfo {
	if s.Known() && finite(s.PxPerMetre[0], s.PxPerMetre[1]) {
		info := scaleInfo{
			sx:       s.PxPerMetre[0],
			sy:       s.PxPerMetre[1],
			source:   scaleMeasured,
			evidence: s.Evidence,
		}
		if s.Exact {
			info.source = scaleExact
			if info.evidence == "" {
				info.evidence = "the drawing's units"
			}
		} else if info.evidence == "" {
			info.evidence = "measured by the front end"
		}
		return info
	}
	var doors, gaps []float64
	for _, o := range ops {
		w := dist(o.A, o.B)
		switch o.Kind {
		case Door:
			doors = append(doors, w)
		case DoubleDoor:
			doors = append(doors, w/2)
		case Gap:
			gaps = append(gaps, w)
		}
	}
	uniform := func(pxPerM float64, source int, evidence string) scaleInfo {
		v := math.Round(pxPerM*1000) / 1000
		return scaleInfo{sx: v, sy: v, source: source, evidence: evidence}
	}
	if len(doors) > 0 {
		return uniform(
			median(doors)/leafM,
			scaleDoors,
			fmt.Sprintf(
				"assumed from %s (%.1f m per leaf)",
				count(len(doors), "door width", "door widths"),
				leafM,
			),
		)
	}
	if len(gaps) > 0 {
		// Doorways are 0.7–1.2 m; wider gaps (archways, open sides of a
		// room) would skew the median, so only those near it count.
		m := median(gaps)
		var band []float64
		for _, g := range gaps {
			if g >= m*0.7/leafM && g <= m*1.2/leafM {
				band = append(band, g)
			}
		}
		if len(band) == 0 {
			// An even count split wide apart: the median falls between.
			band = gaps
		}
		return uniform(
			median(band)/leafM,
			scaleGaps,
			fmt.Sprintf(
				"assumed from %s (%.1f m each)",
				count(len(band), "doorway width", "doorway widths"),
				leafM,
			),
		)
	}
	if len(interiorT) > 0 {
		return uniform(weightedMedian(interiorT, interiorL)/partitionM, scaleThickness,
			fmt.Sprintf("assumed from the median interior wall thickness (%.2f m)", partitionM))
	}
	return uniform(math.Max(extent, 1)/guessSpanM, scaleGuess,
		fmt.Sprintf("guessed: the walls taken to span %.0f m", guessSpanM))
}

func count(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}
