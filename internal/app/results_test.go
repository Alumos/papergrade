package app

import (
	"github.com/papergrade/papergrade/internal/grading"
	"testing"
)

func TestSummaryAndQuestionStatistics(t *testing.T) {
	q := grading.Question{ID: "q", Number: "1", Type: "single_choice", Options: []string{"A", "B"}, Answer: []string{"A"}, ScoreUnits: 100}
	ex := Exam{StudentCount: 4, Questions: []grading.Question{q}}
	rs := []Result{{Status: "committed", Score: 100, Answers: []grading.Answer{{QuestionID: "q", Selected: []string{"A"}, State: "answered"}}}, {Status: "needs_review", Score: 0, Answers: []grading.Answer{{QuestionID: "q", Selected: []string{"A"}, State: "uncertain"}}}, {Status: "committed", Score: 0, Answers: []grading.Answer{{QuestionID: "q", Selected: []string{}, State: "blank"}}}}
	s := summary(ex, rs)
	if s["expected"] != 4 || s["received"] != 3 || s["missing"] != 1 || s["needs_review"] != 1 || s["average_units"] != float64(50) {
		t.Fatal(s)
	}
	stats := questionStats(ex, rs)
	if len(stats) != 1 || stats[0]["full_score_count"] != 1 || stats[0]["blank_count"] != 1 || stats[0]["uncertain_count"] != 1 {
		t.Fatal(stats)
	}
}
