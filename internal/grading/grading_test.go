package grading

import "testing"

func TestRules(t *testing.T) {
	q := Question{ID: "q", Number: "1", Type: "multiple_choice", Options: []string{"A", "B", "C"}, Answer: []string{"A", "B"}, ScoreUnits: 200, PartialScoreUnits: 50}
	cases := []struct {
		selected       []string
		state          string
		score, reviews int
	}{{[]string{"B", "A"}, "answered", 200, 0}, {[]string{"A"}, "answered", 50, 0}, {[]string{"A", "C"}, "answered", 0, 0}, {[]string{}, "blank", 0, 0}, {[]string{"A"}, "uncertain", 0, 1}}
	for _, c := range cases {
		g, e := Calculate([]Question{q}, []Answer{{QuestionID: "q", Selected: c.selected, State: c.state}})
		if e != nil || g.Score != c.score || g.Reviews != c.reviews || g.Max != 200 {
			t.Fatalf("%+v: %+v %v", c, g, e)
		}
	}
}
func TestValidation(t *testing.T) {
	q := Question{ID: "q", Number: "01", Type: "single_choice", Options: []string{"A", "B"}, Answer: []string{"A"}, ScoreUnits: 100}
	if e := Validate([]Question{q}, true); e != nil {
		t.Fatal(e)
	}
	if e := Validate([]Question{q, q}, true); e == nil {
		t.Fatal("duplicate accepted")
	}
	if _, e := Calculate([]Question{q}, []Answer{{QuestionID: "q", Selected: []string{"X"}, State: "answered"}}); e == nil {
		t.Fatal("unknown option")
	}
	g, e := Calculate([]Question{q}, nil)
	if e != nil || g.Reviews != 1 {
		t.Fatal("missing answer must review", g, e)
	}
	q.Answer = nil
	if e = Validate([]Question{q}, true); e == nil {
		t.Fatal("missing key accepted")
	}
}
