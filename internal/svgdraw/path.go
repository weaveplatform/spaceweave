package svgdraw

import (
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/weaveplatform/spaceweave/internal/vector"
)

var errPathData = errors.New("bad path data")

// pathScanner reads numbers and command letters from SVG path data, where
// "1-2", "1.5.5" and "1e-3" are all legal ways to run numbers together.
type pathScanner struct {
	s string
	i int
}

func (sc *pathScanner) skip() {
	for sc.i < len(sc.s) && (sc.s[sc.i] == ' ' || sc.s[sc.i] == ',' || sc.s[sc.i] == '\t' ||
		sc.s[sc.i] == '\n' || sc.s[sc.i] == '\r') {
		sc.i++
	}
}

func (sc *pathScanner) command() (byte, bool) {
	sc.skip()
	if sc.i < len(sc.s) && strings.IndexByte("MmLlHhVvCcSsQqTtAaZz", sc.s[sc.i]) >= 0 {
		sc.i++
		return sc.s[sc.i-1], true
	}
	return 0, false
}

func (sc *pathScanner) hasNumber() bool {
	sc.skip()
	if sc.i >= len(sc.s) {
		return false
	}
	c := sc.s[sc.i]
	return c == '-' || c == '+' || c == '.' || (c >= '0' && c <= '9')
}

func (sc *pathScanner) number() (float64, error) {
	sc.skip()
	start := sc.i
	if sc.i < len(sc.s) && (sc.s[sc.i] == '-' || sc.s[sc.i] == '+') {
		sc.i++
	}
	dot, exp := false, false
	for sc.i < len(sc.s) {
		c := sc.s[sc.i]
		switch {
		case c >= '0' && c <= '9':
		case c == '.' && !dot && !exp:
			dot = true
		case (c == 'e' || c == 'E') && !exp:
			exp = true
			if sc.i+1 < len(sc.s) && (sc.s[sc.i+1] == '-' || sc.s[sc.i+1] == '+') {
				sc.i++
			}
		default:
			return parseNum(sc.s[start:sc.i])
		}
		sc.i++
	}
	return parseNum(sc.s[start:sc.i])
}

func parseNum(s string) (float64, error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, errPathData
	}
	return f, nil
}

// flag reads an arc flag, which may be written with no separator ("a1 1 0 01 1 1").
func (sc *pathScanner) flag() (bool, error) {
	sc.skip()
	if sc.i < len(sc.s) && (sc.s[sc.i] == '0' || sc.s[sc.i] == '1') {
		sc.i++
		return sc.s[sc.i-1] == '1', nil
	}
	return false, errPathData
}

