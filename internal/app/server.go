package app

import (
	"context"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/papergrade/papergrade/internal/store"
	"golang.org/x/crypto/bcrypt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Addr, DataDir, Username, Password, PublicOrigin string
	EncryptionKey                                   string
	// SQLitePath is the single database file. Empty means the ephemeral
	// development memory store, which is never used in production.
	SQLitePath string
	Dev        bool
}

// StoreURL is what the store package opens: sqlite:<path>, or empty for the
// ephemeral development memory store. There is no database server.
func (c Config) StoreURL() string {
	if c.SQLitePath != "" {
		return "sqlite:" + c.SQLitePath
	}
	return ""
}

func ConfigFromEnv() Config {
	c := Config{Addr: os.Getenv("SERVER_ADDR"), DataDir: os.Getenv("DATA_DIR"), Username: os.Getenv("ADMIN_USERNAME"), Password: os.Getenv("ADMIN_PASSWORD"), PublicOrigin: os.Getenv("PUBLIC_ORIGIN"), SQLitePath: os.Getenv("SQLITE_PATH"), Dev: os.Getenv("DEV_MODE") == "true", EncryptionKey: os.Getenv("APP_ENCRYPTION_KEY")}
	if c.Addr == "" {
		c.Addr = ":8080"
	}
	if c.DataDir == "" {
		c.DataDir = "./data"
	}
	return c
}

type APIError struct {
	Status        int
	Code, Message string
}

func (e *APIError) Error() string { return e.Message }
func bad(s string) error          { return &APIError{400, "VALIDATION_ERROR", s} }
func conflict(s string) error     { return &APIError{409, "CONFLICT", s} }

type App struct {
	Store                *store.Store
	Config               Config
	account              Account
	loginMu              sync.Mutex
	failures             map[string]attempt
	heavy                chan struct{}
	ocrSlot              chan struct{}
	loginSlot            chan struct{}
	providerCipher       cipher.AEAD
	ephemeralProviderKey bool
}
type attempt struct {
	Count int
	Until time.Time
}

func New(ctx context.Context, c Config) (*App, error) {
	if c.StoreURL() == "" && !c.Dev {
		return nil, fmt.Errorf("SQLITE_PATH required unless DEV_MODE=true (ephemeral memory)")
	}
	if c.Username == "" || len(c.Password) < 12 || len(c.Password) > 72 {
		return nil, fmt.Errorf("ADMIN_USERNAME and ADMIN_PASSWORD (12–72 bytes) required")
	}
	if c.Dev && c.PublicOrigin != "" {
		u, e := url.Parse(c.PublicOrigin)
		if e != nil || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return nil, fmt.Errorf("invalid development PUBLIC_ORIGIN")
		}
		host := u.Hostname()
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return nil, fmt.Errorf("DEV_MODE insecure cookies only allowed on loopback origin")
		}
	}
	if !c.Dev {
		u, e := url.Parse(c.PublicOrigin)
		if e != nil || u.Scheme != "https" || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return nil, fmt.Errorf("PUBLIC_ORIGIN must be an https origin in production")
		}
	}
	providerCipher, ephemeral, e := setupEncryption(c.EncryptionKey, c.Dev, c.StoreURL() == "")
	if e != nil {
		return nil, e
	}
	s, e := store.Open(ctx, c.StoreURL())
	if e != nil {
		return nil, e
	}
	// The SQLite schema is a set of IF NOT EXISTS statements, so creating it on
	// every start is the migration: no separate migrate container is needed.
	if s.Mode() == "sqlite" {
		if e = s.Migrate(ctx); e != nil {
			s.Close()
			return nil, e
		}
	}
	a := &App{Store: s, Config: c, failures: map[string]attempt{}, heavy: make(chan struct{}, 1), ocrSlot: make(chan struct{}, 1), loginSlot: make(chan struct{}, 1), providerCipher: providerCipher, ephemeralProviderKey: ephemeral}
	if e = os.MkdirAll(c.DataDir, 0700); e != nil {
		s.Close()
		return nil, e
	}
	return a, nil
}

