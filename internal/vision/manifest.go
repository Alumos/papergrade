package vision

import (
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"
)

func BuildManifest(questions []Question) (Manifest, error) {
	if len(questions) < 1 || len(questions) > MaxQuestions {
		return Manifest{}, fmt.Errorf("%w: require 1..40 questions", ErrInvalidManifest)
	}
	m := Manifest{Version: Version, CoordinateSpace: "normalized", WidthMM: 210, HeightMM: 297, MarkerSize: .024,
		Markers: []Point{{.05, .05}, {.95, .05}, {.95, .95}, {.05, .95}},
		QRBox:   Box{.76, .075, .17, .17 * 210 / 297}, BlackReference: Box{.56, .115, .035, .035 * 210 / 297}, WhiteReference: Box{.61, .115, .035, .035 * 210 / 297},
		Questions: make([]Question, 0, len(questions)), Bubbles: make([]Bubble, 0, len(questions)*5)}
	ids := map[string]bool{}
	for i, q := range questions {
		if strings.TrimSpace(q.ID) == "" || len(q.ID) > 200 || ids[q.ID] || !validText(q.ID) || strings.TrimSpace(q.Number) == "" || !validText(q.Number) || utf8.RuneCountInString(q.Number) > 12 {
			return Manifest{}, fmt.Errorf("%w: invalid/duplicate question identity", ErrInvalidManifest)
		}
		ids[q.ID] = true
		if q.Type != "single_choice" && q.Type != "multiple_choice" && q.Type != "true_false" {
			return Manifest{}, fmt.Errorf("%w: unknown question type", ErrInvalidManifest)
		}
		if len(q.Options) < 2 || len(q.Options) > 5 || (q.Type == "true_false" && len(q.Options) != 2) {
			return Manifest{}, fmt.Errorf("%w: require 2..5 options (true_false: 2)", ErrInvalidManifest)
		}
		seen := map[string]bool{}
		for _, o := range q.Options {
			if strings.TrimSpace(o) == "" || !validText(o) || utf8.RuneCountInString(o) > 8 || seen[o] {
				return Manifest{}, fmt.Errorf("%w: invalid/duplicate option", ErrInvalidManifest)
			}
			seen[o] = true
		}
		q.Options = append([]string(nil), q.Options...)
		m.Questions = append(m.Questions, q)
		column, row := i/20, i%20
		for j, o := range q.Options {
			m.Bubbles = append(m.Bubbles, Bubble{q.ID, o, .17 + float64(column)*.43 + float64(j)*.065, .25 + float64(row)*.031, .009})
		}
	}
	return m, nil
}

// Validate by reconstruction: callers cannot move sampling regions or inject
// extra questions into an otherwise version-matching manifest.
func validateManifest(m Manifest) error {
	canonical, err := BuildManifest(m.Questions)
	if err != nil || !reflect.DeepEqual(m, canonical) {
		return ErrInvalidManifest
	}
	return nil
}
func validText(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r < 32 && r != '\n' && r != '\t' && r != '\r' {
			return false
		}
		if r == 0xfffe || r == 0xffff {
			return false
		}
	}
	return true
}
