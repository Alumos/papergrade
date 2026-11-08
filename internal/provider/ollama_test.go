package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func imageFixture(t *testing.T) []byte {
	var b bytes.Buffer
	if e := png.Encode(&b, image.NewGray(image.Rect(0, 0, 64, 64))); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestNativeOllamaProtocolOptionalKeyAndPrompt(t *testing.T) {
	raw := imageFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" || r.Method != "POST" || r.Header.Get("Authorization") != "" {
			t.Error("native request mismatch")
		}
		var b struct {
			Model         string
			Think, Stream bool
			Messages      []struct {
				Content string
				Images  []string
			}
			Options map[string]int
		}
		if e := json.NewDecoder(r.Body).Decode(&b); e != nil {
			t.Fatal(e)
		}
		if b.Model != "ocr-model" || b.Think || b.Stream || len(b.Messages) != 1 || !strings.Contains(b.Messages[0].Content, "Tables should be expressed in OTSL format") || b.Options["num_ctx"] != 8192 || b.Options["num_predict"] != 4096 {
			t.Error("official request contract", b)
		}
		decoded, _ := base64.StdEncoding.DecodeString(b.Messages[0].Images[0])
		if !bytes.Equal(decoded, raw) {
			t.Error("image altered")
		}
		json.NewEncoder(w).Encode(map[string]any{"done": true, "done_reason": "stop", "message": map[string]string{"content": "Question with $\\frac{1}{2}$"}})
	}))
	defer server.Close()
	client := New(Config{OCRType: "ollama", OCRURL: server.URL, OCRModel: "ocr-model"})
	if client.http.Timeout != 90*time.Second {
		t.Fatal("AI/shared timeout must stay90; native clone raises OCR only")
	}
	out, e := client.Extract(context.Background(), "paper.png", raw)
	if e != nil || out.Source != "ollama" || out.Pages != 1 {
		t.Fatal(out, e)
	}
	if _, e = client.Extract(context.Background(), "paper.pdf", []byte("%PDF-1.4")); e == nil {
		t.Fatal("native PDF accepted")
	}
}
func TestOllamaRejectsIncompleteAndRedirects(t *testing.T) {
	for _, reason := range []string{"length", "max_tokens"} {
		t.Run(reason, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(map[string]any{"done": true, "done_reason": reason, "message": map[string]string{"content": "partial"}})
			}))
			defer s.Close()
			if _, e := New(Config{OCRType: "ollama", OCRURL: s.URL, OCRModel: "ocr"}).Extract(context.Background(), "x.png", imageFixture(t)); e == nil {
				t.Fatal("truncated OCR accepted")
			}
		})
	}
}
func TestOllamaPrivateHTTPPolicy(t *testing.T) {
	for _, url := range []string{"http://192.168.2.3:11434", "http://10.2.3.4:11434", "http://100.80.1.2:11434"} {
		if e := ValidateConfig(Config{OCRType: "ollama", OCRURL: url}); e != nil {
			t.Fatal(e)
		}
		if e := ValidateConfig(Config{OCRURL: url}); e == nil {
			t.Fatal("generic gateway privateHTTP accepted")
		}
	}
	for _, url := range []string{"http://nas.example:11434", "http://8.8.8.8:11434", "http://169.254.169.254"} {
		if e := ValidateConfig(Config{OCRType: "ollama", OCRURL: url}); e == nil {
			t.Fatal("unsafe HTTP accepted", url)
		}
	}
	if directOllamaTransport.Proxy != nil {
		t.Fatal("private Ollama must bypass environment proxy")
	}
}
