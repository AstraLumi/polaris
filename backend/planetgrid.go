package main

import (
	"math"
	"sort"
)

// The planet's hex grid: an icosahedron whose 20 triangles are cut into a
// triangular lattice of frequency n and puffed out onto the unit sphere.
// The lattice is spread over each face so its points are evenly spaced
// along the face's edges (see warp), rather than projected straight from
// the flat face, which crowds them towards the corners.
// Every lattice point is a cell: hexagons everywhere except the 12 corners of
// the icosahedron, which are pentagons. There are 10n²+2 cells.
//
// The triangles pair up into 10 diamonds: five around the north pole and five
// around the south. A diamond has corners T (top), A (west), B (bottom) and C
// (east); the lattice point (i, j) sits i/n of the way from T towards A and
// j/n from T towards C. Each diamond owns the points with 1 ≤ i ≤ n and
// 0 ≤ j < n; the two poles are cells 0 (north) and 1 (south), and diamond d's
// point (i, j) is cell 2 + d·n² + (i−1)·n + j.
//
// frontend/src/planetGrid.js is the same grid; keep them in step.

type vec3 [3]float64

func (a vec3) add(b vec3) vec3      { return vec3{a[0] + b[0], a[1] + b[1], a[2] + b[2]} }
func (a vec3) sub(b vec3) vec3      { return vec3{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }
func (a vec3) scale(k float64) vec3 { return vec3{a[0] * k, a[1] * k, a[2] * k} }
func (a vec3) dot(b vec3) float64   { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }
func (a vec3) cross(b vec3) vec3 {
	return vec3{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}
func (a vec3) norm() vec3 { return a.scale(1 / math.Sqrt(a.dot(a))) }

func fromLatLon(lat, lon float64) vec3 {
	return vec3{math.Cos(lat) * math.Cos(lon), math.Cos(lat) * math.Sin(lon), math.Sin(lat)}
}

// latLon of a unit vector, in radians.
func latLon(v vec3) (lat, lon float64) {
	return math.Asin(math.Max(-1, math.Min(1, v[2]))), math.Atan2(v[1], v[0])
}

// The icosahedron: poles, an upper ring at latitude atan(1/2) every 72°
// from longitude 0, and a lower ring 36° round from it.
var (
	icoNorth = vec3{0, 0, 1}
	icoSouth = vec3{0, 0, -1}
	icoUpper [5]vec3
	icoLower [5]vec3
	// diamonds[d] = {T, A, B, C}
	diamonds [10][4]vec3
	// The 20 faces: their diamond, whether it is the upper (T, A, C) or the
	// lower (A, B, C) triangle, and the direction of their centre.
	icoFaces [20]struct {
		d      int
		upper  bool
		centre vec3
	}
)

func init() {
	ringLat := math.Atan(0.5)
	for k := 0; k < 5; k++ {
		icoUpper[k] = fromLatLon(ringLat, float64(k)*2*math.Pi/5)
		icoLower[k] = fromLatLon(-ringLat, float64(k)*2*math.Pi/5+math.Pi/5)
	}
	for k := 0; k < 5; k++ {
		diamonds[k] = [4]vec3{icoNorth, icoUpper[k], icoLower[k], icoUpper[(k+1)%5]}
		diamonds[5+k] = [4]vec3{icoUpper[(k+1)%5], icoLower[k], icoSouth, icoLower[(k+1)%5]}
	}
	for d := 0; d < 10; d++ {
		T, A, B, C := diamonds[d][0], diamonds[d][1], diamonds[d][2], diamonds[d][3]
		icoFaces[2*d].d, icoFaces[2*d].upper, icoFaces[2*d].centre = d, true, T.add(A).add(C).norm()
		icoFaces[2*d+1].d, icoFaces[2*d+1].upper, icoFaces[2*d+1].centre = d, false, A.add(B).add(C).norm()
	}
}

// icoEdge is the angle between two neighbouring corners of the icosahedron.
var icoEdge = math.Acos(1 / math.Sqrt(5))

// warp turns a barycentric coordinate on a face into the weight of that
// corner: along an edge, the weights sin(bα), sin((1−b)α) place a point as
// slerp does, at even angles, and inside a face they stay close to even.
// Projected straight (weight = b), cells near the twelve pentagons came out
// about 40% smaller than those in the middle of a face; this way the
// difference is under 20%. A cell's number and its neighbours don't change.
func warp(b float64) float64 { return math.Sin(b*icoEdge) / math.Sin(icoEdge) }

// unwarp is warp backwards: c are where a direction meets the flat face, as
// barycentric coordinates (summing to 1); it finds the b, also summing to 1,
// whose weights warp(b) are in the proportions of c. warp's sum grows with
// a common factor λ, so Newton's method finds λ, starting from the top
// (where the sum is at least 1) and coming down.
func unwarp(c [3]float64) [3]float64 {
	lam := math.Sin(icoEdge)
	asin := func(x float64) float64 { return math.Asin(math.Max(-1, math.Min(1, x))) }
	for range 40 {
		f, df := -1.0, 0.0
		for _, x := range c {
			y := math.Max(-0.999999, math.Min(0.999999, lam*x))
			f += asin(y) / icoEdge
			df += x / (icoEdge * math.Sqrt(1-y*y))
		}
		if df <= 0 {
			break
		}
		step := f / df
		lam -= step
		if math.Abs(step) < 1e-15 {
			break
		}
	}
	return [3]float64{asin(lam*c[0]) / icoEdge, asin(lam*c[1]) / icoEdge, asin(lam*c[2]) / icoEdge}
}

// planetGrid is the grid of one frequency.
type planetGrid struct{ n int }

func (g planetGrid) cellCount() int { return 10*g.n*g.n + 2 }

// lattice is the (unnormalised) point at s = i/n, t = j/n of diamond d; s
// and t may run past the diamond, which extends the face they fall in.
func (g planetGrid) lattice(d int, i, j float64) vec3 {
	T, A, B, C := diamonds[d][0], diamonds[d][1], diamonds[d][2], diamonds[d][3]
	s, t := i/float64(g.n), j/float64(g.n)
	if s+t <= 1 {
		return T.scale(warp(1 - s - t)).add(A.scale(warp(s))).add(C.scale(warp(t)))
	}
	return B.scale(warp(s + t - 1)).add(C.scale(warp(1 - s))).add(A.scale(warp(1 - t)))
}

// owner is the cell at lattice point (i, j) of diamond d, 0 ≤ i, j ≤ n:
// points on a diamond's edge belong to one of the diamonds sharing it.
func (g planetGrid) owner(d, i, j int) int {
	n := g.n
	for range 4 {
		k := d % 5
		north := d < 5
		switch {
		case i == 0 && j == 0:
			if north {
				return 0
			}
			d, i, j = (k+1)%5, n, 0
		case i == 0:
			if north {
				d, i, j = (k+1)%5, j, 0
			} else {
				d, i = (k+1)%5, n
			}
		case j == n:
			if north {
				d, j = 5+k, 0
			} else if i == n {
				return 1
			} else {
				d, i, j = 5+(k+1)%5, n, i
			}
		default:
			return 2 + d*n*n + (i-1)*n + j
		}
	}
	return 2 + d*n*n + (i-1)*n + j
}

// coords is where a cell sits: its diamond and lattice point (the poles
// answer d = -1).
func (g planetGrid) coords(cell int) (d, i, j int) {
	if cell < 2 {
		return -1, 0, 0
	}
	idx := cell - 2
	d = idx / (g.n * g.n)
	rem := idx % (g.n * g.n)
	return d, rem/g.n + 1, rem % g.n
}

// center is a cell's centre on the unit sphere.
func (g planetGrid) center(cell int) vec3 {
	switch cell {
	case 0:
		return icoNorth
	case 1:
		return icoSouth
	}
	d, i, j := g.coords(cell)
	return g.lattice(d, float64(i), float64(j)).norm()
}

// cellAt is the cell a direction falls in: the nearest lattice point on the
// icosahedron face the direction passes through.
func (g planetGrid) cellAt(v vec3) int {
	best, bestDot := 0, math.Inf(-1)
	for f := range icoFaces {
		if dt := icoFaces[f].centre.dot(v); dt > bestDot {
			best, bestDot = f, dt
		}
	}
	face := icoFaces[best]
	T, A, B, C := diamonds[face.d][0], diamonds[face.d][1], diamonds[face.d][2], diamonds[face.d][3]
	// Where the direction meets the face's plane.
	origin := T
	if !face.upper {
		origin = B
	}
	p := v.scale(face.centre.dot(origin) / face.centre.dot(v))
	var s, t float64
	if face.upper {
		a, b := solve2(A.sub(T), C.sub(T), p.sub(T))
		w := unwarp([3]float64{1 - a - b, a, b})
		s, t = w[1], w[2]
	} else {
		a, b := solve2(C.sub(B), A.sub(B), p.sub(B))
		w := unwarp([3]float64{1 - a - b, a, b})
		s, t = 1-w[1], 1-w[2]
	}
	i, j := axialRound(s*float64(g.n), t*float64(g.n))
	clamp := func(x int) int { return max(0, min(g.n, x)) }
	return g.owner(face.d, clamp(i), clamp(j))
}

// solve2 finds a, b with a·e1 + b·e2 = w (w in their plane).
func solve2(e1, e2, w vec3) (float64, float64) {
	a11, a12, a22 := e1.dot(e1), e1.dot(e2), e2.dot(e2)
	b1, b2 := w.dot(e1), w.dot(e2)
	det := a11*a22 - a12*a12
	return (b1*a22 - b2*a12) / det, (a11*b2 - a12*b1) / det
}

// axialRound rounds fractional axial coordinates to the nearest lattice
// point (the lattice's third axis is −a−b).
func axialRound(a, b float64) (int, int) {
	x, z := a, b
	y := -x - z
	rx, ry, rz := math.Round(x), math.Round(y), math.Round(z)
	dx, dy, dz := math.Abs(rx-x), math.Abs(ry-y), math.Abs(rz-z)
	if dx > dy && dx > dz {
		rx = -ry - rz
	} else if dy <= dz {
		rz = -rx - ry
	}
	return int(rx), int(rz)
}

// neighbors are the cells around a cell, in order round it (6 for a
// hexagon, 5 for a pentagon).
func (g planetGrid) neighbors(cell int) []int {
	n := g.n
	seen := map[int]bool{cell: true}
	var out []int
	add := func(c int) {
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	switch cell {
	case 0:
		for d := 0; d < 5; d++ {
			add(g.owner(d, 1, 0))
		}
	case 1:
		for d := 5; d < 10; d++ {
			add(g.owner(d, n, n-1))
		}
	default:
		d, i, j := g.coords(cell)
		for _, o := range [6][2]int{{1, 0}, {1, -1}, {0, -1}, {-1, 0}, {-1, 1}, {0, 1}} {
			ci, cj := i+o[0], j+o[1]
			if ci >= 0 && cj >= 0 && ci <= n && cj <= n {
				add(g.owner(d, ci, cj))
			} else {
				// Past the diamond: the lattice carried on across the edge
				// lands in the neighbouring cell.
				add(g.cellAt(g.lattice(d, float64(ci), float64(cj)).norm()))
			}
		}
	}
	// In order round the centre.
	c := g.center(cell)
	e1 := vec3{0, 0, 1}.cross(c)
	if e1.dot(e1) < 1e-12 {
		e1 = vec3{1, 0, 0}
	}
	e1 = e1.norm()
	e2 := c.cross(e1)
	angle := func(x int) float64 {
		v := g.center(x).sub(c)
		return math.Atan2(v.dot(e2), v.dot(e1))
	}
	sort.Slice(out, func(a, b int) bool { return angle(out[a]) < angle(out[b]) })
	return out
}

// frequencyFor is the grid frequency whose cells are about hexKm across on
// a planet planetKm round. Neighbouring icosahedron corners are atan(2)
// radians apart.
func frequencyFor(planetKm, hexKm float64) int {
	r := planetKm / (2 * math.Pi)
	n := int(math.Round(r * math.Atan(2) / hexKm))
	return max(minFrequency, min(maxFrequency, n))
}

// The grid's limits: below 2 the lattice has no inside; above 256 a planet
// has over 650,000 cells, more than a browser draws comfortably.
const (
	minFrequency = 2
	maxFrequency = 256
)

// hexKmOf is how wide a cell of frequency n is on a planet planetKm round.
func hexKmOf(planetKm float64, n int) float64 {
	return planetKm / (2 * math.Pi) * math.Atan(2) / float64(n)
}
