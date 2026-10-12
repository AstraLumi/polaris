package main

import "math"

// Map projections between the planet (longitude, latitude in radians) and a
// flat map (x east, y north, in planet radii). Each can also be run
// backwards; ok is false off the map. frontend/src/projection.js is the same;
// keep them in step.

type projection struct {
	forward func(lon, lat float64) (x, y float64)
	inverse func(x, y float64) (lon, lat float64, ok bool)
}

var projectionNames = []string{"equirectangular", "mercator", "mollweide", "robinson"}

func projectionByName(name string) projection {
	switch name {
	case "mercator":
		return projection{mercatorForward, mercatorInverse}
	case "mollweide":
		return projection{mollweideForward, mollweideInverse}
	case "robinson":
		return projection{robinsonForward, robinsonInverse}
	}
	return projection{
		func(lon, lat float64) (float64, float64) { return lon, lat },
		func(x, y float64) (float64, float64, bool) {
			return x, y, math.Abs(x) <= math.Pi && math.Abs(y) <= math.Pi/2
		},
	}
}

func validProjection(name string) bool {
	for _, p := range projectionNames {
		if p == name {
			return true
		}
	}
	return false
}

// Mercator stops short of the poles, which it would put at infinity.
const mercatorMaxLat = 85.0511 * math.Pi / 180

func mercatorForward(lon, lat float64) (float64, float64) {
	lat = math.Max(-mercatorMaxLat, math.Min(mercatorMaxLat, lat))
	return lon, math.Log(math.Tan(math.Pi/4 + lat/2))
}

func mercatorInverse(x, y float64) (float64, float64, bool) {
	return x, 2*math.Atan(math.Exp(y)) - math.Pi/2, math.Abs(x) <= math.Pi
}

// mollweideTheta solves 2θ + sin 2θ = π sin φ.
func mollweideTheta(lat float64) float64 {
	if math.Abs(lat) >= math.Pi/2-1e-9 {
		return math.Copysign(math.Pi/2, lat)
	}
	target := math.Pi * math.Sin(lat)
	theta := lat
	for range 30 {
		f := 2*theta + math.Sin(2*theta) - target
		step := f / (2 + 2*math.Cos(2*theta))
		theta -= step
		if math.Abs(step) < 1e-12 {
			break
		}
	}
	return theta
}

func mollweideForward(lon, lat float64) (float64, float64) {
	theta := mollweideTheta(lat)
	return 2 * math.Sqrt2 / math.Pi * lon * math.Cos(theta), math.Sqrt2 * math.Sin(theta)
}

func mollweideInverse(x, y float64) (float64, float64, bool) {
	if math.Abs(y) > math.Sqrt2 {
		return 0, 0, false
	}
	theta := math.Asin(y / math.Sqrt2)
	lat := math.Asin(math.Max(-1, math.Min(1, (2*theta+math.Sin(2*theta))/math.Pi)))
	c := math.Cos(theta)
	if c < 1e-12 {
		return 0, lat, math.Abs(x) < 1e-9
	}
	lon := math.Pi * x / (2 * math.Sqrt2 * c)
	return lon, lat, math.Abs(lon) <= math.Pi
}

// Robinson's table, every 5° of latitude from 0 to 90: how long the parallel
// is (X) and how far from the equator it is drawn (Y).
var robinsonX = []float64{1.0000, 0.9986, 0.9954, 0.9900, 0.9822, 0.9730, 0.9600, 0.9427, 0.9216, 0.8962,
	0.8679, 0.8350, 0.7986, 0.7597, 0.7186, 0.6732, 0.6213, 0.5722, 0.5322}
var robinsonY = []float64{0.0000, 0.0620, 0.1240, 0.1860, 0.2480, 0.3100, 0.3720, 0.4340, 0.4958, 0.5571,
	0.6176, 0.6769, 0.7346, 0.7903, 0.8435, 0.8936, 0.9394, 0.9761, 1.0000}

// robinsonAt interpolates the table at |lat| (radians).
func robinsonAt(lat float64) (float64, float64) {
	deg := math.Min(90, math.Abs(lat)*180/math.Pi)
	k := min(17, int(deg/5))
	f := (deg - float64(k)*5) / 5
	return robinsonX[k] + (robinsonX[k+1]-robinsonX[k])*f, robinsonY[k] + (robinsonY[k+1]-robinsonY[k])*f
}

func robinsonForward(lon, lat float64) (float64, float64) {
	px, py := robinsonAt(lat)
	return 0.8487 * px * lon, math.Copysign(1.3523*py, lat)
}

func robinsonInverse(x, y float64) (float64, float64, bool) {
	py := math.Abs(y) / 1.3523
	if py > 1 {
		return 0, 0, false
	}
	k := 0
	for k < 17 && robinsonY[k+1] < py {
		k++
	}
	f := (py - robinsonY[k]) / (robinsonY[k+1] - robinsonY[k])
	lat := math.Copysign((float64(k)+f)*5*math.Pi/180, y)
	px, _ := robinsonAt(lat)
	lon := x / (0.8487 * px)
	return lon, lat, math.Abs(lon) <= math.Pi
}
