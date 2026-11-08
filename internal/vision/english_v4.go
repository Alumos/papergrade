package vision

import "fmt"

// EnglishV4Version is the current English half-A4 card. Unlike a5-v3 it prints
// one question per horizontal row: the number sits on the left and the option
// cells run left to right as wide rectangles. Column and row counts are derived
// from the real paper (question count and option counts) instead of assuming a
// fixed 60-question split.
const EnglishV4Version = "a5-v4"

const (
	englishV4CellW    = 4.6  // fill cell width in mm
	englishV4CellH    = 3.4  // fill cell height in mm
	englishV4Pitch    = 5.1  // option cell centre pitch in mm
	englishV4NumberW  = 8.5  // reserved width for the printed question number
	englishV4ColGap   = 3.0  // gap between question columns in mm
	englishV4Left     = 10.0 // left content edge in mm
	englishV4Right    = 200.0
	englishV4Top      = 61.0 // first question row centre in mm
	englishV4RowPitch = 4.5
	englishV4MaxRows  = 15
	englishV4Bottom   = 128.0
)

// englishV4Plan is the computed grid for one concrete paper.
type englishV4Plan struct {
	Cols       int
	Rows       int
	MaxOptions int
	ColWidth   float64
}

func (p englishV4Plan) colX(col int) float64 {
	return englishV4Left + float64(col)*(p.ColWidth+englishV4ColGap)
}

func (p englishV4Plan) rowY(row int) float64 {
	return englishV4Top + float64(row)*englishV4RowPitch
}

// englishV4Grid derives the number of columns that fit the printed width and
// the rows actually needed. A paper with few questions gets fewer rows, not a
// half-empty fixed grid; a paper that cannot fit is rejected, never shrunk.
func englishV4Grid(questions []Question) (englishV4Plan, error) {
	if len(questions) == 0 {
		return englishV4Plan{}, fmt.Errorf("%w: english card needs at least one question", ErrInvalidManifest)
	}
	maxOptions := 2
	for _, q := range questions {
		if len(q.Options) > maxOptions {
			maxOptions = len(q.Options)
		}
	}
	plan := englishV4Plan{MaxOptions: maxOptions}
	plan.ColWidth = englishV4NumberW + float64(maxOptions)*englishV4Pitch
	available := englishV4Right - englishV4Left
	fit := int((available + englishV4ColGap) / (plan.ColWidth + englishV4ColGap))
	if fit < 1 {
		return englishV4Plan{}, fmt.Errorf("%w: options do not fit the printable card width", ErrInvalidManifest)
	}
	if fit > len(questions) {
		fit = len(questions)
	}
	plan.Cols = fit
	plan.Rows = (len(questions) + plan.Cols - 1) / plan.Cols
	if plan.Rows > englishV4MaxRows {
		return englishV4Plan{}, fmt.Errorf("%w: %d questions need %d rows but only %d fit half A4; split the paper", ErrInvalidManifest, len(questions), plan.Rows, englishV4MaxRows)
	}
	if last := plan.rowY(plan.Rows-1) + englishV4CellH/2; last > englishV4Bottom {
		return englishV4Plan{}, fmt.Errorf("%w: question rows exceed the printable height", ErrInvalidManifest)
	}
	return plan, nil
}

// BuildEnglishManifestV4 lays the real paper out as horizontal rows of
// rectangular fill cells. Question order is column-major, the usual reading
// order for answer sheets.
func BuildEnglishManifestV4(questions []EnglishQuestion) (Manifest, error) {
	base, err := englishBase(questions)
	if err != nil {
		return Manifest{}, err
	}
	m, err := buildManifestCore(base, englishQuestions, englishOptions)
	if err != nil {
		return Manifest{}, err
	}
	plan, err := englishV4Grid(m.Questions)
	if err != nil {
		return Manifest{}, err
	}
	m.Version = EnglishV4Version
	m.HeightMM = 148.5
	m.MarkerSize = 4.0 / 210
	m.BubbleW = englishV4CellW / 210
	m.BubbleH = englishV4CellH / 148.5
	p := func(x, y float64) Point { return Point{x / 210, y / 148.5} }
	box := func(x, y, w, h float64) Box { return Box{x / 210, y / 148.5, w / 210, h / 148.5} }
	m.Markers = []Point{p(10, 10), p(200, 10), p(200, 138.5), p(10, 138.5)}
	m.QRBox = box(166, 25, 24, 24)
	m.BlackReference = box(88, 48, 4.5, 4.5)
	m.WhiteReference = box(95, 48, 4.5, 4.5)
	m.StudentNumberDigits = 4
	m.StudentBubbles = englishV4Digits()
	m.Bubbles = make([]Bubble, 0, len(m.Questions)*englishOptions)
	for i, q := range m.Questions {
		col, row := i/plan.Rows, i%plan.Rows
		for j, o := range q.Options {
			m.Bubbles = append(m.Bubbles, Bubble{
				QuestionID: q.ID,
				Option:     o,
				X:          (plan.colX(col) + englishV4NumberW + englishV4Pitch*(float64(j)+0.5)) / 210,
				Y:          plan.rowY(row) / 148.5,
				R:          englishV4CellW / 2 / 210,
			})
		}
	}
	return m, nil
}

// englishV4Digits prints the fixed four-digit student number as four horizontal
// rows, one row per digit position, using the same rectangular cells.
func englishV4Digits() []DigitBubble {
	out := make([]DigitBubble, 0, 40)
	for column := 0; column < 4; column++ {
		for digit := 0; digit < 10; digit++ {
			out = append(out, DigitBubble{
				Column: column,
				Digit:  digit,
				X:      (22 + 5.4*float64(digit)) / 210,
				Y:      (32 + 4.4*float64(column)) / 148.5,
				R:      englishV4CellW / 2 / 210,
			})
		}
	}
	return out
}
