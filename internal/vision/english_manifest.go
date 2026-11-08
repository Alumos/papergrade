package vision

import "fmt"

// EnglishVersion is the fixed half-A4 card aimed at English teachers: short
// three-option listening items, four-option reading items and seven-option
// "choose five of seven" items all share one versioned physical layout.
const EnglishVersion = "a5-v3"

const (
	englishQuestions  = 60
	englishOptions    = 7
	englishCols       = 20
	englishBands      = 3
	englishPitchY     = 4.2 // option row pitch in mm
	englishPitchX     = 9.0 // question column pitch in mm
	englishGroupGap   = 2.5 // extra gap after every five questions
	englishBandStart  = 64.0
	englishBandGap    = 10.0 // headers plus spacing between option stacks
	englishBottomEdge = 134.0
)

// englishColumnX returns the horizontal centre of one question column. Every
// five questions get a visual break so a teacher can find a number quickly.
func englishColumnX(col int) float64 {
	return 10 + englishPitchX*float64(col) + englishGroupGap*float64(col/5)
}

// EnglishQuestion is the English-card question. It adds no fields of its own:
// the optional printed group heading lives in Question.Section, which is also
// what the manifest stores, so validation rebuilds exactly the same card.
type EnglishQuestion struct {
	Question
}

// BuildEnglishManifest constructs the fixed English half-A4 layout. Anything
// that cannot be printed at the validated physical size is rejected instead of
// being shrunk into an unfillable card.
func BuildEnglishManifest(questions []EnglishQuestion) (Manifest, error) {
	if len(questions) < 1 || len(questions) > englishQuestions {
		return Manifest{}, fmt.Errorf("%w: english card supports 1..%d questions", ErrInvalidManifest, englishQuestions)
	}
	base := make([]Question, len(questions))
	for i, q := range questions {
		if len(q.Options) < 2 || len(q.Options) > englishOptions {
			return Manifest{}, fmt.Errorf("%w: english options require 2..%d", ErrInvalidManifest, englishOptions)
		}
		base[i] = q.Question
		base[i].Section = q.Section
	}
	m, err := buildManifestCore(base, englishQuestions, englishOptions)
	if err != nil {
		return Manifest{}, err
	}
	m.Version = EnglishVersion
	m.HeightMM = 148.5
	m.MarkerSize = 4.0 / 210
	p := func(x, y float64) Point { return Point{x / 210, y / 148.5} }
	box := func(x, y, w, h float64) Box { return Box{x / 210, y / 148.5, w / 210, h / 148.5} }
	m.Markers = []Point{p(10, 10), p(200, 10), p(200, 138.5), p(10, 138.5)}
	// The QR only identifies the shared exam release. It is deliberately small so
	// the answer area keeps its physical size.
	m.QRBox = box(166, 25, 24, 24)
	m.BlackReference = box(119, 30, 5, 5)
	m.WhiteReference = box(128, 30, 5, 5)
	m.StudentNumberDigits = 4
	m.StudentBubbles = englishDigitBubbles()

	maxBand := [englishBands]int{2, 2, 2}
	for i, q := range questions {
		if n := len(q.Options); n > maxBand[i/englishCols] {
			maxBand[i/englishCols] = n
		}
	}
	starts := englishStarts(maxBand)
	if starts[englishBands-1]+float64(maxBand[englishBands-1]-1)*englishPitchY+1.8 > englishBottomEdge {
		return Manifest{}, fmt.Errorf("%w: english option bands exceed the printable half-A4 height; reduce questions/options", ErrInvalidManifest)
	}
	m.Bubbles = make([]Bubble, 0, len(questions)*englishOptions)
	for i, q := range questions {
		band, col := i/englishCols, i%englishCols
		for j, o := range q.Options {
			m.Bubbles = append(m.Bubbles, Bubble{
				QuestionID: q.ID,
				Option:     o,
				X:          englishColumnX(col) / 210,
				Y:          (starts[band] + float64(j)*englishPitchY) / 148.5,
				R:          1.8 / 210,
			})
		}
	}
	return m, nil
}

// englishStarts returns the first option-row centre for each band. Bands are
// stacked by the tallest option stack they contain, so a seven-option group
// never overlaps the band below it.
func englishStarts(maxBand [englishBands]int) [englishBands]float64 {
	out := [englishBands]float64{}
	out[0] = englishBandStart
	for i := 1; i < englishBands; i++ {
		out[i] = out[i-1] + float64(maxBand[i-1]-1)*englishPitchY + englishBandGap
	}
	return out
}

// englishDigitBubbles lays the fixed four-digit student number out as four
// horizontal rows (one row per digit position, bubbles 0..9). This keeps the
// identity block short enough that a 60-question card still fits half A4.
func englishDigitBubbles() []DigitBubble {
	out := make([]DigitBubble, 0, 40)
	for column := 0; column < 4; column++ {
		for digit := 0; digit < 10; digit++ {
			out = append(out, DigitBubble{
				Column: column,
				Digit:  digit,
				X:      (24 + englishPitchY*float64(digit)) / 210,
				Y:      (37 + englishPitchY*float64(column)) / 148.5,
				R:      1.8 / 210,
			})
		}
	}
	return out
}
