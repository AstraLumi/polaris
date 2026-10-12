// Command skygen draws Polaris's default sky: the picture behind the planet
// in the 3D map, and the template a story's own sky must follow.
//
//	cd backend && go run ./cmd/skygen ../frontend/public/sky/default-sky.jpg
//
// The format (frontend/src/components/PlanetGlobe.vue reads it so): one
// equirectangular image, twice as wide as it is tall, of the sky as seen from
// the planet. The top edge is the sky above the north pole (the north
// celestial pole), the bottom edge the sky above the south pole, the middle
// row the celestial equator. It is the sky as seen from the ground, like a
// photo of it: the middle column lies above longitude 0, east is to the left
// and west to the right, and the left and right edges meet, so they must
// match.
//
// Every pixel is worked out from its direction on the sky, so the picture is
// seamless and stars stay round near the poles, where the image stretches.
// The sky is quiet on purpose: a faint galaxy, a few thousand stars, two
// moons, and Ursa Minor round the pole for anyone who looks.
package main

import (
	"fmt"
	"image"
	"image/jpeg"
	"math"
	"math/rand"
	"os"
	"runtime"
	"sync"
)

const (
	width  = 8192
	height = 4096
)

type vec struct{ x, y, z float64 }

func (a vec) dot(b vec) float64   { return a.x*b.x + a.y*b.y + a.z*b.z }
func (a vec) add(b vec) vec       { return vec{a.x + b.x, a.y + b.y, a.z + b.z} }
func (a vec) scale(k float64) vec { return vec{a.x * k, a.y * k, a.z * k} }
func (a vec) cross(b vec) vec {
	return vec{a.y*b.z - a.z*b.y, a.z*b.x - a.x*b.z, a.x*b.y - a.y*b.x}
}
func (a vec) norm() vec { return a.scale(1 / math.Sqrt(a.dot(a))) }

// The direction of sky longitude lon, latitude lat (radians); z is the
// planet's axis, x longitude 0.
func dir(lon, lat float64) vec {
	return vec{math.Cos(lat) * math.Cos(lon), math.Cos(lat) * math.Sin(lon), math.Sin(lat)}
}

// pixel is where a direction lands: x grows westwards from the middle
// column (longitude 0; east is to the left, as the sky looks from the
// ground), y from the north pole down.
func pixel(lon, lat float64) (float64, float64) {
	x := (0.5 - lon/(2*math.Pi)) * width
	x = math.Mod(math.Mod(x, width)+width, width)
	return x, (0.5 - lat/math.Pi) * height
}

func lonLatOf(px, py float64) (float64, float64) {
	return (0.5 - px/width) * 2 * math.Pi, (0.5 - py/height) * math.Pi
}

// ---- Noise on the sphere (3D value noise, so it has no seams) ---------------

type noise3 struct {
	perm [512]int
	vals [256]float64
}

func newNoise(seed int64) *noise3 {
	r := rand.New(rand.NewSource(seed))
	n := &noise3{}
	p := r.Perm(256)
	for i := 0; i < 512; i++ {
		n.perm[i] = p[i&255]
	}
	for i := range n.vals {
		n.vals[i] = r.Float64()
	}
	return n
}

func (n *noise3) at(x, y, z float64) float64 {
	xi, yi, zi := math.Floor(x), math.Floor(y), math.Floor(z)
	fx, fy, fz := x-xi, y-yi, z-zi
	s := func(t float64) float64 { return t * t * (3 - 2*t) }
	ux, uy, uz := s(fx), s(fy), s(fz)
	X, Y, Z := int(xi)&255, int(yi)&255, int(zi)&255
	v := func(i, j, k int) float64 {
		return n.vals[n.perm[n.perm[n.perm[(X+i)&255]+(Y+j)&255]+(Z+k)&255]]
	}
	lerp := func(a, b, t float64) float64 { return a + (b-a)*t }
	return lerp(
		lerp(lerp(v(0, 0, 0), v(1, 0, 0), ux), lerp(v(0, 1, 0), v(1, 1, 0), ux), uy),
		lerp(lerp(v(0, 0, 1), v(1, 0, 1), ux), lerp(v(0, 1, 1), v(1, 1, 1), ux), uy), uz)
}

func (n *noise3) fbm(d vec, freq float64, octaves int) float64 {
	sum, amp, norm := 0.0, 1.0, 0.0
	for o := 0; o < octaves; o++ {
		sum += amp * n.at(d.x*freq+11.3, d.y*freq+7.1, d.z*freq+3.7)
		norm += amp
		amp *= 0.5
		freq *= 2
	}
	return sum / norm
}

// ---- The picture ----------------------------------------------------------------

type canvas struct{ r, g, b []float32 }

func (c *canvas) add(x, y int, k float64, col [3]float64) {
	x = ((x % width) + width) % width
	if y < 0 || y >= height {
		return
	}
	i := y*width + x
	c.r[i] += float32(k * col[0])
	c.g[i] += float32(k * col[1])
	c.b[i] += float32(k * col[2])
}

