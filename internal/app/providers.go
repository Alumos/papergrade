package app

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/papergrade/papergrade/internal/provider"
	"github.com/papergrade/papergrade/internal/store"
	"net/http"
	"time"
)

const providerAAD = "papergrade:settings/providers:v1"

type sealedProviders struct {
	Revision   int       `json:"revision"`
	Ciphertext string    `json:"ciphertext"`
	UpdatedAt  time.Time `json:"updated_at"`
}
type providerEndpoint struct {
	URL         string `json:"url"`
	Secret      string `json:"secret,omitempty"`
	ClearSecret bool   `json:"clear_secret,omitempty"`
}
type aiEndpoint struct {
	BaseURL     string `json:"base_url"`
	Model       string `json:"model"`
	Secret      string `json:"secret,omitempty"`
	ClearSecret bool   `json:"clear_secret,omitempty"`
}
type providerInput struct {
	Revision int              `json:"revision"`
	OCR      providerEndpoint `json:"ocr"`
	AI       aiEndpoint       `json:"ai"`
	Card     providerEndpoint `json:"card"`
}

func setupEncryption(key string, dev bool, memory bool) (cipher.AEAD, bool, error) {
	ephemeral := false
	if key == "" && !dev {
		return nil, false, fmt.Errorf("APP_ENCRYPTION_KEY is required outside DEV_MODE")
	}
	if key == "" && dev {
		if !memory {
			return nil, false, fmt.Errorf("APP_ENCRYPTION_KEY is required when DEV_MODE uses PostgreSQL")
		}
		raw := make([]byte, 32)
		if _, e := rand.Read(raw); e != nil {
			return nil, false, e
		}
		key = string(raw)
		ephemeral = true
	}
	if len(key) < 32 {
		return nil, false, fmt.Errorf("APP_ENCRYPTION_KEY must contain at least 32 bytes; retain it across API/worker restarts")
	}
	derived := sha256.Sum256([]byte(key))
	block, e := aes.NewCipher(derived[:])
	if e != nil {
		return nil, false, e
	}
	aead, e := cipher.NewGCM(block)
	return aead, ephemeral, e
}
func (a *App) sealConfig(cfg provider.Config) (string, error) {
	if a.providerCipher == nil {
		return "", &APIError{503, "ENCRYPTION_NOT_CONFIGURED", "APP_ENCRYPTION_KEY is required to persist provider settings"}
	}
	raw, e := json.Marshal(cfg)
	if e != nil {
		return "", e
	}
	nonce := make([]byte, a.providerCipher.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return "", e
	}
	out := a.providerCipher.Seal(nonce, nonce, raw, []byte(providerAAD))
	return base64.StdEncoding.EncodeToString(out), nil
}
func (a *App) openConfig(text string) (provider.Config, error) {
	var cfg provider.Config
	if a.providerCipher == nil {
		return cfg, errors.New("provider settings encryption key unavailable")
	}
	raw, e := base64.StdEncoding.DecodeString(text)
	n := a.providerCipher.NonceSize()
	if e != nil || len(raw) < n {
		return cfg, errors.New("provider settings ciphertext invalid")
	}
	plain, e := a.providerCipher.Open(nil, raw[:n], raw[n:], []byte(providerAAD))
	if e != nil {
		return cfg, errors.New("provider settings decryption failed")
	}
	if e = json.Unmarshal(plain, &cfg); e != nil {
		return cfg, errors.New("provider settings payload invalid")
	}
	return cfg, nil
}
func (a *App) configIn(t *store.Tx) (provider.Config, int, string, error) {
	var row sealedProviders
	e := t.Get("settings", "providers", &row)
	if errors.Is(e, store.ErrNotFound) {
		cfg := provider.ConfigFromEnv()
		if err := provider.ValidateConfig(cfg); err != nil {
			return provider.Config{}, 0, "environment", errors.New("invalid environment provider configuration")
		}
		return cfg, 0, "environment", nil
	}
	if e != nil {
		return provider.Config{}, 0, "", e
	}
	cfg, e := a.openConfig(row.Ciphertext)
	if e == nil {
		e = provider.ValidateConfig(cfg)
	}
	if e != nil {
		return provider.Config{}, row.Revision, "database", errors.New("provider settings invalid or cannot be decrypted")
	}
	return cfg, row.Revision, "database", nil
}
func (a *App) providerConfig(ctx context.Context) (provider.Config, int, string, error) {
	var cfg provider.Config
	var rev int
	var source string
	e := a.Store.View(ctx, func(t *store.Tx) error { var e error; cfg, rev, source, e = a.configIn(t); return e })
	return cfg, rev, source, e
}
func (a *App) cloud(ctx context.Context) (*provider.Client, error) {
	cfg, _, _, e := a.providerConfig(ctx)
	if e != nil {
		return nil, &APIError{503, "PROVIDER_SETTINGS_UNAVAILABLE", "provider settings unavailable; check storage and encryption key"}
	}
	return provider.New(cfg), nil
}
func (a *App) settingsView(cfg provider.Config, rev int, source string) map[string]any {
	caps := provider.New(cfg).Capabilities()
	out := map[string]any{"revision": rev, "source": source, "storage_mode": a.Store.Mode(), "encryption_ready": a.providerCipher != nil, "ocr": map[string]any{"url": cfg.OCRURL, "has_secret": cfg.OCRKey != "", "configured": caps["ocr_configured"]}, "ai": map[string]any{"base_url": cfg.AIBaseURL, "model": cfg.AIModel, "has_secret": cfg.AIKey != "", "configured": caps["ai_configured"]}, "card": map[string]any{"url": cfg.CardURL, "has_secret": cfg.CardKey != "", "configured": caps["card_configured"]}}
	if a.ephemeralProviderKey {
		out["warning"] = "memory_dev settings and encryption key are ephemeral; all settings disappear on restart"
	} else if a.providerCipher == nil {
		out["warning"] = "APP_ENCRYPTION_KEY missing: persisted provider settings cannot be saved"
	}
	return out
}
func settingsUnavailable(w http.ResponseWriter) {
	fail(w, &APIError{503, "PROVIDER_SETTINGS_UNAVAILABLE", "provider settings unavailable; check storage and encryption key"})
}
func (a *App) getProviders(w http.ResponseWriter, r *http.Request) {
	cfg, rev, source, e := a.providerConfig(r.Context())
	if e != nil {
		settingsUnavailable(w)
		return
	}
	respond(w, 200, a.settingsView(cfg, rev, source))
}
func chooseSecret(old, next string, clear bool) (string, error) {
	if len(next) > 8192 {
		return "", bad("provider secret exceeds limit")
	}
	if clear && next != "" {
		return "", bad("cannot supply secret and clear_secret together")
	}
	if clear {
		return "", nil
	}
	if next == "" {
		return old, nil
	}
	return next, nil
}
func (a *App) putProviders(w http.ResponseWriter, r *http.Request) {
	var in providerInput
	if e := decode(r, &in); e != nil {
		fail(w, e)
		return
	}
	var cfg provider.Config
	var rev int
	e := a.Store.With(r.Context(), func(t *store.Tx) error {
		old, current, _, e := a.configIn(t)
		if e != nil {
			return &APIError{503, "PROVIDER_SETTINGS_UNAVAILABLE", "provider settings unavailable; check encryption key"}
		}
		if in.Revision != current {
			return conflict("stale provider settings revision")
		}
		cfg = provider.Config{OCRURL: in.OCR.URL, AIBaseURL: in.AI.BaseURL, AIModel: in.AI.Model, CardURL: in.Card.URL, Timeout: 90 * time.Second}
		if cfg.OCRKey, e = chooseSecret(old.OCRKey, in.OCR.Secret, in.OCR.ClearSecret); e != nil {
			return e
		}
		if cfg.AIKey, e = chooseSecret(old.AIKey, in.AI.Secret, in.AI.ClearSecret); e != nil {
			return e
		}
		if cfg.CardKey, e = chooseSecret(old.CardKey, in.Card.Secret, in.Card.ClearSecret); e != nil {
			return e
		}
		if e = provider.ValidateConfig(cfg); e != nil {
			return bad(e.Error())
		}
		sealed, e := a.sealConfig(cfg)
		if e != nil {
			return e
		}
		rev = current + 1
		if e = t.Put("settings", "providers", "", "", sealedProviders{rev, sealed, time.Now().UTC()}); e != nil {
			return e
		}
		return t.Put("settings_audit", ID(), "providers", "", map[string]any{"revision": rev, "previous_revision": current, "action": "provider_settings_updated", "actor": "teacher", "at": time.Now().UTC(), "changed": map[string]bool{"ocr": old.OCRURL != cfg.OCRURL || old.OCRKey != cfg.OCRKey, "ai": old.AIBaseURL != cfg.AIBaseURL || old.AIModel != cfg.AIModel || old.AIKey != cfg.AIKey, "card": old.CardURL != cfg.CardURL || old.CardKey != cfg.CardKey}})
	})
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, a.settingsView(cfg, rev, "database"))
}
func (a *App) testProvider(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Provider string `json:"provider"`
	}
	if e := decode(r, &in); e != nil {
		fail(w, e)
		return
	}
	if in.Provider != "ocr" && in.Provider != "ai" && in.Provider != "card" {
		fail(w, bad("provider must be ocr, ai, or card"))
		return
	}
	select {
	case a.heavy <- struct{}{}:
		defer func() { <-a.heavy }()
	default:
		fail(w, &APIError{429, "BUSY", "provider connection test busy"})
		return
	}
	cfg, rev, _, e := a.providerConfig(r.Context())
	if e != nil {
		settingsUnavailable(w)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	result, e := provider.New(cfg).TestConnection(ctx, in.Provider)
	if errors.Is(e, provider.ErrNotConfigured) {
		fail(w, &APIError{503, "PROVIDER_NOT_CONFIGURED", "selected provider is not configured"})
		return
	}
	if e != nil {
		respond(w, 200, map[string]any{"provider": in.Provider, "ok": false, "revision": rev, "message": "connection failed or timed out; no configuration changed"})
		return
	}
	respond(w, 200, map[string]any{"provider": in.Provider, "ok": result.OK, "status_code": result.StatusCode, "revision": rev, "message": result.Message})
}
func (a *App) registerProviders(m *http.ServeMux) {
	m.HandleFunc("GET /api/v1/settings/providers", a.getProviders)
	m.HandleFunc("PUT /api/v1/settings/providers", a.putProviders)
	m.HandleFunc("POST /api/v1/settings/providers/test", a.testProvider)
}
