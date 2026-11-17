package rasterfront

// recoverFaces keeps long thin bands or paired contours only when their
// ends join the already accepted wall network. It deliberately leaves free
// furniture edges alone. This is an experimental evidence source; the
// baseline extractor remains available for an ablation.
func (r *reader) recoverFaces(thinnest, reference int) []piece {
	candidates := make([]piece, 0, 32)
	for _, ax := range []axis{horizontal, vertical} {
		candidates = append(candidates, r.ink.pieces(ax, max(24, 12*reference))...)
	}
	var out []piece
	accepted := mask{w: r.w, h: r.h, px: make([]bool, len(r.ink.px))}
	for _, p := range candidates {
		if p.thick() >= thinnest || p.solid < minSolid {
			continue
		}
		lo, _, gl := r.anchored(p, p.lo, -1, accepted)
		hi, _, gh := r.anchored(p, p.hi, 1, accepted)
		if lo && hi && !gl && !gh {
			out = append(out, p)
		}
	}
	// Close a narrow strip between parallel, equally long faces, rather than
	// applying morphological closing to every gap in the drawing.
	for i, p := range candidates {
		for _, q := range candidates[i+1:] {
			if p.ax != q.ax || p.thick() > 2 || q.thick() > 2 {
				continue
			}
			if abs(p.mid()-q.mid()) < 2 || abs(p.mid()-q.mid()) > float64(3*reference) {
				continue
			}
			if abs(float64(p.lo-q.lo)) > 3 || abs(float64(p.hi-q.hi)) > 3 {
				continue
			}
			band := piece{
				ax:    p.ax,
				lo:    max(p.lo, q.lo),
				hi:    min(p.hi, q.hi),
				a0:    min(p.a0, q.a0),
				a1:    max(p.a1, q.a1),
				solid: 1,
			}
			lo, _, gl := r.anchored(band, band.lo, -1, accepted)
			hi, _, gh := r.anchored(band, band.hi, 1, accepted)
			if lo && hi && !gl && !gh {
				out = append(out, band)
			}
		}
	}
	for _, p := range out {
		paint(r.wall, p)
	}
	return merge(out)
}
