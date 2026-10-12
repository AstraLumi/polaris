package main

import (
	"math"
	"testing"
)

// The grid is whole: every cell is where its centre says, hexagons have six
// neighbours and the twelve icosahedron corners five, and neighbours go both
// ways and sit about one cell apart.
func TestPlanetGrid(t *testing.T) {
	for _, n := range []int{2, 3, 5, 8, 13} {
		g := planetGrid{n}
		if g.cellCount() != 10*n*n+2 {
			t.Fatalf("n=%d: %d cells", n, g.cellCount())
		}
		pentagons := 0
		spacing := math.Atan(2) / float64(n)
		for c := 0; c < g.cellCount(); c++ {
			if got := g.cellAt(g.center(c)); got != c {
				t.Fatalf("n=%d: cell %d's centre is in cell %d", n, c, got)
			}
			nb := g.neighbors(c)
			switch len(nb) {
			case 5:
				pentagons++
			case 6:
			default:
				t.Fatalf("n=%d: cell %d has %d neighbours: %v", n, c, len(nb), nb)
			}
			for _, o := range nb {
				back := false
				for _, x := range g.neighbors(o) {
					back = back || x == c
				}
				if !back {
					t.Fatalf("n=%d: %d is next to %d but not the other way", n, c, o)
				}
				if a := math.Acos(math.Min(1, g.center(c).dot(g.center(o)))); a > spacing*1.6 || a < spacing*0.5 {
					t.Fatalf("n=%d: neighbours %d and %d are %.3f apart (a cell is %.3f)", n, c, o, a, spacing)
				}
			}
		}
		if pentagons != 12 {
			t.Errorf("n=%d: %d pentagons", n, pentagons)
		}
	}
}

// The cells are about the same size everywhere: neighbours near the
// pentagons sit nearly as far apart as in the middle of a face.
func TestPlanetGridIsEven(t *testing.T) {
	g := planetGrid{24}
	lo, hi := math.Inf(1), 0.0
	for c := 0; c < g.cellCount(); c++ {
		for _, o := range g.neighbors(c) {
			a := math.Acos(math.Min(1, g.center(c).dot(g.center(o))))
			lo, hi = math.Min(lo, a), math.Max(hi, a)
		}
	}
	if hi/lo > 1.2 {
		t.Errorf("neighbours are from %.4f to %.4f apart: %.2f times", lo, hi, hi/lo)
	}
}

// Fixed values the frontend's copy checks too (frontend/tests/planetGrid.test.mjs).
func TestPlanetGridFixtures(t *testing.T) {
	g := planetGrid{5}
	for _, c := range []struct {
		lat, lon float64
		cell     int
	}{
		{90, 0, 0}, {-90, 0, 1}, {0, 0, 228}, {45, 100, 34}, {-30, -120, 181}, {10, 179, 70},
	} {
		got := g.cellAt(fromLatLon(c.lat*math.Pi/180, c.lon*math.Pi/180))
		if got != c.cell {
			t.Errorf("%v°, %v°: cell %d, want %d", c.lat, c.lon, got, c.cell)
		}
	}
	if f := frequencyFor(40075, 50); f != 141 {
		t.Errorf("an Earth of 50 km hexes: frequency %d", f)
	}
}

func TestProjections(t *testing.T) {
	for _, name := range projectionNames {
		p := projectionByName(name)
		for _, ll := range [][2]float64{{0, 0}, {1.2, 0.5}, {-3, -1.1}, {2.5, 1.3}, {-0.4, 0.05}} {
			x, y := p.forward(ll[0], ll[1])
			lon, lat, ok := p.inverse(x, y)
			if !ok || math.Abs(lon-ll[0]) > 1e-6 || math.Abs(lat-ll[1]) > 1e-6 {
				t.Errorf("%s: %v -> (%.4f, %.4f) -> %.6f, %.6f, %v", name, ll, x, y, lon, lat, ok)
			}
		}
		if _, _, ok := p.inverse(10, 0); ok {
			t.Errorf("%s: a point far off the map has no place", name)
		}
	}
	if x, y := projectionByName("mollweide").forward(math.Pi, 0); math.Abs(x-2*math.Sqrt2) > 1e-9 || y != 0 {
		t.Errorf("mollweide edge: %v, %v", x, y)
	}
}
