package vision

import (
	"image"
	"math"
)

func ringSupport(g *image.Gray, h homography, b Bubble, black, white float64) float64 {
	sum := 0.
	for i := 0; i < 24; i++ {
		angle := float64(i) * 2 * math.Pi / 24
		darkest := 0.
		for _, r := range []float64{.80, .9, 1, 1.1, 1.2} {
			p := Point{b.X + math.Cos(angle)*b.R*r, b.Y + math.Sin(angle)*b.R*r*210/297}
			d := math.Max(0, math.Min(1, (white-sample(g, project(h, p)))/(white-black)))
			darkest = math.Max(darkest, d)
		}
		sum += darkest
	}
	return sum / 24
}

func bubbleFill(g *image.Gray, h homography, b Bubble, black, white float64) (float64, float64) {
	sum, dark, n := 0., 0., 0.
	// Sample inner 60% radius, avoiding printed bubble outline and option labels.
	for y := -8; y <= 8; y++ {
		for x := -8; x <= 8; x++ {
			if x*x+y*y > 64 {
				continue
			}
			p := Point{b.X + float64(x)/8*b.R*.60, b.Y + float64(y)/8*b.R*.60*210/297}
			v := sample(g, project(h, p))
			d := math.Max(0, math.Min(1, (white-v)/(white-black)))
			sum += d
			if d > .45 {
				dark++
			}
			n++
		}
	}
	return sum / n, dark / n
}
