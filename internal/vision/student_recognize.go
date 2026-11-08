package vision

import (
	"image"
	"math"
)

func recognizeStudent(g *image.Gray, m Manifest, corners []Point) (Recognition, error) {
	r := Recognition{Answers: []Answer{}, IdentityState: "blank", Quality: Quality{Reasons: []string{}}}
	ratio := m.WidthMM / m.HeightMM
	var transform homography
	found := false
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
		if white-black < 90 || bs > 35 || ws > 35 {
			continue
		}
		if found {
			return r, ErrQuality
		} // ambiguous orientation must never bind identity
		transform = hh
		r.Quality.Contrast = (white - black) / 255
		r.Quality.Markers = dst
		found = true
	}
	if !found {
		return r, ErrQuality
	}
	r.Quality.Sharpness = sharpness(g, transform, m, r.Quality.Contrast*255)
	if r.Quality.Sharpness < .20 {
		return r, ErrQuality
	}
	// Token is only an optional shared release/exam identifier in v2.
	// It must never be used to infer or validate a student identity.
	r.Token, _ = decodeQR(g, transform, m.QRBox)
	all := append([]Bubble(nil), m.Bubbles...)
	for _, d := range m.StudentBubbles {
		all = append(all, Bubble{X: d.X, Y: d.Y, R: d.R})
	}
	for _, b := range all {
		center := project(transform, Point{b.X, b.Y})
		edge := project(transform, Point{b.X + b.R, b.Y})
		if math.Hypot(center.X-edge.X, center.Y-edge.Y) < 4.5 {
			return r, ErrQuality
		}
	}
	black, _ := boxMean(g, transform, m.BlackReference)
	white, _ := boxMean(g, transform, m.WhiteReference)
	// English cards print the option letter inside the fill box, so their centre
	// always carries template ink. Those cards sample the outer ring and verify
	// the printed square border instead of the circular v2 bubble.
	english := m.Version == EnglishVersion
	fill := func(b Bubble) (float64, float64) {
		if english {
			return bubbleFillRing(g, transform, b, black, white, ratio)
		}
		return bubbleFill(g, transform, b, black, white, ratio)
	}
	outline := func(b Bubble) float64 {
		if english {
			return boxSupport(g, transform, b, black, white, ratio)
		}
		return ringSupport(g, transform, b, black, white, ratio)
	}
	cols := make([][]DigitBubble, m.StudentNumberDigits)
	for _, d := range m.StudentBubbles {
		if d.Column >= 0 && d.Column < len(cols) && d.Digit >= 0 && d.Digit < 10 {
			cols[d.Column] = append(cols[d.Column], d)
		}
	}
	digits := make([]byte, 0, m.StudentNumberDigits)
	blank, uncertain, multiple := 0, false, false
	for _, col := range cols {
		selected := ""
		count := 0
		for _, d := range col {
			digit := Bubble{X: d.X, Y: d.Y, R: d.R}
			if outline(digit) < .22 {
				uncertain = true
			}
			dark, coverage := fill(digit)
			if dark >= .62 && coverage >= .70 {
				selected = string(rune('0' + d.Digit))
				count++
			} else if !(dark <= .16 && coverage <= .12) {
				uncertain = true
			}
		}
		if count == 0 {
			blank++
		} else if count > 1 {
			multiple = true
		} else {
			digits = append(digits, selected[0])
		}
	}
	switch {
	case multiple:
		r.IdentityState = "invalid_multiple"
	case uncertain:
		r.IdentityState = "uncertain"
	case blank == len(cols):
		r.IdentityState = "blank"
	case blank > 0:
		r.IdentityState = "uncertain"
	default:
		r.IdentityState = "recognized"
		r.StudentNumber = string(digits)
	}
	byID := map[string][]Bubble{}
	for _, b := range m.Bubbles {
		byID[b.QuestionID] = append(byID[b.QuestionID], b)
	}
	for _, q := range m.Questions {
		a := Answer{QuestionID: q.ID, Selected: []string{}, State: "blank", Confidence: 1}
		u := false
		for _, b := range byID[q.ID] {
			if outline(b) < .22 {
				u = true
				a.Confidence = math.Min(a.Confidence, .35)
			}
			dark, cov := fill(b)
			if dark >= .62 && cov >= .70 {
				a.Selected = append(a.Selected, b.Option)
				a.Confidence = math.Min(a.Confidence, math.Min(dark, cov))
			} else if !(dark <= .16 && cov <= .12) {
				u = true
				a.Confidence = math.Min(a.Confidence, .49)
			}
		}
		if u {
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
