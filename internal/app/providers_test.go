package app

import (
	"context"
	"encoding/json"
	"github.com/papergrade/papergrade/internal/provider"
	"github.com/papergrade/papergrade/internal/store"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }
func TestProviderSettingsMaskedEncryptedCASAndClear(t *testing.T) {
	h := setup(t)
	h.login()
	get := func() map[string]any {
		return parse[map[string]any](t, h.request("GET", "/api/v1/settings/providers", nil, 200))
	}
	initial := get()
	if initial["source"] != "environment" || initial["encryption_ready"] != true {
		t.Fatal(initial)
	}
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer endpoint.Close()
	put := func(body any, status int) map[string]any {
		return parse[map[string]any](t, h.request("PUT", "/api/v1/settings/providers", body, status))
	}
	saved := put(map[string]any{"revision": initial["revision"], "ocr": map[string]any{"url": endpoint.URL, "secret": "ocr-secret-value"}, "ai": map[string]any{"base_url": "", "model": ""}, "card": map[string]any{"url": ""}}, 200)
	if saved["revision"] != float64(1) || strings.Contains(string(mustJSON(saved)), "ocr-secret-value") {
		t.Fatal("secret leaked", saved)
	}
	var row store.Record
	h.a.Store.View(context.Background(), func(tx *store.Tx) error {
		rows, e := tx.List("settings", "")
		if e == nil && len(rows) == 1 {
			row = rows[0]
		}
		return e
	})
	if strings.Contains(string(row.Data), "ocr-secret-value") {
		t.Fatal("encrypted secret appears in DB JSON")
	}
	h.request("PUT", "/api/v1/settings/providers", map[string]any{"revision": initial["revision"], "ocr": map[string]any{}, "ai": map[string]any{}, "card": map[string]any{}}, 409)
	cleared := put(map[string]any{"revision": saved["revision"], "ocr": map[string]any{"url": endpoint.URL, "clear_secret": true}, "ai": map[string]any{"base_url": "", "model": ""}, "card": map[string]any{"url": ""}}, 200)
	ocr := cleared["ocr"].(map[string]any)
	if ocr["has_secret"] != false {
		t.Fatal("secret not cleared", cleared)
	}
	h.request("POST", "/api/v1/settings/providers/test", map[string]string{"provider": "ocr"}, 200)
	h.request("POST", "/api/v1/settings/providers/test", map[string]string{"provider": "card"}, 503)
}
func TestProviderSettingsPersistAcrossRead(t *testing.T) {
	h := setup(t)
	h.login()
	initial := parse[map[string]any](t, h.request("GET", "/api/v1/settings/providers", nil, 200))
	h.request("PUT", "/api/v1/settings/providers", map[string]any{"revision": initial["revision"], "ocr": map[string]any{"url": "https://ocr.example.test"}, "ai": map[string]any{"base_url": "https://ai.example.test", "model": "model"}, "card": map[string]any{"url": "https://card.example.test"}}, 200)
	var cfg provider.Config
	var rev int
	var source string
	e := h.a.Store.View(context.Background(), func(tx *store.Tx) error { var err error; cfg, rev, source, err = h.a.configIn(tx); return err })
	if e != nil || rev != 1 || source != "database" || cfg.AIModel != "model" {
		t.Fatal(rev, source, cfg, e)
	}
}
func TestEncryptionKeyRequiredOutsideEphemeralDev(t *testing.T) {
	if _, e := New(context.Background(), Config{Dev: false, DatabaseURL: "postgres://invalid", Username: "teacher", Password: "long-password-here", PublicOrigin: "https://grade.example"}); e == nil || !strings.Contains(e.Error(), "APP_ENCRYPTION_KEY") {
		t.Fatal("production key missing not rejected", e)
	}
	if _, e := New(context.Background(), Config{Dev: true, DatabaseURL: "postgres://invalid", Username: "teacher", Password: "long-password-here", PublicOrigin: "http://localhost"}); e == nil || !strings.Contains(e.Error(), "APP_ENCRYPTION_KEY") {
		t.Fatal("dev postgres key missing not rejected", e)
	}
}
