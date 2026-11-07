package vision

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"reflect"
	"strings"
	"testing"
)

const testToken = "sheet_opaque_random_token_0123456789"

func fixture(t *testing.T) (Manifest, *image.Gray) {
	t.Helper()
	m, err := BuildManifest([]Question{{"q1", "1", "single_choice", []string{"A", "B", "C", "D"}}, {"q2", "2", "multiple_choice", []string{"A", "B", "C"}}, {"q3", "3", "true_false", []string{"T", "F"}}})
	if err != nil {
		t.Fatal(err)
	}
	qr, err := qrMatrix(testToken)
	if err != nil {
		t.Fatal(err)
	}
	return m, raster(m, qr, 1000)
}
func fill(im *image.Gray, b Bubble, v uint8) {
	w, h := float64(im.Rect.Dx()), float64(im.Rect.Dy())
	cx, cy, r := b.X*w, b.Y*h, b.R*w*.88
	for y := int(cy - r); y <= int(cy+r); y++ {
		for x := int(cx - r); x <= int(cx+r); x++ {
			if math.Hypot(float64(x)-cx, float64(y)-cy) <= r {
				im.SetGray(x, y, color.Gray{v})
			}
		}
	}
}
func TestRecognitionStates(t *testing.T) {
	tests := []struct {
		name     string
		fills    map[int]uint8
		states   []string
		selected [][]string
	}{
		{"blank", nil, []string{"blank", "blank", "blank"}, [][]string{{}, {}, {}}},
		{"single", map[int]uint8{1: 0}, []string{"answered", "blank", "blank"}, [][]string{{"B"}, {}, {}}},
		{"single_invalid_multiple", map[int]uint8{0: 0, 2: 0}, []string{"invalid_multiple", "blank", "blank"}, [][]string{{"A", "C"}, {}, {}}},
		{"multiple_allowed", map[int]uint8{4: 0, 6: 0}, []string{"blank", "answered", "blank"}, [][]string{{}, {"A", "C"}, {}}},
		{"faint_uncertain", map[int]uint8{0: 140}, []string{"uncertain", "blank", "blank"}, [][]string{{}, {}, {}}},
		{"mixed_uncertain", map[int]uint8{0: 0, 1: 140}, []string{"uncertain", "blank", "blank"}, [][]string{{"A"}, {}, {}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, im := fixture(t)
			for i, v := range tc.fills {
				fill(im, m.Bubbles[i], v)
			}
			r, err := Recognize(im, m)
			if err != nil {
				t.Fatalf("%v quality=%+v", err, r.Quality)
			}
			if !r.Quality.Accepted || r.Token != testToken {
				t.Fatalf("unexpected result %+v", r)
			}
			for i, a := range r.Answers {
				if a.State != tc.states[i] || !reflect.DeepEqual(a.Selected, tc.selected[i]) {
					t.Fatalf("answer %d: %+v want %s %v", i, a, tc.states[i], tc.selected[i])
				}
			}
		})
	}
}
func warp(t *testing.T, src *image.Gray, dst []Point, w, h int) *image.Gray {
	t.Helper()
	original := []Point{{0, 0}, {float64(src.Rect.Dx() - 1), 0}, {float64(src.Rect.Dx() - 1), float64(src.Rect.Dy() - 1)}, {0, float64(src.Rect.Dy() - 1)}}
	hh, ok := fit(dst, original)
	if !ok {
		t.Fatal("fit")
	}
	out := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			out.SetGray(x, y, color.Gray{uint8(sample(src, project(hh, Point{float64(x), float64(y)})))})
		}
	}
	return out
}
func TestSkewPerspectiveAndRotation(t *testing.T) {
	m, im := fixture(t)
	fill(im, m.Bubbles[2], 0)
	fill(im, m.Bubbles[4], 0)
	fill(im, m.Bubbles[5], 0)
	tests := []struct {
		name string
		quad []Point
		w, h int
	}{
		{"tilt", []Point{{150, 35}, {1100, 130}, {980, 1490}, {30, 1395}}, 1160, 1530},
		{"perspective", []Point{{190, 100}, {1070, 30}, {1140, 1490}, {20, 1400}}, 1200, 1550},
		{"ninety", []Point{{1490, 20}, {1490, 1020}, {75, 1020}, {75, 20}}, 1550, 1080},
		{"upside_down", []Point{{1100, 1500}, {100, 1500}, {100, 85}, {1100, 85}}, 1200, 1600},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			warped := warp(t, im, tc.quad, tc.w, tc.h)
			r, err := Recognize(warped, m)
			if err != nil {
				t.Fatalf("%v quality=%+v", err, r.Quality)
			}
			if r.Token != testToken || r.Answers[0].State != "answered" || !reflect.DeepEqual(r.Answers[0].Selected, []string{"C"}) || !reflect.DeepEqual(r.Answers[1].Selected, []string{"A", "B"}) {
				t.Fatalf("wrong recognition %+v", r)
			}
		})
	}
}

type sizedImage struct{ bounds image.Rectangle }

func (s sizedImage) Bounds() image.Rectangle { return s.bounds }
func (s sizedImage) ColorModel() color.Model { return color.GrayModel }
func (s sizedImage) At(x, y int) color.Color { panic("oversized image must never be sampled") }

