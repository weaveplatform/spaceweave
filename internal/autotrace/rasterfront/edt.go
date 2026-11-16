package rasterfront

import "math"

// inf stands for "no target pixel in reach". It is far above any squared
// distance on an image Read accepts, and small enough that inf plus a squared
// image dimension stays exact in the float64 the parabola intersections use.
const inf = 1 << 40

// sqDist is the exact squared Euclidean distance from every pixel of a w×h
// grid to the nearest pixel where target is true, by Felzenszwalb and
// Huttenlocher's separable lower-envelope method: linear in the pixel count
// and exact, where a chamfer approximation would misjudge diagonal strokes by
// a pixel and blur the line between a 2 px hairline and a 3 px wall. With
// border set, everything outside the grid counts as target too, so ink
// touching the edge still gets a finite distance. Pixels with no target in
// reach get inf.
func sqDist(w, h int, target []bool, border bool) []int {
	d := make([]int, w*h)
	n := max(w, h)
	f, out, v := make([]int, n), make([]int, n), make([]int, n)
	z := make([]float64, n+1)
	for x := range w {
		for y := range h {
			if target[y*w+x] {
				f[y] = 0
			} else {
				f[y] = inf
			}
		}
		envelope(f[:h], out[:h], v, z)
		for y := range h {
			d[y*w+x] = out[y]
		}
	}
	for y := range h {
		row := d[y*w : (y+1)*w]
		copy(f, row)
		envelope(f[:w], out[:w], v, z)
		for x := range w {
			e := min(out[x], inf)
			if border {
				// The nearest outside pixel is straight out through the
				// closest edge.
				b := min(x+1, w-x, y+1, h-y)
				e = min(e, b*b)
			}
			row[x] = e
		}
	}
	return d
}

// envelope is the one-dimensional transform: d[q] = min over p of
// (q-p)² + f[p]. v and z are scratch space for the parabolas of the lower
// envelope and the boundaries between them.
func envelope(f, d, v []int, z []float64) {
	n := len(f)
	if n == 0 {
		return
	}
	k := 0
	v[0] = 0
	z[0] = -float64(inf)
	z[1] = float64(inf)
	for q := 1; q < n; q++ {
		s := cross(f, q, v[k])
		for s <= z[k] {
			k--
			s = cross(f, q, v[k])
		}
		k++
		v[k] = q
		z[k] = s
		z[k+1] = float64(inf)
	}
	k = 0
	for q := range n {
		for z[k+1] < float64(q) {
			k++
		}
		dq := q - v[k]
		d[q] = dq*dq + f[v[k]]
	}
}

// cross is where the parabolas rooted at q and p (p < q) meet.
func cross(f []int, q, p int) float64 {
	return float64((f[q]+q*q)-(f[p]+p*p)) / float64(2*(q-p))
}

// open keeps the ink that a disc of radius k-1 fits inside, which is every
// stroke at least 2k-1 px wide, whole, and nothing of the thinner strokes,
// even where they touch a wall. dt is the squared distance from each pixel to
// paper. Cores are the pixels at least k from paper; growing them back by
// k-1 restores the wall bodies they came from. Reconstructing by flood fill
// instead would also restore every thin line attached to a wall.
func open(w, h int, dt []int, k int) []bool {
	core := make([]bool, len(dt))
	found := false
	for i, d := range dt {
		if d >= k*k {
			core[i] = true
			found = true
		}
	}
	if !found {
		return core
	}
	r2 := (k - 1) * (k - 1)
	grow := sqDist(w, h, core, false)
	out := make([]bool, len(dt))
	for i, d := range grow {
		out[i] = d <= r2 && dt[i] > 0
	}
	return out
}

// isqrt is the floor of the square root of a non-negative int.
func isqrt(n int) int {
	// The float root can be one off either way for large n; the integer
	// checks settle it.
	r := int(math.Sqrt(float64(n)))
	for r*r > n {
		r--
	}
	for (r+1)*(r+1) <= n {
		r++
	}
	return r
}
