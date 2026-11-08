package vision

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func englishV4Sample(t *testing.T, counts map[string]int) []EnglishQuestion {
	t.Helper()
	var qs []EnglishQuestion
	n := 0
	for _, section := range []string{"听力", "阅读", "七选五", "知识运用"} {
		options := []string{"A", "B", "C", "D"}
		switch section {
		case "听力":
			options = []string{"A", "B", "C"}
		case "七选五":
			options = []string{"A", "B", "C", "D", "E", "F", "G"}
		}
		for i := 0; i < counts[section]; i++ {
			n++
			qs = append(qs, EnglishQuestion{Question: Question{
				ID: "q" + itoaVision(n), Number: itoaVision(n), Type: "single_choice",
				Options: options, Section: section,
			}})
		}
	}
	return qs
}

func itoaVision(n int) string {
	if n == 0 {
		return "0"
	}
	out := ""
	for n > 0 {
		out = string(rune('0'+n%10)) + out
		n /= 10
	}
	return out
}

// TestEnglishV4AdaptsToRealPaper proves the grid is derived from the actual
// paper: a short paper uses fewer rows instead of a half-empty fixed layout.
func TestEnglishV4AdaptsToRealPaper(t *testing.T) {
	short := englishV4Sample(t, map[string]int{"听力": 10, "阅读": 5})
	m, err := BuildEnglishManifestV4(short)
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != EnglishV4Version || m.StudentNumberDigits != 4 {
		t.Fatalf("version/digits %q/%d", m.Version, m.StudentNumberDigits)
	}
	if m.BubbleW <= 0 || m.BubbleH <= 0 {
		t.Fatal("rectangular cell size missing from manifest")
	}
	if m.BubbleW > m.BubbleH {
		t.Fatalf("cells must be wide rectangles, got w=%f h=%f", m.BubbleW, m.BubbleH)
	}
	plan, err := englishV4Grid(m.Questions)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Cols != 6 || plan.Rows != 3 {
		t.Fatalf("15 questions should use 6 columns of 3 rows, got %d/%d", plan.Cols, plan.Rows)
	}
	// One horizontal row per question: every option of a question shares a Y.
	byQuestion := map[string][]Bubble{}
	for _, b := range m.Bubbles {
		byQuestion[b.QuestionID] = append(byQuestion[b.QuestionID], b)
	}
	for id, bubbles := range byQuestion {
		for _, b := range bubbles {
			if b.Y != bubbles[0].Y {
				t.Fatalf("question %s is not printed on one horizontal row", id)
			}
		}
		for i := 1; i < len(bubbles); i++ {
			if bubbles[i].X <= bubbles[i-1].X {
				t.Fatalf("question %s options are not left to right", id)
			}
		}
	}
}

func TestEnglishV4FullSixtyFitsWithSevenOptions(t *testing.T) {
	full := englishV4Sample(t, map[string]int{"听力": 20, "阅读": 15, "七选五": 5, "知识运用": 20})
	m, err := BuildEnglishManifestV4(full)
	if err != nil {
		t.Fatalf("60-question paper must fit the new layout: %v", err)
	}
	plan, err := englishV4Grid(m.Questions)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Cols != 4 || plan.Rows != 15 {
		t.Fatalf("expected 4 columns x 15 rows, got %d/%d", plan.Cols, plan.Rows)
	}
	for _, b := range m.Bubbles {
		if b.Y*148.5 > 128.5 || b.X*210 > 201 {
			t.Fatalf("cell outside printable area: %+v", b)
		}
	}
	if _, err := RenderTwoUpSVG(m, "英语", []PrintCard{{StudentLabel: "1932 张三", Token: "exam-release-token-0001"}, {StudentLabel: "1933 李四", Token: "exam-release-token-0001"}}); err != nil {
		t.Fatal(err)
	}
}

func TestEnglishV4RejectsTooManyQuestions(t *testing.T) {
	qs := englishV4Sample(t, map[string]int{"听力": 40, "阅读": 21})
	if _, err := BuildEnglishManifestV4(qs); err == nil {
		t.Fatal("61 questions accepted")
	}
}

// englishV4FilledCard fills whole rectangles the way a student would.
func englishV4FilledCard(t *testing.T, m Manifest, number string, picked map[string][]string) image.Image {
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
	halfW, halfH := m.BubbleW/2*w*.92, m.BubbleH/2*h*.92
	fill := func(cx, cy float64) {
		for yy := int(cy - halfH - 1); yy <= int(cy+halfH)+1; yy++ {
			for xx := int(cx - halfW - 1); xx <= int(cx+halfW)+1; xx++ {
				img.Set(xx, yy, color.Black)
			}
		}
	}
	for _, d := range m.StudentBubbles {
		if d.Column >= 0 && d.Column < len(number) && int(number[d.Column]-'0') == d.Digit {
			fill(d.X*w, d.Y*h)
		}
	}
	for _, b := range m.Bubbles {
		for _, want := range picked[b.QuestionID] {
			if b.Option == want {
				fill(b.X*w, b.Y*h)
			}
		}
	}
	return img
}

func TestEnglishV4RecognizeRectangularCells(t *testing.T) {
	qs := englishV4Sample(t, map[string]int{"听力": 20, "阅读": 15, "七选五": 5, "知识运用": 20})
	m, err := BuildEnglishManifestV4(qs)
	if err != nil {
		t.Fatal(err)
	}
	picked := map[string][]string{}
	for i, q := range m.Questions {
		if i%3 == 0 {
			picked[q.ID] = []string{q.Options[len(q.Options)-1]}
		}
	}
	r, err := Recognize(englishV4FilledCard(t, m, "1932", picked), m)
	if err != nil {
		t.Fatalf("recognize: %v (%+v)", err, r.Quality)
	}
	if r.IdentityState != "recognized" || r.StudentNumber != "1932" {
		t.Fatalf("identity %q state %q", r.StudentNumber, r.IdentityState)
	}
	answered, blank := 0, 0
	for _, a := range r.Answers {
		if a.State == "uncertain" || a.State == "invalid_multiple" {
			t.Fatalf("question %s wrongly ambiguous: %+v", a.QuestionID, a)
		}
		if len(picked[a.QuestionID]) == 0 {
			blank++
			continue
		}
		answered++
	}
	if answered == 0 || blank == 0 {
		t.Fatalf("coverage %d/%d", answered, blank)
	}
}

func TestEnglishV4BlankCellsStayBlank(t *testing.T) {
	qs := englishV4Sample(t, map[string]int{"听力": 10, "阅读": 10})
	m, err := BuildEnglishManifestV4(qs)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Recognize(englishV4FilledCard(t, m, "1932", nil), m)
	if err != nil {
		t.Fatalf("recognize blank: %v", err)
	}
	for _, a := range r.Answers {
		if a.State != "blank" {
			t.Fatalf("printed letter made a blank rectangular cell ambiguous: %+v", a)
		}
	}
}