func TestRejectMalformedAndPoorImages(t *testing.T) {
	m, im := fixture(t)
	for _, tc := range []struct {
		name  string
		image image.Image
	}{{"nil", nil}, {"tiny", image.NewGray(image.Rect(0, 0, 200, 300))}, {"white", image.NewUniform(color.White)}, {"over_8mp", sizedImage{image.Rect(0, 0, 3000, 3000)}}, {"empty", image.NewGray(image.Rect(0, 0, 1000, 1400))}} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Recognize(tc.image, m)
			if err == nil || r.Quality.Accepted {
				t.Fatalf("unexpected acceptance %+v", r)
			}
		})
	}
	t.Run("missing_marker", func(t *testing.T) {
		m, im := fixture(t)
		for y := 0; y < 130; y++ {
			for x := 0; x < 130; x++ {
				im.SetGray(x, y, color.Gray{255})
			}
		}
		r, err := Recognize(im, m)
		if err == nil || r.Quality.Accepted {
			t.Fatal("accepted missing marker")
		}
	})
	t.Run("covered_qr", func(t *testing.T) {
		m, im := fixture(t)
		for y := 90; y < 300; y++ {
			for x := 740; x < 950; x++ {
				im.SetGray(x, y, color.Gray{255})
			}
		}
		r, err := Recognize(im, m)
		if !errors.Is(err, ErrQR) || r.Quality.Accepted {
			t.Fatalf("%v %+v", err, r)
		}
	})
	t.Run("low_contrast", func(t *testing.T) {
		m, im := fixture(t)
		for i, v := range im.Pix {
			im.Pix[i] = 150 + v/5
		}
		r, err := Recognize(im, m)
		if err == nil || r.Quality.Accepted {
			t.Fatal("accepted low contrast")
		}
	})
	t.Run("blur", func(t *testing.T) {
		blur := image.NewGray(im.Rect)
		const radius = 9
		w, h := im.Rect.Dx(), im.Rect.Dy()
		// Summed-area blur keeps the rejection fixture O(pixels), not O(radius²).
		integral := make([]int, (w+1)*(h+1))
		for y := 0; y < h; y++ {
			row := 0
			for x := 0; x < w; x++ {
				row += int(im.GrayAt(x, y).Y)
				integral[(y+1)*(w+1)+x+1] = integral[y*(w+1)+x+1] + row
			}
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				x0, x1 := max(0, x-radius), min(w, x+radius+1)
				y0, y1 := max(0, y-radius), min(h, y+radius+1)
				sum := integral[y1*(w+1)+x1] - integral[y0*(w+1)+x1] - integral[y1*(w+1)+x0] + integral[y0*(w+1)+x0]
				blur.SetGray(x, y, color.Gray{uint8(sum / ((x1 - x0) * (y1 - y0)))})
			}
		}
		r, err := Recognize(blur, m)
		if err == nil || r.Quality.Accepted {
			t.Fatal("accepted heavy blur")
		}
	})
}
func TestManifestAndSVG(t *testing.T) {
	questions := make([]Question, 40)
	for i := range questions {
		questions[i] = Question{fmt.Sprintf("q%d", i), fmt.Sprint(i + 1), "single_choice", []string{"A", "B", "C", "D", "E"}}
	}
	m, err := BuildManifest(questions)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Bubbles) != 200 {
		t.Fatal("capacity")
	}
	for _, b := range m.Bubbles {
		if b.X-b.R < .07 || b.X+b.R > .94 || b.Y < .22 || b.Y > .87 {
			t.Fatalf("out of page %+v", b)
		}
	}
	js, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var roundtrip Manifest
	if err = json.Unmarshal(js, &roundtrip); err != nil {
		t.Fatal(err)
	}
	if err = validateManifest(roundtrip); err != nil {
		t.Fatal(err)
	}
	svg, err := RenderSVG(m, `<script>&"title`, "学生<&>", testToken)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(svg, []byte("<script>")) || !bytes.Contains(svg, []byte("&lt;script&gt;")) {
		t.Fatal("unescaped text")
	}
	decoder := xml.NewDecoder(bytes.NewReader(svg))
	for {
		_, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	pngData, err := RenderPNG(m, testToken, 1000)
	if err != nil {
		t.Fatal(err)
	}
	raster, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		t.Fatal(err)
	}
	r, err := Recognize(raster, m)
	if err != nil {
		t.Fatalf("%v %+v", err, r.Quality)
	}
	if len(r.Answers) != 40 {
		t.Fatal("missing answers")
	}
	m.Bubbles[0].X = .99
	if _, err = Recognize(raster, m); !errors.Is(err, ErrInvalidManifest) {
		t.Fatal("tampered manifest accepted")
	}
}
func TestInvalidManifest(t *testing.T) {
	for _, qs := range [][]Question{nil, make([]Question, 41), {{"q", "1", "unknown", []string{"A", "B"}}}, {{"q", "1", "single_choice", []string{"A", "A"}}}, {{"q", "1", "true_false", []string{"A", "B", "C"}}}, {{"q", "1", "single_choice", []string{"A", "B"}}, {"q", "2", "single_choice", []string{"A", "B"}}}} {
		if _, err := BuildManifest(qs); err == nil {
			t.Fatalf("accepted invalid questions %+v", qs)
		}
	}
	m, _ := fixture(t)
	if _, err := RenderSVG(m, "bad"+string(rune(0)), "name", testToken); err == nil {
		t.Fatal("accepted invalid XML")
	}
	if _, err := RenderSVG(m, "title", "name", strings.Repeat("x", 257)); err == nil {
		t.Fatal("accepted huge token")
	}
}
