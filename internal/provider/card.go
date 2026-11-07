package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"strings"
)

// CardQuestion deliberately excludes the answer key and scores.
type CardQuestion struct {
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	Options []string `json:"options"`
}
type CardAnswer struct {
	QuestionID string   `json:"question_id"`
	Selected   []string `json:"selected"`
	State      string   `json:"state"`
	Confidence float64  `json:"confidence"`
}
type CardRecognition struct {
	Token    string       `json:"token"`
	Answers  []CardAnswer `json:"answers"`
	Warnings []string     `json:"warnings"`
}

// RecognizeCard streams a bounded multipart upload to a configured card-reading
// gateway. An ordinary OCR endpoint is NOT sufficient unless it implements this
// contract. The gateway must verify the image's markers and QR independently;
// candidate identity/answers from the browser are never authoritative.
func (c *Client) RecognizeCard(ctx context.Context, raw []byte, manifest any, questions []CardQuestion) (CardRecognition, error) {
	if c.cfg.CardURL == "" {
		return CardRecognition{}, ErrNotConfigured
	}
	if len(raw) == 0 || len(raw) > 4<<20 {
		return CardRecognition{}, errors.New("card image outside size limit")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || (format != "jpeg" && format != "png") || cfg.Width < 100 || cfg.Height < 100 || int64(cfg.Width)*int64(cfg.Height) > 8000000 {
		return CardRecognition{}, errors.New("card image dimensions or type invalid")
	}
	if len(questions) == 0 || len(questions) > 40 {
		return CardRecognition{}, errors.New("unsupported question count")
	}
	spec, err := json.Marshal(map[string]any{"schema_version": "card-v1", "manifest": manifest, "questions": questions})
	if err != nil || len(spec) > 128<<10 {
		return CardRecognition{}, errors.New("invalid template contract")
	}
	r, w := io.Pipe()
	form := multipart.NewWriter(w)
	go func() {
		var writeErr error
		defer func() {
			if writeErr != nil {
				_ = w.CloseWithError(writeErr)
			} else {
				_ = w.Close()
			}
		}()
		if writeErr = form.WriteField("template", string(spec)); writeErr != nil {
			return
		}
		var part io.Writer
		part, writeErr = form.CreateFormFile("image", "card."+format)
		if writeErr != nil {
			return
		}
		if _, writeErr = part.Write(raw); writeErr != nil {
			return
		}
		writeErr = form.Close()
	}()
	defer r.Close()
	var out CardRecognition
	if err = c.requestReader(ctx, c.cfg.CardURL, c.cfg.CardKey, form.FormDataContentType(), r, &out); err != nil {
		return CardRecognition{}, err
	}
	if strings.TrimSpace(out.Token) == "" || len(out.Token) > 512 {
		return CardRecognition{}, errors.New("cloud card identity missing or invalid")
	}
	if len(out.Answers) != len(questions) {
		return CardRecognition{}, errors.New("cloud response missing or adding questions")
	}
	allowed := make(map[string]CardQuestion, len(questions))
	for _, q := range questions {
		allowed[q.ID] = q
	}
	seen := map[string]bool{}
	for i := range out.Answers {
		a := &out.Answers[i]
		q, ok := allowed[a.QuestionID]
		if !ok || seen[a.QuestionID] {
			return CardRecognition{}, errors.New("cloud response has unknown/duplicate question")
		}
		seen[a.QuestionID] = true
		if a.Confidence < 0 || a.Confidence > 1 {
			return CardRecognition{}, errors.New("invalid cloud confidence")
		}
		labels := map[string]bool{}
		for _, v := range q.Options {
			labels[v] = true
		}
		selected := map[string]bool{}
		for _, v := range a.Selected {
			if !labels[v] || selected[v] {
				return CardRecognition{}, errors.New("cloud selected invalid option")
			}
			selected[v] = true
		}
		switch a.State {
		case "answered":
			if len(a.Selected) == 0 {
				return CardRecognition{}, errors.New("answered without selection")
			}
			if q.Type != "multiple_choice" && len(a.Selected) > 1 {
				a.State = "invalid_multiple"
			}
		case "blank":
			if len(a.Selected) != 0 {
				return CardRecognition{}, errors.New("blank with selection")
			}
		case "invalid_multiple":
			if len(a.Selected) < 2 {
				return CardRecognition{}, errors.New("invalid multiple state")
			}
		case "uncertain":
		default:
			return CardRecognition{}, errors.New("unknown cloud answer state")
		}
	}
	return out, nil
}