// background: a dark sky with a faint band of galaxy along a tilted great
// circle, and a few dim clouds of colour.
func background(c *canvas) {
	gal := newNoise(7)
	dust := newNoise(21)
	tint := newNoise(42)
	// The galaxy's plane, tilted 62° to the equator.
	pole := dir(1.9, math.Pi/2-62*math.Pi/180)
	var wg sync.WaitGroup
	rows := make(chan int, height)
	for y := 0; y < height; y++ {
		rows <- y
	}
	close(rows)
	for w := 0; w < runtime.NumCPU(); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for y := range rows {
				for x := 0; x < width; x++ {
					lon, lat := lonLatOf(float64(x)+0.5, float64(y)+0.5)
					d := dir(lon, lat)
					off := math.Asin(math.Max(-1, math.Min(1, d.dot(pole)))) // angle from the galaxy's plane
					band := math.Exp(-off * off / (2 * 0.16 * 0.16))
					core := math.Exp(-off * off / (2 * 0.05 * 0.05))
					g := gal.fbm(d, 3, 6)
					lanes := dust.fbm(d, 9, 5) // dark dust lanes along the band
					glow := band*(0.55*g+0.15) + core*0.35*g
					glow *= 1 - 0.75*math.Max(0, lanes-0.45)/0.55*core
					glow = math.Max(0, glow)
					t := tint.fbm(d, 1.6, 4)
					// Base night, the galaxy's grey-blue glow, and a hint of
					// violet and teal where the tint noise is high.
					r := 0.012 + glow*0.085 + math.Max(0, t-0.62)*0.05
					gg := 0.016 + glow*0.090 + math.Max(0, 0.40-t)*0.035
					b := 0.030 + glow*0.115 + math.Max(0, t-0.55)*0.06
					i := y*width + x
					c.r[i], c.g[i], c.b[i] = float32(r), float32(gg), float32(b)
				}
			}
		}()
	}
	wg.Wait()
}

// star adds a round point of light at (lon, lat): a Gaussian of the given
// angular size, widened across the image by 1/cos(lat) so it stays round on
// the sky.
func star(c *canvas, lon, lat, sizeDeg, brightness float64, col [3]float64) {
	center := dir(lon, lat)
	px, py := pixel(lon, lat)
	sigma := sizeDeg * math.Pi / 180
	ry := int(math.Ceil(4*sigma/math.Pi*height)) + 1
	stretch := 1 / math.Max(math.Cos(lat), 1e-3)
	rx := min(width/2, int(math.Ceil(float64(ry)*stretch))+1)
	for dy := -ry; dy <= ry; dy++ {
		y := int(py) + dy
		if y < 0 || y >= height {
			continue
		}
		for dx := -rx; dx <= rx; dx++ {
			x := int(px) + dx
			lo, la := lonLatOf(float64(x)+0.5, float64(y)+0.5)
			a := math.Acos(math.Max(-1, math.Min(1, dir(lo, la).dot(center))))
			k := brightness * math.Exp(-a*a/(2*sigma*sigma))
			if k > 1e-4 {
				c.add(x, y, k, col)
			}
		}
	}
}

// Star colours, from blue-white to a soft orange, muted.
var starColors = [][3]float64{
	{0.78, 0.86, 1.0}, {0.90, 0.94, 1.0}, {1.0, 1.0, 1.0}, {1.0, 0.96, 0.86}, {1.0, 0.88, 0.72}, {1.0, 0.80, 0.64},
}

func stars(c *canvas) {
	r := rand.New(rand.NewSource(1977))
	for n := 0; n < 5200; n++ {
		lon := r.Float64()*2*math.Pi - math.Pi
		lat := math.Asin(r.Float64()*2 - 1) // even over the sphere
		// Many faint stars, a few bright ones.
		m := math.Pow(r.Float64(), 3.2)
		brightness := 0.10 + 0.95*m
		size := 0.018 + 0.03*m
		col := starColors[r.Intn(len(starColors))]
		star(c, lon, lat, size, brightness, col)
	}
}

// Ursa Minor (J2000 right ascension in hours, declination in degrees,
// magnitude): the Little Dipper, with Polaris at the end of its handle,
// almost exactly at the north celestial pole.
var ursaMinor = []struct {
	name      string
	ra, dec   float64
	mag       float64
	isPolaris bool
}{
	{"Polaris", 2.5303, 89.2641, 1.98, true},
	{"Yildun", 17.5369, 86.5864, 4.35, false},
	{"Epsilon UMi", 16.7661, 82.0372, 4.21, false},
	{"Zeta UMi", 15.7344, 77.7944, 4.32, false},
	{"Eta UMi", 16.2917, 75.7553, 4.95, false},
	{"Pherkad", 15.3456, 71.8339, 3.00, false},
	{"Kochab", 14.8450, 74.1556, 2.08, false},
}