func (sc *pathScanner) numbers(n int) ([]float64, error) {
	out := make([]float64, n)
	for i := range out {
		v, err := sc.number()
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

// parsePath reads SVG path data into p. Like browsers, it keeps what it read
// before an error.
//
//nolint:gocyclo // one case per path command
func parsePath(d string, p *vector.Path) {
	sc := &pathScanner{s: d}
	var cur, start, ctrl vector.Point
	var prev byte
	cmd, ok := sc.command()
	if !ok {
		return
	}
	for {
		rel := cmd >= 'a'
		off := vector.Point{}
		if rel {
			off = cur
		}
		var err error
		switch cmd | 0x20 {
		case 'm':
			var v []float64
			if v, err = sc.numbers(2); err == nil {
				cur = off.Add(vector.Point{X: v[0], Y: v[1]})
				start = cur
				p.MoveTo(cur)
				// Further pairs after a moveto are implicit linetos.
				cmd = map[bool]byte{true: 'l', false: 'L'}[rel]
				prev = 'm'
				goto next
			}
		case 'l':
			var v []float64
			if v, err = sc.numbers(2); err == nil {
				cur = off.Add(vector.Point{X: v[0], Y: v[1]})
				p.LineTo(cur)
			}
		case 'h':
			var v float64
			if v, err = sc.number(); err == nil {
				cur.X = off.X + v
				if !rel {
					cur.X = v
				}
				p.LineTo(cur)
			}
		case 'v':
			var v float64
			if v, err = sc.number(); err == nil {
				cur.Y = off.Y + v
				if !rel {
					cur.Y = v
				}
				p.LineTo(cur)
			}
		case 'c':
			var v []float64
			if v, err = sc.numbers(6); err == nil {
				c1 := off.Add(vector.Point{X: v[0], Y: v[1]})
				ctrl = off.Add(vector.Point{X: v[2], Y: v[3]})
				cur = off.Add(vector.Point{X: v[4], Y: v[5]})
				p.CubicTo(c1, ctrl, cur)
			}
		case 's':
			var v []float64
			if v, err = sc.numbers(4); err == nil {
				c1 := cur
				if prev == 'c' || prev == 's' {
					c1 = cur.Mul(2).Sub(ctrl)
				}
				ctrl = off.Add(vector.Point{X: v[0], Y: v[1]})
				end := off.Add(vector.Point{X: v[2], Y: v[3]})
				p.CubicTo(c1, ctrl, end)
				cur = end
			}
		case 'q':
			var v []float64
			if v, err = sc.numbers(4); err == nil {
				ctrl = off.Add(vector.Point{X: v[0], Y: v[1]})
				end := off.Add(vector.Point{X: v[2], Y: v[3]})
				quad(p, cur, ctrl, end)
				cur = end
			}
		case 't':
			var v []float64
			if v, err = sc.numbers(2); err == nil {
				c := cur
				if prev == 'q' || prev == 't' {
					c = cur.Mul(2).Sub(ctrl)
				}
				ctrl = c
				end := off.Add(vector.Point{X: v[0], Y: v[1]})
				quad(p, cur, c, end)
				cur = end
			}
		case 'a':
			cur, err = arcCommand(sc, p, cur, off)
		case 'z':
			p.Close()
			cur = start
			prev = 'z'
			if c, ok := sc.command(); ok {
				cmd = c
				continue
			}
			return
		}
		if err != nil {
			return
		}
		prev = cmd | 0x20
	next:
		if sc.hasNumber() {
			continue
		}
		c, ok := sc.command()
		if !ok {
			return
		}
		cmd = c
	}
}

func quad(p *vector.Path, a, c, b vector.Point) {
	p.CubicTo(a.Add(c.Sub(a).Mul(2.0/3)), b.Add(c.Sub(b).Mul(2.0/3)), b)
}

func arcCommand(sc *pathScanner, p *vector.Path, cur, off vector.Point) (vector.Point, error) {
	v, err := sc.numbers(3)
	if err != nil {
		return cur, err
	}
	large, err := sc.flag()
	if err != nil {
		return cur, err
	}
	sweep, err := sc.flag()
	if err != nil {
		return cur, err
	}
	e, err := sc.numbers(2)
	if err != nil {
		return cur, err
	}
	end := off.Add(vector.Point{X: e[0], Y: e[1]})
	arcTo(p, cur, end, v[0], v[1], v[2]*math.Pi/180, large, sweep)
	return end, nil
}

// arcTo adds an elliptical arc given by its end points, converted to centre
// form as in the SVG specification's implementation notes (appendix B.2.4).
func arcTo(p *vector.Path, a, b vector.Point, rx, ry, phi float64, large, sweep bool) {
	rx, ry = math.Abs(rx), math.Abs(ry)
	if rx == 0 || ry == 0 || (a.X == b.X && a.Y == b.Y) {
		p.LineTo(b)
		return
	}
	sin, cos := math.Sincos(phi)
	dx, dy := (a.X-b.X)/2, (a.Y-b.Y)/2
	x1 := cos*dx + sin*dy
	y1 := -sin*dx + cos*dy
	// Radii too small to reach are scaled up until they just do.
	if l := x1*x1/(rx*rx) + y1*y1/(ry*ry); l > 1 {
		s := math.Sqrt(l)
		rx, ry = rx*s, ry*s
	}
	num := rx*rx*ry*ry - rx*rx*y1*y1 - ry*ry*x1*x1
	den := rx*rx*y1*y1 + ry*ry*x1*x1
	k := math.Sqrt(math.Max(0, num/den))
	if large == sweep {
		k = -k
	}
	cx1, cy1 := k*rx*y1/ry, -k*ry*x1/rx
	c := vector.Point{X: cos*cx1 - sin*cy1 + (a.X+b.X)/2, Y: sin*cx1 + cos*cy1 + (a.Y+b.Y)/2}
	angle := func(ux, uy, vx, vy float64) float64 {
		return math.Atan2(ux*vy-uy*vx, ux*vx+uy*vy)
	}
	t0 := angle(1, 0, (x1-cx1)/rx, (y1-cy1)/ry)
	dt := angle((x1-cx1)/rx, (y1-cy1)/ry, (-x1-cx1)/rx, (-y1-cy1)/ry)
	if !sweep && dt > 0 {
		dt -= 2 * math.Pi
	} else if sweep && dt < 0 {
		dt += 2 * math.Pi
	}
	major := vector.Point{X: rx * cos, Y: rx * sin}
	p.EllipseArc(c, major, ry/rx, t0, t0+dt, true)
	p.Segs[len(p.Segs)-1].P[2] = b
}
