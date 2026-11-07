package vision

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestWhiteOcclusionNotConfidentBlank(t *testing.T) {
	m, im := fixture(t)
	b := m.Bubbles[0]
	for y := int(b.Y*float64(im.Rect.Dy()) - 15); y < int(b.Y*float64(im.Rect.Dy())+15); y++ {
		for x := int(b.X*float64(im.Rect.Dx()) - 15); x < int(b.X*float64(im.Rect.Dx())+15); x++ {
			im.SetGray(x, y, color.Gray{255})
		}
	}
	r, err := Recognize(im, m)
	if err != nil {
		t.Fatalf("%v %+v", err, r.Quality)
	}
	if r.Answers[0].State != "uncertain" {
		t.Fatalf("occlusion reported confident blank: %+v", r.Answers[0])
	}
}

func TestPartialMarkIsUncertain(t *testing.T) {
	m, im := fixture(t)
	b := m.Bubbles[0]
	fill(im, b, 0)
	// Erase half the mark; solid central half is not a confident full bubble.
	for y := int(b.Y*float64(im.Rect.Dy()) - 12); y < int(b.Y*float64(im.Rect.Dy())+12); y++ {
		for x := int(b.X * float64(im.Rect.Dx())); x < int(b.X*float64(im.Rect.Dx())+8); x++ {
			im.SetGray(x, y, color.Gray{255})
		}
	}
	r, err := Recognize(im, m)
	if err != nil {
		t.Fatalf("%v %+v", err, r.Quality)
	}
	if r.Answers[0].State != "uncertain" {
		t.Fatalf("partial mark not uncertain: %+v", r.Answers[0])
	}
}
func TestJPEGAndNonzeroImageOrigin(t *testing.T) {
	m, im := fixture(t)
	fill(im, m.Bubbles[0], 0)
	var data bytes.Buffer
	if err := jpeg.Encode(&data, im, &jpeg.Options{Quality: 75}); err != nil {
		t.Fatal(err)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(data.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	r, err := Recognize(decoded, m)
	if err != nil {
		t.Fatalf("JPEG %v %+v", err, r.Quality)
	}
	if r.Answers[0].State != "answered" || r.Answers[0].Selected[0] != "A" {
		t.Fatalf("bad JPEG recognition %+v", r.Answers)
	}
	shifted := image.NewGray(image.Rect(20, 30, 20+im.Rect.Dx(), 30+im.Rect.Dy()))
	for y := 0; y < im.Rect.Dy(); y++ {
		copy(shifted.Pix[y*shifted.Stride:y*shifted.Stride+im.Rect.Dx()], im.Pix[y*im.Stride:y*im.Stride+im.Rect.Dx()])
	}
	r, err = Recognize(shifted, m)
	if err != nil {
		t.Fatalf("shifted origin %v %+v", err, r.Quality)
	}
	if r.Token != testToken || r.Answers[0].State != "answered" {
		t.Fatal("nonzero image origin mismatch")
	}
}
