package vision

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"testing"
)

func v2Fixture(t *testing.T, digits int, token string) (Manifest, *image.Gray) {
	t.Helper()
	m, err := BuildStudentManifest([]Question{{ID: "q1", Number: "1", Type: "single_choice", Options: []string{"A", "B", "C"}}, {ID: "q2", Number: "2", Type: "multiple_choice", Options: []string{"A", "B", "C"}}}, digits)
	if err != nil {
		t.Fatal(err)
	}
	var qr [][]bool
	if token != "" {
		qr, err = qrMatrix(token)
		if err != nil {
			t.Fatal(err)
		}
	}
	return m, raster(m, qr, 1400)
}
func fillNumber(im *image.Gray, m Manifest, number string, value uint8) {
	for _, d := range m.StudentBubbles {
		if d.Column < len(number) && d.Digit == int(number[d.Column]-48) {
			fill(im, Bubble{X: d.X, Y: d.Y, R: d.R}, value)
		}
	}
}

func TestV2ManifestAndRender(t *testing.T) {
	for _, count := range []int{1, 10, 11, 40} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			qs := make([]Question, count)
			for i := range qs {
				qs[i] = Question{ID: fmt.Sprint(i), Number: fmt.Sprint(i + 1), Type: "single_choice", Options: []string{"A", "B", "C", "D", "E"}}
			}
			m, err := BuildStudentManifest(qs, 12)
			if err != nil {
				t.Fatal(err)
			}
			if len(m.Bubbles) != count*5 || len(m.StudentBubbles) != 120 {
				t.Fatal("capacity")
			}
			if math.Abs(m.Markers[0].X*210-10) > 1e-9 || math.Abs(m.Markers[0].Y*148.5-10) > 1e-9 {
				t.Fatal("margin")
			}
			for _, b := range m.Bubbles {
				if math.Abs(b.R*210*2-3.6) > 1e-9 {
					t.Fatal("physical bubble size")
				}
			}
			data, _ := json.Marshal(m)
			var restored Manifest
			if json.Unmarshal(data, &restored) != nil || validateManifest(restored) != nil {
				t.Fatal("JSON roundtrip")
			}
			svg, err := RenderSVG(m, "<title>&", "学生<&>", "")
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(svg, []byte(`height="148.5mm"`)) || !bytes.Contains(svg, []byte("&lt;title&gt;")) {
				t.Fatal("single geometry or escaping")
			}
			dec := xml.NewDecoder(bytes.NewReader(svg))
			for {
				_, err = dec.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			two, err := RenderTwoUpSVG(m, "exam", []PrintCard{{"S1", ""}, {"S2", ""}})
			if err != nil || !bytes.Contains(two, []byte(`height="297mm"`)) || !bytes.Contains(two, []byte("translate(0 148.500)")) {
				t.Fatal("two up geometry")
			}
			odd, err := RenderTwoUpSVG(m, "exam", []PrintCard{{"S1", ""}})
			if err != nil || bytes.Contains(odd, []byte("translate(0 148.500)")) {
				t.Fatal("odd lower not blank")
			}
		})
	}
}

func TestV2IdentityStates(t *testing.T) {
	cases := []struct {
		name, number, state, expected string
		extra                         int
		faint                         bool
	}{{"blank", "", "blank", "", -1, false}, {"leading_zeros", "0012", "recognized", "0012", -1, false}, {"missing_column", "00", "uncertain", "", -1, false}, {"multiple", "0012", "invalid_multiple", "", 1, false}, {"faint", "0012", "uncertain", "", -1, true}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, im := v2Fixture(t, 4, "")
			v := uint8(0)
			if tc.faint {
				v = 140
			}
			fillNumber(im, m, tc.number, v)
			if tc.extra >= 0 {
				fill(im, Bubble{X: m.StudentBubbles[tc.extra].X, Y: m.StudentBubbles[tc.extra].Y, R: m.StudentBubbles[tc.extra].R}, 0)
			}
			r, err := Recognize(im, m)
			if err != nil {
				t.Fatalf("%v %+v", err, r.Quality)
			}
			if r.IdentityState != tc.state || r.StudentNumber != tc.expected {
				t.Fatalf("got %+v want %s %q", r, tc.state, tc.expected)
			}
		})
	}
}

func TestV2PerspectiveAndNoQR(t *testing.T) {
	m, im := v2Fixture(t, 4, "")
	fillNumber(im, m, "0012", 0)
	fill(im, m.Bubbles[1], 0)
	warped := warp(t, im, []Point{{130, 40}, {1510, 120}, {1450, 1100}, {60, 1010}}, 1600, 1160)
	r, err := Recognize(warped, m)
	if err != nil || r.IdentityState != "recognized" || r.StudentNumber != "0012" || r.Answers[0].State != "answered" {
		t.Fatalf("perspective %v %+v", err, r)
	}
	mirror := image.NewGray(im.Rect)
	for y := 0; y < im.Rect.Dy(); y++ {
		for x := 0; x < im.Rect.Dx(); x++ {
			mirror.SetGray(x, y, im.GrayAt(im.Rect.Dx()-1-x, y))
		}
	}
	if r, err = Recognize(mirror, m); err == nil || r.Quality.Accepted {
		t.Fatalf("mirror accepted %+v", r)
	}
}

func TestV2RejectTwoCards(t *testing.T) {
	m, im := v2Fixture(t, 4, "")
	fillNumber(im, m, "0012", 0)
	both := image.NewGray(image.Rect(0, 0, im.Rect.Dx(), im.Rect.Dy()*2))
	for y := 0; y < both.Rect.Dy(); y++ {
		copy(both.Pix[y*both.Stride:y*both.Stride+im.Rect.Dx()], im.Pix[(y%im.Rect.Dy())*im.Stride:(y%im.Rect.Dy())*im.Stride+im.Rect.Dx()])
	}
	r, err := Recognize(both, m)
	if err == nil || r.Quality.Accepted {
		t.Fatalf("uncut sheet accepted %+v", r)
	}
}

func TestV2WhiteOcclusion(t *testing.T) {
	m, im := v2Fixture(t, 4, "")
	fillNumber(im, m, "0012", 0)
	d := m.StudentBubbles[0]
	cx, cy := int(d.X*1400), int(d.Y*990)
	for y := cy - 15; y <= cy+15; y++ {
		for x := cx - 15; x <= cx+15; x++ {
			im.SetGray(x, y, color.Gray{255})
		}
	}
	r, err := Recognize(im, m)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if r.IdentityState == "recognized" {
		t.Fatal("occlusion bound identity")
	}
}
