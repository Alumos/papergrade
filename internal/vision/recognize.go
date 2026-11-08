package vision

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

// Recognize independently detects marker coordinates and decodes the personal
// QR. It never accepts frontend corners, fill results or a candidate token.
// This is an opt-in local PoC; callers should not enable it by default in the
// low-memory production/cloud recognition path. Do not run unlimited parallel
// recognitions. Preflight encoded uploads with image.DecodeConfig before decode.
func Recognize(img image.Image, m Manifest) (Recognition, error) {
	r := Recognition{Answers: []Answer{}, IdentityState: "not_required", Quality: Quality{Reasons: []string{}}}
	reject := func(err error, reason string) (Recognition, error) {
		r.Quality.Reasons = append(r.Quality.Reasons, reason)
		return r, err
	}
	if err := validateManifest(m); err != nil {
		return reject(err, "unsupported_manifest")
	}
	if img == nil {
		return reject(ErrImageSize, "missing_image")
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	r.Quality.Width = w
	r.Quality.Height = h
	if w < 500 || h < 500 || w > MaxPixels || h > MaxPixels || int64(w)*int64(h) > MaxPixels {
		return reject(ErrImageSize, "image_size")
	}
	g := grayImage(img)
	// Both the fixed student-number cards are half-A4 single cards: reject a whole
	// uncut two-up sheet instead of mixing the two halves.
	studentCard := m.Version == StudentVersion || m.Version == EnglishVersion || m.Version == EnglishV4Version
	corners, ok := findCardMarkers(g, studentCard)
	if !ok {
		return reject(ErrMarkers, "markers_not_found")
	}
	if studentCard {
		v2, err := recognizeStudent(g, m, corners)
		v2.Quality.Width, v2.Quality.Height = w, h
		if err != nil {
			v2.Quality.Reasons = []string{"student_identity_or_quality_rejected"}
			return v2, err
		}
		return v2, nil
	}
	var transform homography
	found := false
	// Try all four rotations, validating orientation by the top reference patches
	// AND decoding only the expected top-right QR region after rectification.
	for rotation := 0; rotation < 4; rotation++ {
		dst := make([]Point, 4)
		for i := range dst {
			dst[i] = corners[(i+rotation)%4]
		}
		hh, ok := fit(m.Markers, dst)
		if !ok {
			continue
		}
		black, bs := boxMean(g, hh, m.BlackReference)
		white, ws := boxMean(g, hh, m.WhiteReference)
		contrast := white - black
		if contrast < 90 || bs > 35 || ws > 35 {
			continue
		}
		token, err := decodeQR(g, hh, m.QRBox)
		if err != nil {
			continue
		}
		transform = hh
		found = true
		r.Token = token
		r.Quality.Contrast = contrast / 255
		r.Quality.Markers = dst
		break
	}
	if !found {
		return reject(ErrQR, "orientation_references_or_qr_invalid")
	}
	r.Quality.Sharpness = sharpness(g, transform, m, r.Quality.Contrast*255)
	if r.Quality.Sharpness < .20 {
		return reject(ErrQuality, "blurred_marker_edges")
	}
	// Check projected bubble resolution before allowing confidently blank output.
	for _, bb := range m.Bubbles {
		center := project(transform, Point{bb.X, bb.Y})
		edge := project(transform, Point{bb.X + bb.R, bb.Y})
		if math.Hypot(center.X-edge.X, center.Y-edge.Y) < 4.5 {
			return reject(ErrQuality, "bubble_resolution_too_low")
		}
	}
	black, _ := boxMean(g, transform, m.BlackReference)
	white, _ := boxMean(g, transform, m.WhiteReference)
	byID := map[string][]Bubble{}
	for _, bb := range m.Bubbles {
		byID[bb.QuestionID] = append(byID[bb.QuestionID], bb)
	}
	for _, q := range m.Questions {
		a := Answer{QuestionID: q.ID, Selected: []string{}, State: "blank", Confidence: 1}
		uncertain := false
		for _, bb := range byID[q.ID] {
			// A white occlusion or severe local blur must not masquerade as
			// confident blank: verify the preprinted ring around each bubble.
			if ringSupport(g, transform, bb, black, white) < .22 {
				uncertain = true
				a.Confidence = math.Min(a.Confidence, .35)
			}
			darkness, coverage := bubbleFill(g, transform, bb, black, white)
			// Wide reject band: faint pencil, erasures, partial fills and shading are
			// deliberately uncertain, never silently promoted to blank or selected.
			if darkness >= .62 && coverage >= .70 {
				a.Selected = append(a.Selected, bb.Option)
				a.Confidence = math.Min(a.Confidence, math.Min(darkness, coverage))
			} else if darkness <= .16 && coverage <= .12 {
				a.Confidence = math.Min(a.Confidence, 1-math.Max(darkness, coverage))
			} else {
				uncertain = true
				a.Confidence = math.Min(a.Confidence, .49)
			}
		}
		if uncertain {
			a.State = "uncertain"
		} else if len(a.Selected) > 1 && q.Type != "multiple_choice" {
			a.State = "invalid_multiple"
		} else if len(a.Selected) > 0 {
			a.State = "answered"
		}
		r.Answers = append(r.Answers, a)
	}
	r.Quality.Accepted = true
	return r, nil
}
func decodeQR(g *image.Gray, h homography, b Box) (string, error) {
	const size = 360
	crop := image.NewGray(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			p := Point{b.X + (float64(x)+.5)/size*b.Width, b.Y + (float64(y)+.5)/size*b.Height}
			crop.SetGray(x, y, color.Gray{uint8(sample(g, project(h, p)))})
		}
	}
	bitmap, err := gozxing.NewBinaryBitmapFromImage(crop)
	if err != nil {
		return "", err
	}
	result, err := qrcode.NewQRCodeReader().Decode(bitmap, map[gozxing.DecodeHintType]interface{}{gozxing.DecodeHintType_TRY_HARDER: true})
	if err != nil {
		return "", err
	}
	token := result.GetText()
	if len(token) < 16 || len(token) > 256 || strings.TrimSpace(token) != token || !validText(token) {
		return "", fmt.Errorf("invalid decoded token")
	}
	return token, nil
}
