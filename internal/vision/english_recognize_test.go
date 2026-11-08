package vision

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// englishFilledCard renders the real English card and fills the given options
// the way a student would. It is used to prove that the letters printed inside
// the cells never make an untouched cell look half-filled.
func englishFilledCard(t *testing.T, m Manifest, number string, picked map[string][]string) image.Image {
	t.Helper()
	raw, err := RenderPNG(m, "exam-release-token", 1800)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(decoded.Bounds())
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			img.Set(x, y, decoded.At(x, y))
		}
	}
	w, h := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
	fill := func(nx, ny, nr float64) {
		cx, cy := nx*w, ny*h
		r := nr * w * 1.05
		for yy := int(cy - r - 1); yy <= int(cy+r)+1; yy++ {
			for xx := int(cx - r - 1); xx <= int(cx+r)+1; xx++ {
				dx, dy := float64(xx)-cx, float64(yy)-cy
				if dx*dx+dy*dy <= r*r {
					img.Set(xx, yy, color.Black)
				}
			}
		}
	}
	if len(number) != m.StudentNumberDigits {
		t.Fatalf("number %q does not match %d digits", number, m.StudentNumberDigits)
	}
	for _, d := range m.StudentBubbles {
		if d.Column >= 0 && d.Column < len(number) && int(number[d.Column]-'0') == d.Digit {
			fill(d.X, d.Y, d.R)
		}
	}
	for _, b := range m.Bubbles {
		for _, want := range picked[b.QuestionID] {
			if b.Option == want {
				fill(b.X, b.Y, b.R)
			}
		}
	}
	return img
}

func TestEnglishRecognizeLettersInsideCells(t *testing.T) {
	sample := englishSample(t)
	m, err := BuildEnglishManifest(sample)
	if err != nil {
		t.Fatal(err)
	}
	picked := map[string][]string{}
	// Answer every third question; the rest must stay a confident blank.
	for i, q := range m.Questions {
		if i%3 == 0 {
			picked[q.ID] = []string{q.Options[len(q.Options)-1]}
		}
	}
	img := englishFilledCard(t, m, "1932", picked)
	r, err := Recognize(img, m)
	if err != nil {
		t.Fatalf("recognize: %v (quality %+v)", err, r.Quality)
	}
	if r.IdentityState != "recognized" || r.StudentNumber != "1932" {
		t.Fatalf("identity %q state %q", r.StudentNumber, r.IdentityState)
	}
	answered, blank := 0, 0
	for _, a := range r.Answers {
		if a.State == "uncertain" || a.State == "invalid_multiple" {
			t.Fatalf("question %s wrongly ambiguous: %+v", a.QuestionID, a)
		}
		want := picked[a.QuestionID]
		if len(want) == 0 {
			if a.State != "blank" || len(a.Selected) != 0 {
				t.Fatalf("unanswered %s read as %+v", a.QuestionID, a)
			}
			blank++
			continue
		}
		if a.State != "answered" || len(a.Selected) != 1 || a.Selected[0] != want[0] {
			t.Fatalf("question %s expected %v got %+v", a.QuestionID, want, a)
		}
		answered++
	}
	if answered == 0 || blank == 0 {
		t.Fatalf("test did not cover both answered and blank: %d/%d", answered, blank)
	}
}

func TestEnglishRecognizeBlankCardIsBlankNotUncertain(t *testing.T) {
	m, err := BuildEnglishManifest(englishSample(t))
	if err != nil {
		t.Fatal(err)
	}
	img := englishFilledCard(t, m, "1932", nil)
	r, err := Recognize(img, m)
	if err != nil {
		t.Fatalf("recognize blank: %v (quality %+v)", err, r.Quality)
	}
	if r.IdentityState != "recognized" || r.StudentNumber != "1932" {
		t.Fatalf("identity %q state %q", r.StudentNumber, r.IdentityState)
	}
	for _, a := range r.Answers {
		if a.State != "blank" {
			t.Fatalf("printed letters made a blank cell ambiguous: %+v", a)
		}
	}
}
