package vision

import "fmt"

// BuildStudentManifest constructs a new physical half-A4 layout, NOT a scaled
// legacy card. All bubbles have 3.6mm diameter; numeric identity is never padded.
func BuildStudentManifest(questions []Question, digits int) (Manifest, error) {
	if digits < 1 || digits > 12 {
		return Manifest{}, fmt.Errorf("%w: student number requires 1..12 digits", ErrInvalidManifest)
	}
	m, err := BuildManifest(questions)
	if err != nil {
		return Manifest{}, err
	}
	m.Version = StudentVersion
	m.HeightMM = 148.5
	m.MarkerSize = 4.0 / 210
	p := func(x, y float64) Point { return Point{x / 210, y / 148.5} }
	box := func(x, y, w, h float64) Box { return Box{x / 210, y / 148.5, w / 210, h / 148.5} }
	m.Markers = []Point{p(10, 10), p(200, 10), p(200, 138.5), p(10, 138.5)}
	m.QRBox = box(158, 25, 32, 32)
	m.BlackReference = box(119, 30, 5, 5)
	m.WhiteReference = box(128, 30, 5, 5)
	m.StudentNumberDigits = digits
	m.StudentBubbles = make([]DigitBubble, 0, digits*10)
	for column := 0; column < digits; column++ {
		for digit := 0; digit < 10; digit++ {
			m.StudentBubbles = append(m.StudentBubbles, DigitBubble{column, digit, (22 + 5*float64(column)) / 210, (35 + 4.5*float64(digit)) / 148.5, 1.8 / 210})
		}
	}
	m.Bubbles = m.Bubbles[:0]
	for i, q := range m.Questions {
		column, row := i/10, i%10
		for j, option := range q.Options {
			m.Bubbles = append(m.Bubbles, Bubble{q.ID, option, (27 + 48*float64(column) + 6*float64(j)) / 210, (88 + 5*float64(row)) / 148.5, 1.8 / 210})
		}
	}
	return m, nil
}
