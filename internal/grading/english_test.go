package grading

import "testing"

func TestEnglishValidateMax60SevenOptionsAndSection(t *testing.T) {
	qs := make([]Question, 60)
	for i := range qs {
		opts := []string{"A", "B", "C", "D"}
		if i == 40 {
			opts = []string{"A", "B", "C", "D", "E", "F", "G"}
		}
		qs[i] = Question{ID: fmtID(i), Number: fmtNum(i), Type: "single_choice", Options: opts, Answer: []string{opts[0]}, ScoreUnits: 100, Section: "Reading"}
	}
	if e := Validate(qs, true); e != nil {
		t.Fatal(e)
	}
	if e := Validate(append(qs, Question{ID: "overflow", Number: "61", Type: "single_choice", Options: []string{"A", "B"}, Answer: []string{"A"}, ScoreUnits: 100}), false); e == nil {
		t.Fatal("overflow accepted")
	}
}
func fmtID(i int) string { return "q" + fmtNum(i) }
func fmtNum(i int) string {
	if i < 9 {
		return "0" + string(rune('1'+i))
	}
	return string(rune('1'+i/10)) + string(rune('0'+i%10))
}