func constellation(c *canvas) {
	for _, s := range ursaMinor {
		lon := s.ra / 24 * 2 * math.Pi
		if lon > math.Pi {
			lon -= 2 * math.Pi
		}
		brightness := 0.35 + (5-s.mag)*0.32
		size := 0.03 + (5-s.mag)*0.012
		col := [3]float64{1.0, 0.97, 0.90}
		if s.isPolaris {
			col = [3]float64{1.0, 0.95, 0.82} // a touch of gold, for the logo
			star(c, lon, s.dec*math.Pi/180, 0.22, 0.05, col)
		}
		if s.name == "Kochab" {
			col = [3]float64{1.0, 0.82, 0.64}
		}
		star(c, lon, s.dec*math.Pi/180, size, brightness, col)
	}
}

// moon paints a moon: a disc of the given angular radius at (lon, lat), its
// surface mottled, lit from the direction sun (a phase), with a faint glow.
func moon(c *canvas, lon, lat, radiusDeg float64, sun vec, albedo [3]float64, seed int64) {
	center := dir(lon, lat)
	rad := radiusDeg * math.Pi / 180
	surface := newNoise(seed)
	craters := newNoise(seed + 1)
	// A frame on the sky round the moon's centre: east, north, and the
	// direction from the moon back to the viewer.
	east := vec{0, 0, 1}.cross(center).norm()
	north := center.cross(east)
	view := center.scale(-1)
	reach := rad * 3.2
	_, py := pixel(lon, lat)
	ry := int(reach/math.Pi*height) + 2
	rx := min(width/2, int(float64(ry)/math.Max(math.Cos(lat), 1e-3))+2)
	px, _ := pixel(lon, lat)
	for dy := -ry; dy <= ry; dy++ {
		y := int(py) + dy
		if y < 0 || y >= height {
			continue
		}
		for dx := -rx; dx <= rx; dx++ {
			x := int(px) + dx
			lo, la := lonLatOf(float64(x)+0.5, float64(y)+0.5)
			d := dir(lo, la)
			a := math.Acos(math.Max(-1, math.Min(1, d.dot(center))))
			if a > reach {
				continue
			}
			// The glow round it, fading out (under the disc too, so the rim
			// blends into it instead of leaving a dark ring).
			c.add(x, y, 0.035*math.Exp(-math.Max(0, a-rad)/(rad*0.6)), albedo)
			if a >= rad {
				continue
			}
			// Where on the moon's face this is, as a point on a unit sphere
			// facing the viewer.
			u := d.dot(east) / math.Sin(rad)
			v := d.dot(north) / math.Sin(rad)
			w := math.Sqrt(math.Max(0, 1-u*u-v*v))
			normal := east.scale(u).add(north.scale(v)).add(view.scale(w))
			light := math.Max(0, normal.dot(sun))
			light = math.Pow(light, 0.8)
			p := normal.scale(1)
			mottle := 0.72 + 0.28*surface.fbm(p, 2.2, 5)
			pits := craters.fbm(p, 7, 3)
			if pits > 0.62 {
				mottle *= 0.82
			}
			// Earthshine keeps the dark side just visible; the rim softens.
			edge := math.Min(1, (rad-a)/(rad*0.04))
			k := (0.06 + 0.95*light) * mottle * edge
			i := ((y%height)*width + ((x%width)+width)%width)
			cr, cg, cb := c.r[i], c.g[i], c.b[i]
			c.r[i] = float32(float64(cr)*(1-edge) + k*albedo[0])
			c.g[i] = float32(float64(cg)*(1-edge) + k*albedo[1])
			c.b[i] = float32(float64(cb)*(1-edge) + k*albedo[2])
		}
	}
}

func main() {
	out := "default-sky.jpg"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	c := &canvas{make([]float32, width*height), make([]float32, width*height), make([]float32, width*height)}
	background(c)
	stars(c)
	constellation(c)
	// Two moons: a large pale one, lit from the side, and a small rusty
	// one past full.
	moon(c, 100*math.Pi/180, 16*math.Pi/180, 2.6, dir(160*math.Pi/180, 5*math.Pi/180), [3]float64{0.86, 0.84, 0.80}, 5)
	moon(c, -130*math.Pi/180, -22*math.Pi/180, 1.2, dir(-80*math.Pi/180, -30*math.Pi/180), [3]float64{0.80, 0.66, 0.58}, 9)

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	tone := func(v float32) uint8 {
		// A gentle curve: stars don't clip harshly, the night stays dark.
		f := 1 - math.Exp(-float64(v)*1.25)
		return uint8(math.Round(math.Min(1, math.Pow(f, 1/1.15)) * 255))
	}
	for i := 0; i < width*height; i++ {
		img.Pix[i*4] = tone(c.r[i])
		img.Pix[i*4+1] = tone(c.g[i])
		img.Pix[i*4+2] = tone(c.b[i])
		img.Pix[i*4+3] = 255
	}
	f, err := os.Create(out)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 88}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
