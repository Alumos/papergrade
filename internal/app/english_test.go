package app

import (
	"strings"
	"testing"

	"github.com/papergrade/papergrade/internal/grading"
)

// englishExam publishes a 60-question English paper with the usual mix: short
// three-option listening, four-option reading, a seven-option "choose five of
// seven" group and four-option language use.
func (h *harness) englishExam(number string) Exam {
	h.t.Helper()
	cls := parse[Class](h.t, h.request("POST", "/api/v1/classes", map[string]string{"name": "英语班 " + number}, 201))
	h.request("POST", "/api/v1/classes/"+cls.ID+"/students", map[string]any{"students": []map[string]string{{"number": number, "name": "学生" + number}}}, 201)
	questions := []grading.Question{}
	add := func(n int, options []string, section string) {
		for i := 0; i < n; i++ {
			questions = append(questions, grading.Question{
				Number:     "",
				Type:       "single_choice",
				Options:    options,
				Answer:     []string{options[0]},
				ScoreUnits: 200,
				Section:    section,
			})
		}
	}
	add(20, []string{"A", "B", "C"}, "第一部分 听力（1–20）")
	add(15, []string{"A", "B", "C", "D"}, "第二部分 阅读理解（21–35）")
	add(5, []string{"A", "B", "C", "D", "E", "F", "G"}, "第三部分 七选五（36–40）")
	add(20, []string{"A", "B", "C", "D"}, "第四部分 英语知识运用（41–60）")
	for i := range questions {
		questions[i].Number = func(n int) string { return strings.TrimSpace(strings.Repeat("", 0) + itoa(n)) }(i + 1)
	}
	ex := parse[Exam](h.t, h.request("POST", "/api/v1/exams", examInput{Title: "英语期中", Subject: "英语", ClassID: cls.ID, Questions: questions}, 201))
	return parse[Exam](h.t, h.request("POST", "/api/v1/exams/"+ex.ID+"/publish", map[string]int{"revision": ex.Revision}, 200))
}

func itoa(n int) string {
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

func TestEnglishPublishUsesFixedFourDigitCard(t *testing.T) {
	h := setup(t)
	h.login()
	ex := h.englishExam("1932")
	if ex.Template == nil || ex.Template.Version != "a5-v3" {
		t.Fatalf("expected a5-v3 English card, got %+v", ex.Template)
	}
	if ex.StudentNumberDigits != 4 || ex.Template.StudentNumberDigits != 4 {
		t.Fatalf("English card must freeze four digits: %d/%d", ex.StudentNumberDigits, ex.Template.StudentNumberDigits)
	}
	if len(ex.Template.Questions) != 60 {
		t.Fatalf("expected 60 printed questions, got %d", len(ex.Template.Questions))
	}
	seven := 0
	for _, q := range ex.Template.Questions {
		if len(q.Options) == 7 {
			seven++
		}
	}
	if seven != 5 {
		t.Fatalf("expected five seven-option questions, got %d", seven)
	}
	if ex.Template.Questions[0].Section == "" {
		t.Fatal("section headings were dropped from the template")
	}
	sheets := parse[struct {
		Items []Sheet `json:"items"`
	}](h.t, h.request("GET", "/api/v1/exams/"+ex.ID+"/sheets", nil, 200))
	var sheet Sheet
	for _, s := range sheets.Items {
		if s.StudentNumber == "1932" {
			sheet = s
		}
	}
	if sheet.StudentID == "" {
		h.t.Fatal("published English sheet not listed")
	}
	svg := h.request("GET", "/api/v1/exams/"+ex.ID+"/print-sheet?student_ids="+sheet.StudentID, nil, 200).Body.String()
	if !strings.Contains(svg, "a5-v3") || !strings.Contains(svg, "第一部分") || !strings.Contains(svg, "七选五") {
		t.Fatal("printed sheet is missing the English card or its group headings")
	}
}

func TestEnglishPublishRequiresFourDigitRoster(t *testing.T) {
	h := setup(t)
	h.login()
	cls := parse[Class](h.t, h.request("POST", "/api/v1/classes", map[string]string{"name": "英语三位班"}, 201))
	h.request("POST", "/api/v1/classes/"+cls.ID+"/students", map[string]any{"students": []map[string]string{{"number": "132", "name": "三位学号"}}}, 201)
	qs := []grading.Question{{Number: "1", Type: "single_choice", Options: []string{"A", "B", "C"}, Answer: []string{"A"}, ScoreUnits: 200, Section: "听力"}}
	ex := parse[Exam](h.t, h.request("POST", "/api/v1/exams", examInput{Title: "英语", Subject: "英语", ClassID: cls.ID, Questions: qs}, 201))
	h.request("POST", "/api/v1/exams/"+ex.ID+"/publish", map[string]int{"revision": ex.Revision}, 400)
}

func TestEnglishPublishRejectsOverCapacityCard(t *testing.T) {
	h := setup(t)
	h.login()
	cls := parse[Class](h.t, h.request("POST", "/api/v1/classes", map[string]string{"name": "英语超量班"}, 201))
	h.request("POST", "/api/v1/classes/"+cls.ID+"/students", map[string]any{"students": []map[string]string{{"number": "1932", "name": "学生"}}}, 201)
	questions := []grading.Question{}
	for i := 0; i < 60; i++ {
		questions = append(questions, grading.Question{
			Number: itoa(i + 1), Type: "single_choice",
			Options: []string{"A", "B", "C", "D", "E", "F", "G"}, Answer: []string{"A"},
			ScoreUnits: 200, Section: "七选五",
		})
	}
	ex := parse[Exam](h.t, h.request("POST", "/api/v1/exams", examInput{Title: "英语超量", Subject: "英语", ClassID: cls.ID, Questions: questions}, 201))
	h.request("POST", "/api/v1/exams/"+ex.ID+"/publish", map[string]int{"revision": ex.Revision}, 400)
}
