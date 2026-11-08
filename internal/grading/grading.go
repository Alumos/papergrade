// Package grading provides deterministic integer-unit objective grading.
package grading

import (
	"fmt"
	"sort"
)

type Question struct {
	ID                string   `json:"id"`
	Number            string   `json:"number"`
	Type              string   `json:"type"`
	Options           []string `json:"options"`
	Answer            []string `json:"answer"`
	ScoreUnits        int      `json:"score_units"`
	PartialScoreUnits int      `json:"partial_score_units,omitempty"`
	ReviewFlags       []string `json:"review_flags,omitempty"`
	Section           string   `json:"section,omitempty"`
}
type Answer struct {
	QuestionID string   `json:"question_id"`
	Selected   []string `json:"selected"`
	State      string   `json:"state"`
	Confidence float64  `json:"confidence,omitempty"`
}
type Grade struct{ Score, Max, Reviews int }

func Set(a []string) []string { b := append([]string{}, a...); sort.Strings(b); return b }
func Equal(a, b []string) bool {
	a = Set(a)
	b = Set(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func Validate(qs []Question, publish bool) error {
	if len(qs) > 60 || (publish && len(qs) == 0) {
		return fmt.Errorf("require 1–60 questions to publish")
	}
	numbers := map[string]bool{}
	ids := map[string]bool{}
	for _, q := range qs {
		if q.Number == "" || numbers[q.Number] || q.ID == "" || ids[q.ID] {
			return fmt.Errorf("question numbers and IDs must be unique")
		}
		numbers[q.Number] = true
		ids[q.ID] = true
		if q.Type != "single_choice" && q.Type != "multiple_choice" && q.Type != "true_false" {
			return fmt.Errorf("unsupported question type")
		}
		if len(q.Options) < 2 || len(q.Options) > 7 || (q.Type == "true_false" && len(q.Options) != 2) {
			return fmt.Errorf("require 2–7 distinct options (2 for true_false)")
		}
		options := map[string]bool{}
		for _, v := range q.Options {
			if v == "" || options[v] {
				return fmt.Errorf("invalid option")
			}
			options[v] = true
		}
		seen := map[string]bool{}
		for _, v := range q.Answer {
			if !options[v] || seen[v] {
				return fmt.Errorf("answer is not a unique option")
			}
			seen[v] = true
		}
		if len(q.Answer) > 1 && q.Type != "multiple_choice" {
			return fmt.Errorf("single answer required")
		}
		if publish && (len(q.Answer) == 0 || q.ScoreUnits <= 0) {
			return fmt.Errorf("all answers and positive scores required")
		}
		if q.ScoreUnits < 0 || q.ScoreUnits > 1000000 || q.PartialScoreUnits < 0 || q.PartialScoreUnits > q.ScoreUnits || (q.Type != "multiple_choice" && q.PartialScoreUnits != 0) {
			return fmt.Errorf("invalid score units")
		}
	}
	return nil
}
func Calculate(qs []Question, answers []Answer) (Grade, error) {
	g := Grade{}
	by := map[string]Answer{}
	valid := map[string]bool{}
	for _, q := range qs {
		valid[q.ID] = true
	}
	for _, a := range answers {
		if !valid[a.QuestionID] {
			return g, fmt.Errorf("unknown question")
		}
		if _, ok := by[a.QuestionID]; ok {
			return g, fmt.Errorf("duplicate answer")
		}
		by[a.QuestionID] = a
	}
	for _, q := range qs {
		g.Max += q.ScoreUnits
		a, ok := by[q.ID]
		if !ok {
			g.Reviews++
			continue
		}
		if a.State != "answered" && a.State != "blank" && a.State != "invalid_multiple" && a.State != "uncertain" {
			return g, fmt.Errorf("invalid answer state")
		}
		opts := map[string]bool{}
		for _, v := range q.Options {
			opts[v] = true
		}
		seen := map[string]bool{}
		for _, v := range a.Selected {
			if !opts[v] || seen[v] {
				return g, fmt.Errorf("invalid selected option")
			}
			seen[v] = true
		}
		if a.State == "uncertain" {
			g.Reviews++
			continue
		}
		if a.State != "answered" {
			continue
		}
		if q.Type != "multiple_choice" && len(a.Selected) != 1 {
			continue
		}
		if Equal(a.Selected, q.Answer) {
			g.Score += q.ScoreUnits
			continue
		}
		if q.Type == "multiple_choice" && len(a.Selected) > 0 && len(a.Selected) < len(q.Answer) {
			correct := map[string]bool{}
			for _, v := range q.Answer {
				correct[v] = true
			}
			subset := true
			for _, v := range a.Selected {
				if !correct[v] {
					subset = false
				}
			}
			if subset {
				g.Score += q.PartialScoreUnits
			}
		}
	}
	return g, nil
}