// Initialize account after explicit migrations; credential rotation invalidates sessions.
func (a *App) Init(ctx context.Context) error {
	return a.Store.With(ctx, func(t *store.Tx) error {
		var acc Account
		e := t.Get("account", "teacher", &acc)
		if e == nil && acc.Username == a.Config.Username && bcrypt.CompareHashAndPassword(acc.PasswordHash, []byte(a.Config.Password)) == nil {
			a.account = acc
			return nil
		}
		if e != nil && !errors.Is(e, store.ErrNotFound) {
			return e
		}
		hash, e := bcrypt.GenerateFromPassword([]byte(a.Config.Password), bcrypt.DefaultCost)
		if e != nil {
			return e
		}
		acc = Account{a.Config.Username, hash}
		if e = t.Put("account", "teacher", "", "", acc); e != nil {
			return e
		}
		rows, e := t.List("session", "*")
		if e != nil {
			return e
		}
		for _, r := range rows {
			if e = t.Delete("session", r.ID); e != nil {
				return e
			}
		}
		a.account = acc
		return nil
	})
}
func ID() string {
	b := make([]byte, 24)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func digest(s string) string { b := sha256.Sum256([]byte(s)); return hex.EncodeToString(b[:]) }
func decode(r *http.Request, v any) error {
	defer r.Body.Close()
	d := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return bad("invalid JSON: " + e.Error())
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return bad("exactly one JSON object required")
	}
	return nil
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		json.NewEncoder(w).Encode(v)
	}
}
func fail(w http.ResponseWriter, e error) {
	status, code, msg := 500, "INTERNAL_ERROR", "operation failed"
	var ae *APIError
	if errors.As(e, &ae) {
		status, code, msg = ae.Status, ae.Code, ae.Message
	} else if errors.Is(e, store.ErrNotFound) {
		status, code, msg = 404, "NOT_FOUND", "record not found"
	} else if errors.Is(e, store.ErrConflict) {
		status, code, msg = 409, "CONFLICT", "record conflict"
	}
	if status == 500 {
		slog.Error("request failed", "error", e)
	}
	respond(w, status, map[string]string{"code": code, "message": msg})
}
func (a *App) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, map[string]string{"status": "ok"}) })
	m.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if e := a.Store.Ready(r.Context()); e != nil {
			fail(w, &APIError{503, "NOT_READY", "storage unavailable or migrations missing"})
			return
		}
		respond(w, 200, map[string]string{"status": "ready", "storage_mode": a.Store.Mode()})
	})
	m.HandleFunc("POST /api/v1/auth/login", a.login)
	m.HandleFunc("/api/v1/", a.authorized(a.routes()))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" {
			if !a.originOK(r) {
				fail(w, &APIError{403, "CSRF_ORIGIN", "same-origin Origin required"})
				return
			}
		}
		m.ServeHTTP(w, r)
	})
}
func (a *App) originOK(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" || origin == "null" {
		return false
	}
	u, e := url.Parse(origin)
	if e != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return false
	}
	if a.Config.PublicOrigin != "" {
		return origin == a.Config.PublicOrigin
	}
	if !a.Config.Dev || u.Scheme != "http" || u.Host != r.Host {
		return false
	}
	host := u.Hostname()
	return host == "localhost" || net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
}
func (a *App) cookie(w http.ResponseWriter, token string, age int) {
	name := "__Host-papergrade_session"
	if a.Config.Dev {
		name = "papergrade_session"
	}
	http.SetCookie(w, &http.Cookie{Name: name, Value: token, Path: "/", HttpOnly: true, Secure: !a.Config.Dev, SameSite: http.SameSiteStrictMode, MaxAge: age})
}
func (a *App) sessionToken(r *http.Request) string {
	name := "__Host-papergrade_session"
	if a.Config.Dev {
		name = "papergrade_session"
	}
	c, e := r.Cookie(name)
	if e != nil {
		return ""
	}
	return c.Value
}
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	select {
	case a.loginSlot <- struct{}{}:
		defer func() { <-a.loginSlot }()
	default:
		fail(w, &APIError{429, "BUSY", "authentication busy"})
		return
	}
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if e := decode(r, &in); e != nil {
		fail(w, e)
		return
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	a.loginMu.Lock()
	v := a.failures[host]
	if time.Now().After(v.Until) {
		v = attempt{Until: time.Now().Add(10 * time.Minute)}
	}
	v.Count++
	if len(a.failures) > 10000 {
		a.failures = map[string]attempt{}
	}
	a.failures[host] = v
	a.loginMu.Unlock()
	if v.Count > 10 {
		w.Header().Set("Retry-After", "600")
		fail(w, &APIError{429, "RATE_LIMITED", "too many login attempts"})
		return
	}
	valid := bcrypt.CompareHashAndPassword(a.account.PasswordHash, []byte(in.Password)) == nil
	if in.Username != a.account.Username || !valid {
		fail(w, &APIError{401, "INVALID_CREDENTIALS", "invalid username or password"})
		return
	}
	token := ID()
	e := a.Store.With(r.Context(), func(t *store.Tx) error {
		old := a.sessionToken(r)
		if old != "" {
			if e := t.Delete("session", digest(old)); e != nil {
				return e
			}
		}
		if e := t.CleanSessions(time.Now()); e != nil {
			return e
		}
		return t.Put("session", digest(token), "", "", Session{time.Now().Add(12 * time.Hour)})
	})
	if e != nil {
		fail(w, e)
		return
	}
	a.loginMu.Lock()
	delete(a.failures, host)
	a.loginMu.Unlock()
	a.cookie(w, token, 43200)
	respond(w, 200, a.me(r.Context()))
}
func (a *App) authorized(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := a.sessionToken(r)
		if token == "" {
			fail(w, &APIError{401, "UNAUTHENTICATED", "login required"})
			return
		}
		var s Session
		e := a.Store.View(r.Context(), func(t *store.Tx) error { return t.Get("session", digest(token), &s) })
		if e != nil && !errors.Is(e, store.ErrNotFound) {
			fail(w, &APIError{503, "STORAGE_UNAVAILABLE", "session storage unavailable"})
			return
		}
		if e != nil || time.Now().After(s.Expires) {
			fail(w, &APIError{401, "UNAUTHENTICATED", "session expired"})
			return
		}
		next.ServeHTTP(w, r)
	}
}
func (a *App) me(ctx context.Context) map[string]any {
	return map[string]any{"user": map[string]string{"id": "teacher", "username": a.account.Username}, "storage_mode": a.Store.Mode(), "capabilities": a.capabilities(ctx)}
}
func (a *App) routes() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /api/v1/me", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, a.me(r.Context())) })
	m.HandleFunc("POST /api/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		e := a.Store.With(r.Context(), func(t *store.Tx) error { return t.Delete("session", digest(a.sessionToken(r))) })
		if e != nil {
			fail(w, e)
			return
		}
		a.cookie(w, "", -1)
		respond(w, 200, map[string]bool{"ok": true})
	})
	m.HandleFunc("GET /api/v1/settings/capabilities", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, a.capabilities(r.Context())) })
	a.registerProviders(m)
	a.registerClasses(m)
	a.registerExams(m)
	a.registerCaptures(m)
	a.registerDocuments(m)
	return m
}
func bounded(s string, n int) bool { return strings.TrimSpace(s) != "" && len(s) <= n }
