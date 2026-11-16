package cad

// samplesPerSpan is how many points each knot span of a spline is drawn
// with; enough for the gentle curves of a floor plan at plan scale.
const samplesPerSpan = 8

// Points samples the spline as a polyline. A spline with only fit points
// (some exporters write no control points) is drawn through them.
func (s *Spline) Points() []Point {
	ctrl := s.Control
	if len(ctrl) < 2 {
		return s.Fit
	}
	p := s.Degree
	if p < 1 || p >= len(ctrl) {
		p = min(3, len(ctrl)-1)
	}
	knots := s.Knots
	if len(knots) != len(ctrl)+p+1 {
		knots = clamped(len(ctrl), p)
	}
	lo, hi := knots[p], knots[len(ctrl)]
	if hi <= lo {
		return ctrl
	}
	n := (len(ctrl) - p) * samplesPerSpan
	out := make([]Point, 0, n+1)
	for i := 0; i <= n; i++ {
		u := lo + (hi-lo)*float64(i)/float64(n)
		out = append(out, s.deBoor(knots, p, u))
	}
	return out
}

// clamped is a uniform knot vector that starts and ends on the end control
// points.
func clamped(n, p int) []float64 {
	k := make([]float64, n+p+1)
	for i := range k {
		switch {
		case i <= p:
			k[i] = 0
		case i >= n:
			k[i] = float64(n - p)
		default:
			k[i] = float64(i - p)
		}
	}
	return k
}

// deBoor evaluates the spline at u, honouring weights when the spline is
// rational.
func (s *Spline) deBoor(t []float64, p int, u float64) Point {
	ctrl := s.Control
	k := p
	for k < len(ctrl)-1 && u >= t[k+1] {
		k++
	}
	type hp struct{ x, y, w float64 }
	d := make([]hp, p+1)
	for j := range d {
		c := ctrl[j+k-p]
		w := 1.0
		if len(s.Weights) == len(ctrl) && s.Weights[j+k-p] > 0 {
			w = s.Weights[j+k-p]
		}
		d[j] = hp{c.X * w, c.Y * w, w}
	}
	for r := 1; r <= p; r++ {
		for j := p; j >= r; j-- {
			den := t[j+1+k-r] - t[j+k-p]
			a := 0.0
			if den != 0 {
				a = (u - t[j+k-p]) / den
			}
			d[j] = hp{
				(1-a)*d[j-1].x + a*d[j].x,
				(1-a)*d[j-1].y + a*d[j].y,
				(1-a)*d[j-1].w + a*d[j].w,
			}
		}
	}
	return Point{X: d[p].x / d[p].w, Y: d[p].y / d[p].w}
}
