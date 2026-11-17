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
// Single leaves run 0.76–0.93 m (2'6"–3'0" in the US, 762–926 mm doorsets
// in the UK), most of them 0.82–0.9 m; 0.86 m is the middle of those, and
// with a dozen doors the median of what is drawn sits within a few per
// cent of it. 0.9 m, the top of the range, read every plan a few per cent
// small.
const (
	leafM      = 0.86
	partitionM = 0.15
	guessSpanM = 20.0
)

// Anisotropy: an image squashed on one axis shows it in its doors, which
// read wider along one axis than the other. Each axis needs minAxisDoors
// doors of its own (doors in one plan run 0.75–1 m, so a handful on each
// axis can differ by a fifth), and the two medians must differ by more than
// minSquash, or door-to-door variation (a few per cent between two small
// samples) would pass for a squash.
const (
	minAxisDoors = 8
	minSquash    = 0.06
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
	var byAxis [2][]float64
	for _, o := range ops {
		w := dist(o.A, o.B)
		switch o.Kind {
		case Door:
		case DoubleDoor:
			w /= 2
		case Gap:
			gaps = append(gaps, w)
			continue
		default:
			continue
		}
		doors = append(doors, w)
		d := sub(o.B, o.A)
		// Only doors on axis-aligned walls measure one axis alone.
		switch {
		case math.Abs(d.Y) <= 0.02*math.Abs(d.X):
			byAxis[0] = append(byAxis[0], w)
		case math.Abs(d.X) <= 0.02*math.Abs(d.Y):
			byAxis[1] = append(byAxis[1], w)
		}
	}
	uniform := func(pxPerM float64, source int, evidence string) scaleInfo {
		v := math.Round(pxPerM*1000) / 1000
		return scaleInfo{sx: v, sy: v, source: source, evidence: evidence}
	}
	if len(byAxis[0]) >= minAxisDoors && len(byAxis[1]) >= minAxisDoors {
		mx, my := median(byAxis[0]), median(byAxis[1])
		if math.Abs(mx/my-1) > minSquash {
			return scaleInfo{
				sx:     math.Round(mx/leafM*1000) / 1000,
				sy:     math.Round(my/leafM*1000) / 1000,
				source: scaleDoors,
				evidence: fmt.Sprintf(
					"assumed from %d door widths across and %d down (%.2f m per leaf); "+
						"they differ by %.0f%%, so the image is taken as squashed",
					len(byAxis[0]), len(byAxis[1]), leafM, 100*math.Abs(mx/my-1),
				),
			}
		}
	}
	if len(doors) > 0 {
		return uniform(
			median(doors)/leafM,
			scaleDoors,
			fmt.Sprintf(
				"assumed from %s (%.2f m per leaf)",
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
				"assumed from %s (%.2f m each)",
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
