package vision

import (
	"image"
	"math"
)

func ringSupport(g *image.Gray, h homography, b Bubble, black, white float64, ratios ...float64) float64 {
	ratio := 210.0 / 297
	if len(ratios) > 0 {
		ratio = ratios[0]
	}
	sum := 0.
	for i := 0; i < 24; i++ {
		angle := float64(i) * 2 * math.Pi / 24
		darkest := 0.
		for _, r := range []float64{.80, .9, 1, 1.1, 1.2} {
			p := Point{b.X + math.Cos(angle)*b.R*r, b.Y + math.Sin(angle)*b.R*r*ratio}
			d := math.Max(0, math.Min(1, (white-sample(g, project(h, p)))/(white-black)))
			darkest = math.Max(darkest, d)
		}
		sum += darkest
	}
	return sum / 24
}

// bubbleFillRing samples only the outer ring of a square fill box. English cards
// print the option letter inside the box, so the centre always carries template
// ink; sampling the ring keeps a blank box clearly blank while a filled box
// still covers it.
func bubbleFillRing(g *image.Gray, h homography, b Bubble, black, white float64, ratio float64) (float64, float64) {
	sum, dark, n := 0., 0., 0.
	for y := -8; y <= 8; y++ {
		for x := -8; x <= 8; x++ {
			rr := math.Hypot(float64(x), float64(y))
			if rr < .55*8 || rr > .90*8 {
				continue
			}
			p := Point{b.X + float64(x)/8*b.R, b.Y + float64(y)/8*b.R*ratio}
			v := sample(g, project(h, p))
			d := math.Max(0, math.Min(1, (white-v)/(white-black)))
			sum += d
			if d > .45 {
				dark++
			}
			n++
		}
	}
	if n == 0 {
		return 0, 0
	}
	return sum / float64(n), dark / float64(n)
}

// boxSupport checks the printed square border of an English fill box by walking
// its actual perimeter, so a printed outline is not mistaken for a filled cell.
func boxSupport(g *image.Gray, h homography, b Bubble, black, white float64, ratio float64) float64 {
	sum := 0.
	for i := 0; i < 24; i++ {
		angle := float64(i) * 2 * math.Pi / 24
		c, s := math.Cos(angle), math.Sin(angle)
		scale := math.Max(math.Abs(c), math.Abs(s))
		if scale < 1e-6 {
			continue
		}
		reach := b.R / scale
		darkest := 0.
		for _, f := range []float64{.94, 1, 1.06} {
			p := Point{b.X + c*reach*f, b.Y + s*reach*f*ratio}
			d := math.Max(0, math.Min(1, (white-sample(g, project(h, p)))/(white-black)))
			darkest = math.Max(darkest, d)
		}
		sum += darkest
	}
	return sum / 24
}

func bubbleFill(g *image.Gray, h homography, b Bubble, black, white float64, ratios ...float64) (float64, float64) {
	ratio := 210.0 / 297
	if len(ratios) > 0 {
		ratio = ratios[0]
	}
	sum, dark, n := 0., 0., 0.
	// Sample inner 60% radius, avoiding printed bubble outline and option labels.
	for y := -8; y <= 8; y++ {
		for x := -8; x <= 8; x++ {
			if x*x+y*y > 64 {
				continue
			}
			p := Point{b.X + float64(x)/8*b.R*.60, b.Y + float64(y)/8*b.R*.60*ratio}
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

// cellFillRing samples the outer ring of a rectangular fill cell. The printed
// letter sits in the centre, so a blank cell reads as blank while a filled cell
// still covers the ring.
func cellFillRing(g *image.Gray, h homography, b Bubble, halfW, halfH, black, white float64) (float64, float64) {
	sum, dark, n := 0., 0., 0.
	for y := -8; y <= 8; y++ {
		for x := -8; x <= 8; x++ {
			rr := math.Hypot(float64(x), float64(y))
			if rr < .62*8 || rr > .90*8 {
				continue
			}
			p := Point{b.X + float64(x)/8*halfW, b.Y + float64(y)/8*halfH}
			v := sample(g, project(h, p))
			d := math.Max(0, math.Min(1, (white-v)/(white-black)))
			sum += d
			if d > .45 {
				dark++
			}
			n++
		}
	}
	if n == 0 {
		return 0, 0
	}
	return sum / float64(n), dark / float64(n)
}

// cellSupport walks the printed border of a rectangular fill cell.
func cellSupport(g *image.Gray, h homography, b Bubble, halfW, halfH, black, white float64) float64 {
	sum := 0.
	for i := 0; i < 24; i++ {
		angle := float64(i) * 2 * math.Pi / 24
		c, s := math.Cos(angle), math.Sin(angle)
		// Scale the direction until it reaches the rectangle border.
		scale := 1.
		if v := math.Abs(c) / halfW; v > scale {
			scale = v
		}
		if v := math.Abs(s) / halfH; v > scale {
			scale = v
		}
		darkest := 0.
		for _, f := range []float64{.94, 1, 1.06} {
			p := Point{b.X + c/scale*f, b.Y + s/scale*f}
			d := math.Max(0, math.Min(1, (white-sample(g, project(h, p)))/(white-black)))
			darkest = math.Max(darkest, d)
		}
		sum += darkest
	}
	return sum / 24
}
