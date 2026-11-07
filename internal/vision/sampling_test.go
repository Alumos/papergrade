package vision

import (
	"image"
	"image/color"
	"math"
	"testing"
)

func TestCalibratedSampling(t *testing.T) {
	const w, h = 700, 990
	im := image.NewGray(image.Rect(0, 0, w, h))
	for i := range im.Pix {
		im.Pix[i] = 255
	}
	transform := homography{w, 0, 0, 0, h, 0, 0, 0, 1}
	b := Bubble{QuestionID: "q", Option: "A", X: .5, Y: .5, R: .009}
	for y := 480; y < 510; y++ {
		for x := 335; x < 365; x++ {
			d := math.Hypot(float64(x)-350, float64(y)-495)
			if math.Abs(d-6.3) < .8 {
				im.SetGray(x, y, color.Gray{0})
			}
		}
	}
	if v := ringSupport(im, transform, b, 0, 255); v < .5 {
		t.Fatalf("ring weak %.3f", v)
	}
	mean, coverage := bubbleFill(im, transform, b, 0, 255)
	if mean > .01 || coverage > .01 {
		t.Fatalf("outline leaked into interior %f %f", mean, coverage)
	}
	for y := 487; y <= 503; y++ {
		for x := 342; x <= 358; x++ {
			if math.Hypot(float64(x)-350, float64(y)-495) < 5.5 {
				im.SetGray(x, y, color.Gray{140})
			}
		}
	}
	mean, coverage = bubbleFill(im, transform, b, 0, 255)
	if mean < .4 || mean > .5 || coverage < .95 {
		t.Fatalf("faint fill sampling %f %f", mean, coverage)
	}
	for y := 487; y <= 503; y++ {
		for x := 342; x <= 358; x++ {
			if math.Hypot(float64(x)-350, float64(y)-495) < 5.5 {
				im.SetGray(x, y, color.Gray{0})
			}
		}
	}
	mean, coverage = bubbleFill(im, transform, b, 0, 255)
	if mean < .95 || coverage < .95 {
		t.Fatalf("solid fill sampling %f %f", mean, coverage)
	}
}
