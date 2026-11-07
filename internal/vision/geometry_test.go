package vision

import (
	"image"
	"image/color"
	"math"
	"testing"
)

func TestHomography(t *testing.T) {
	src := []Point{{.05, .05}, {.95, .05}, {.95, .95}, {.05, .95}}
	dst := []Point{{81, 99}, {1001, 38}, {1090, 1380}, {20, 1410}}
	h, ok := fit(src, dst)
	if !ok {
		t.Fatal("nondegenerate fit failed")
	}
	for i, p := range src {
		q := project(h, p)
		if math.Hypot(q.X-dst[i].X, q.Y-dst[i].Y) > 1e-7 {
			t.Fatalf("reprojection error %+v %+v", q, dst[i])
		}
	}
	if _, ok := fit([]Point{{0, 0}, {0, 0}, {0, 0}, {0, 0}}, dst); ok {
		t.Fatal("accepted degenerate source")
	}
	inverse, ok := fit(dst, src)
	if !ok {
		t.Fatal("inverse fit")
	}
	p := Point{.37, .64}
	q := project(inverse, project(h, p))
	if math.Hypot(q.X-p.X, q.Y-p.Y) > 1e-8 {
		t.Fatal("inverse mismatch")
	}
}
func TestSolidMarkersNotRings(t *testing.T) {
	g := image.NewGray(image.Rect(0, 0, 700, 1000))
	for i := range g.Pix {
		g.Pix[i] = 255
	}
	points := []Point{{35, 50}, {665, 50}, {665, 950}, {35, 950}}
	for _, p := range points {
		for y := int(p.Y) - 8; y <= int(p.Y)+8; y++ {
			for x := int(p.X) - 8; x <= int(p.X)+8; x++ {
				g.SetGray(x, y, color.Gray{0})
			}
		}
	}
	// An interior ring and dense reference should not displace page corners.
	for y := 100; y < 900; y++ {
		for x := 100; x < 600; x++ {
			if math.Abs(math.Hypot(float64(x)-350, float64(y)-500)-50) < 1 {
				g.SetGray(x, y, color.Gray{0})
			}
		}
	}
	got, ok := findMarkers(g)
	if !ok {
		t.Fatal("solid markers not detected")
	}
	for i, p := range got {
		if p != points[i] {
			t.Fatalf("wrong marker %d: %+v", i, p)
		}
	}
	// Delete a marker: fewer than four reliable corners must not be accepted.
	for y := 930; y < 980; y++ {
		for x := 10; x < 60; x++ {
			g.SetGray(x, y, color.Gray{255})
		}
	}
	if _, ok := findMarkers(g); ok {
		t.Fatal("accepted three markers and a ring")
	}
}
