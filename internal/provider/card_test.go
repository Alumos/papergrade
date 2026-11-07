package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func cardBytes(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if e := png.Encode(&b, image.NewGray(image.Rect(0, 0, 200, 300))); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestCloudCardContract(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if e := r.ParseMultipartForm(2 << 20); e != nil {
			t.Fatal(e)
		}
		defer r.MultipartForm.RemoveAll()
		if r.FormValue("template") == "" {
			t.Error("missing template")
		}
		f, _, e := r.FormFile("image")
		if e != nil {
			t.Error(e)
		} else {
			f.Close()
		}
		json.NewEncoder(w).Encode(CardRecognition{Token: "opaque", Answers: []CardAnswer{{QuestionID: "q", Selected: []string{"A"}, State: "answered", Confidence: .9}}})
	}))
	defer s.Close()
	c := New(Config{CardURL: s.URL})
	out, e := c.RecognizeCard(context.Background(), cardBytes(t), map[string]string{"version": "v1"}, []CardQuestion{{ID: "q", Type: "single_choice", Options: []string{"A", "B"}}})
	if e != nil || out.Token != "opaque" {
		t.Fatal(e, out)
	}
}
func TestCloudRejectsMalformedAnswers(t *testing.T) {
	cases := map[string]CardRecognition{
		"missing_identity":   {Answers: []CardAnswer{{QuestionID: "q", State: "blank"}}},
		"missing_answers":    {Token: "token"},
		"unknown_option":     {Token: "token", Answers: []CardAnswer{{QuestionID: "q", State: "answered", Selected: []string{"Z"}}}},
		"duplicate_option":   {Token: "token", Answers: []CardAnswer{{QuestionID: "q", State: "answered", Selected: []string{"A", "A"}}}},
		"blank_selection":    {Token: "token", Answers: []CardAnswer{{QuestionID: "q", State: "blank", Selected: []string{"A"}}}},
		"empty_answered":     {Token: "token", Answers: []CardAnswer{{QuestionID: "q", State: "answered"}}},
		"unknown_state":      {Token: "token", Answers: []CardAnswer{{QuestionID: "q", State: "guess"}}},
		"invalid_confidence": {Token: "token", Answers: []CardAnswer{{QuestionID: "q", State: "blank", Confidence: 1.1}}},
	}
	for name, response := range cases {
		t.Run(name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(response) }))
			defer s.Close()
			_, err := New(Config{CardURL: s.URL}).RecognizeCard(context.Background(), cardBytes(t), nil, []CardQuestion{{ID: "q", Type: "single_choice", Options: []string{"A", "B"}}})
			if err == nil {
				t.Fatal("malformed answer accepted")
			}
		})
	}
}
func TestCloudDoesNotFollowRedirect(t *testing.T) {
	called := false
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer destination.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, 307) }))
	defer source.Close()
	_, err := New(Config{CardURL: source.URL, CardKey: "private-key"}).RecognizeCard(context.Background(), cardBytes(t), nil, []CardQuestion{{ID: "q", Options: []string{"A", "B"}}})
	if err == nil || called || strings.Contains(err.Error(), "private-key") {
		t.Fatal("unsafe redirect", err)
	}
}
func TestCloudTimeoutIsBounded(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(100 * time.Millisecond):
		}
	}))
	defer s.Close()
	_, err := New(Config{CardURL: s.URL, Timeout: 10 * time.Millisecond}).RecognizeCard(context.Background(), cardBytes(t), nil, []CardQuestion{{ID: "q", Options: []string{"A", "B"}}})
	if err == nil {
		t.Fatal("timeout ignored")
	}
}
func TestCloudRejectsUnknownQuestion(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(CardRecognition{Token: "opaque", Answers: []CardAnswer{{QuestionID: "evil", State: "blank"}}})
	}))
	defer s.Close()
	c := New(Config{CardURL: s.URL})
	if _, e := c.RecognizeCard(context.Background(), cardBytes(t), nil, []CardQuestion{{ID: "q", Type: "single_choice", Options: []string{"A", "B"}}}); e == nil {
		t.Fatal("unknown question accepted")
	}
}
