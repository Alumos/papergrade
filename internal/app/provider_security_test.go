package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"github.com/papergrade/papergrade/internal/store"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func providerBody(rev int, url, key string) providerInput {
	return providerInput{Revision: rev, OCR: providerEndpoint{URL: url, Secret: key}, AI: aiEndpoint{}, Card: providerEndpoint{}}
}
func TestProviderSettingsAccessOriginAndValidation(t *testing.T) {
	h := setup(t)
	h.request("GET", "/api/v1/settings/providers", nil, 401)
	h.request("PUT", "/api/v1/settings/providers", providerBody(0, "", ""), 401)
	h.request("POST", "/api/v1/settings/providers/test", map[string]string{"provider": "ocr"}, 401)
	h.login()
	for _, method := range []string{"PUT", "POST"} {
		path := "/api/v1/settings/providers"
		if method == "POST" {
			path += "/test"
		}
		for _, origin := range []string{"", "http://evil.invalid"} {
			r := httptest.NewRequest(method, "http://localhost"+path, bytes.NewReader([]byte("{}")))
			r.AddCookie(h.cookie)
			r.Header.Set("Origin", origin)
			w := httptest.NewRecorder()
			h.h.ServeHTTP(w, r)
			if w.Code != 403 {
				t.Fatalf("%s origin rejected: %d", method, w.Code)
			}
		}
	}
	for _, url := range []string{"https://user:do-not-echo@example.test", "https://example.test?key=do-not-echo", "https://example.test#key=do-not-echo", "http://public.example.test"} {
		w := h.request("PUT", "/api/v1/settings/providers", providerBody(0, url, ""), 400)
		if strings.Contains(w.Body.String(), "do-not-echo") {
			t.Fatal("invalid URL leaked")
		}
	}
}
func TestProviderUnsafeBootstrapCannotLeakThroughGET(t *testing.T) {
	for _, env := range []string{"OCR_ENDPOINT", "AI_BASE_URL", "CARD_ENDPOINT"} {
		t.Run(env, func(t *testing.T) {
			h := setup(t)
			h.login()
			t.Setenv(env, "https://user:bootstrap-secret@example.invalid/path?api_key=bootstrap-secret#secret")
			w := h.request("GET", "/api/v1/settings/providers", nil, 503)
			if strings.Contains(w.Body.String(), "bootstrap-secret") || strings.Contains(w.Body.String(), "example.invalid") {
				t.Fatal("unsafe bootstrap leaked")
			}
			caps := h.a.capabilities(context.Background())
			if caps["provider_settings_available"] != false {
				t.Fatal("invalid bootstrap reported available")
			}
		})
	}
}
func TestProviderSecretPreservationRandomNonceAndTamper(t *testing.T) {
	h := setup(t)
	h.login()
	secret := "unit-test-provider-token"
	h.request("PUT", "/api/v1/settings/providers", providerBody(0, "https://ocr.example.test", secret), 200)
	h.request("PUT", "/api/v1/settings/providers", providerBody(1, "https://new.example.test", ""), 200)
	cfg, rev, source, e := h.a.providerConfig(context.Background())
	if e != nil || cfg.OCRKey != secret || rev != 2 || source != "database" {
		t.Fatal("empty secret did not retain original")
	}
	t.Setenv("OCR_ENDPOINT", "https://env-change.example.test")
	cfg, _, _, e = h.a.providerConfig(context.Background())
	if e != nil || cfg.OCRURL != "https://new.example.test" {
		t.Fatal("environment overrode explicit DB settings")
	}
	one, e := h.a.sealConfig(cfg)
	if e != nil {
		t.Fatal(e)
	}
	two, e := h.a.sealConfig(cfg)
	if e != nil || one == two {
		t.Fatal("nonce reused")
	}
	raw, _ := base64.StdEncoding.DecodeString(one)
	raw[len(raw)-1] ^= 1
	if _, e = h.a.openConfig(base64.StdEncoding.EncodeToString(raw)); e == nil {
		t.Fatal("tamper accepted")
	}
	wrong, _, e := setupEncryption(strings.Repeat("wrong", 8), false, false)
	if e != nil {
		t.Fatal(e)
	}
	other := &App{providerCipher: wrong}
	if _, e = other.openConfig(one); e == nil {
		t.Fatal("wrong key accepted")
	}
	var records []store.Record
	if e = h.a.Store.View(context.Background(), func(tx *store.Tx) error {
		var err error
		records, err = tx.List("settings_audit", "providers")
		return err
	}); e != nil {
		t.Fatal(e)
	}
	for _, r := range records {
		if strings.Contains(string(r.Data), secret) || strings.Contains(string(r.Data), "example.test") {
			t.Fatal("audit contains settings values")
		}
	}
}
func TestProviderConcurrentRevisionSingleWinner(t *testing.T) {
	h := setup(t)
	h.login()
	handler := h.h
	cookie := h.cookie
	start := make(chan struct{})
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			r := httptest.NewRequest("PUT", "http://localhost/api/v1/settings/providers", bytes.NewReader(mustJSON(providerBody(0, "https://ocr.example.test", ""))))
			r.Header.Set("Origin", "http://localhost")
			r.AddCookie(cookie)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			codes <- w.Code
		}()
	}
	close(start)
	wg.Wait()
	close(codes)
	success, conflict := 0, 0
	for code := range codes {
		switch code {
		case 200:
			success++
		case 409:
			conflict++
		default:
			t.Fatal("unexpected CAS code", code)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("CAS allowed multiple winners", success, conflict)
	}
}
func TestProviderConnectionProbeDoesNotInferOrEchoBody(t *testing.T) {
	h := setup(t)
	h.login()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" {
			t.Error("OCR probe attempted inference", r.Method)
		}
		w.WriteHeader(405)
		w.Write([]byte("remote-secret-do-not-echo"))
	}))
	defer server.Close()
	h.request("PUT", "/api/v1/settings/providers", providerBody(0, server.URL, "token"), 200)
	out := parse[map[string]any](t, h.request("POST", "/api/v1/settings/providers/test", map[string]string{"provider": "ocr"}, 200))
	if out["ok"] != false || out["status_code"] != float64(405) || strings.Contains(string(mustJSON(out)), "remote-secret") {
		t.Fatal("probe misrepresented or leaked remote body", out)
	}
}
