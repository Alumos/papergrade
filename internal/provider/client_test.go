package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRejectPublicHTTP(t *testing.T) {
	c := New(Config{OCRURL: "http://public.example.test", OCRKey: "private"})
	_, err := c.Extract(context.Background(), "a.pdf", []byte("%PDF-1.4"))
	if err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatal("public HTTP not rejected", err)
	}
}

func TestNotConfigured(t *testing.T) {
	c := New(Config{})
	if _, e := c.Analyze(context.Background(), "sample"); !errors.Is(e, ErrNotConfigured) {
		t.Fatal(e)
	}
	if _, e := c.Extract(context.Background(), "x.pdf", []byte("notpdf")); e == nil {
		t.Fatal("expected invalid PDF")
	}
}
func TestAnalyze(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer key" {
			t.Error("request contract")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"{\"questions\":[{\"number\":\"1\",\"type\":\"single_choice\",\"options\":[\"A\",\"B\"],\"answer\":[],\"score_units\":null,\"source_page\":1,\"review_flags\":[]}],\"warnings\":[]}"}}],"usage":{"prompt_tokens":10,"completion_tokens":20}}`))
	}))
	defer s.Close()
	c := New(Config{AIBaseURL: s.URL, AIKey: "key", AIModel: "test"})
	a, e := c.Analyze(context.Background(), "untrusted source")
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Questions) != 1 || a.Questions[0].ScoreUnits != nil || a.InputTokens != 10 {
		t.Fatalf("%+v", a)
	}
}
func TestProviderErrorsAreSanitized(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("secret-provider-body"))
	}))
	defer s.Close()
	c := New(Config{OCRURL: s.URL})
	_, e := c.Extract(context.Background(), "a.pdf", []byte("%PDF-broken"))
	if e == nil || strings.Contains(e.Error(), "secret") {
		t.Fatal(e)
	}
}
func TestRejectOversizedModelInput(t *testing.T) {
	c := New(Config{AIBaseURL: "https://example.test", AIKey: "x", AIModel: "m"})
	if _, e := c.Analyze(context.Background(), strings.Repeat("x", maxText+1)); e == nil {
		t.Fatal("unbounded input")
	}
}

// Real OpenAI-compatible models often send bare numbers for `number` and add a
// section label. Both must be accepted, and an English paper may exceed 40
// questions while the printed card still refuses over-capacity layouts.
func TestAnalyzeAcceptsNumericNumberAndSection(t *testing.T) {
	var payload strings.Builder
	payload.WriteString(`{"questions":[`)
	for i := 1; i <= 60; i++ {
		if i > 1 {
			payload.WriteString(",")
		}
		options := `["A","B","C","D"]`
		section := "Reading"
		if i <= 20 {
			options = `["A","B","C"]`
			section = "Listening"
		}
		if i >= 36 && i <= 40 {
			options = `["A","B","C","D","E","F","G"]`
			section = "Seven"
		}
		fmt.Fprintf(&payload, `{"number":%d,"type":"single_choice","options":%s,"answer":[],"score_units":null,"source_page":null,"review_flags":[],"section":"%s"}`, i, options, section)
	}
	payload.WriteString(`],"warnings":[]}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"choices":[{"message":{"content":%s}}],"usage":{"prompt_tokens":5,"completion_tokens":7}}`, jsonString(payload.String()))
	}))
	defer server.Close()
	ry := New(Config{AIBaseURL: server.URL, AIKey: "k", AIModel: "m"})
	a, err := ry.Analyze(context.Background(), "english paper")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Questions) != 60 {
		t.Fatalf("questions %d", len(a.Questions))
	}
	if a.Questions[0].Number != "1" || a.Questions[0].Section != "Listening" || len(a.Questions[0].Options) != 3 {
		t.Fatalf("first question %+v", a.Questions[0])
	}
	if len(a.Questions[36].Options) != 7 || a.Questions[36].Section != "Seven" {
		t.Fatalf("seven-option question %+v", a.Questions[36])
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
