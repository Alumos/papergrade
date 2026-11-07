package app

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"github.com/papergrade/papergrade/internal/store"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
)

// Each integration test has its own schema; never truncates an existing installation.
func TestPostgresProvidersHotReloadRestartAndWrongKey(t *testing.T) {
	database := os.Getenv("TEST_DATABASE_URL")
	if database == "" {
		t.Skip("TEST_DATABASE_URL absent: provider PostgreSQL integration not executed")
	}
	ctx := context.Background()
	admin, e := store.Open(ctx, database)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	schema := "settings_" + ID()
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, e = admin.Pool.Exec(ctx, "CREATE SCHEMA "+identifier); e != nil {
		t.Fatal(e)
	}
	defer admin.Pool.Exec(ctx, "DROP SCHEMA "+identifier+" CASCADE")
	u, e := url.Parse(database)
	if e != nil {
		t.Fatal(e)
	}
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	config := Config{Dev: true, DatabaseURL: u.String(), Username: "teacher", Password: "postgres-integration-password", DataDir: t.TempDir(), EncryptionKey: strings.Repeat("integration-test-key", 2)}
	newApp := func(c Config) *App {
		t.Helper()
		a, e := New(ctx, c)
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(a.Store.Close)
		return a
	}
	api := newApp(config)
	if e = api.Store.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	if e = api.Init(ctx); e != nil {
		t.Fatal(e)
	}
	worker := newApp(config)
	if e = worker.Init(ctx); e != nil {
		t.Fatal(e)
	}
	login := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "http://localhost/api/v1/auth/login", bytes.NewReader(mustJSON(map[string]string{"username": config.Username, "password": config.Password})))
	r.Header.Set("Origin", "http://localhost")
	api.Handler().ServeHTTP(login, r)
	if login.Code != 200 {
		t.Fatal("login failed", login.Code)
	}
	h := &harness{t: t, a: api, h: api.Handler(), cookie: login.Result().Cookies()[0]}
	var tokensMu sync.Mutex
	tokens := []string{}
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokensMu.Lock()
		tokens = append(tokens, r.Header.Get("Authorization"))
		tokensMu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{"text": "cloud extraction", "pages": 1})
	}))
	defer gateway.Close()
	h.request("PUT", "/api/v1/settings/providers", providerBody(0, gateway.URL, "first-private-token"), 200)
	callWorker := func(a *App) {
		t.Helper()
		client, e := a.cloud(ctx)
		if e != nil {
			t.Fatal(e)
		}
		out, e := client.Extract(ctx, "paper.pdf", []byte("%PDF-1.4\n%%EOF"))
		if e != nil || out.Text != "cloud extraction" {
			t.Fatal("runtime Extract did not use persisted settings", e)
		}
	}
	callWorker(worker)
	h.request("PUT", "/api/v1/settings/providers", providerBody(1, gateway.URL, "second-private-token"), 200)
	callWorker(worker)
	// Blank key on an independent API instance preserves the latest stored token.
	h2 := &harness{t: t, a: worker, h: worker.Handler(), cookie: h.cookie}
	h2.request("PUT", "/api/v1/settings/providers", providerBody(2, gateway.URL, ""), 200)
	callWorker(api)
	restarted := newApp(config)
	if e = restarted.Init(ctx); e != nil {
		t.Fatal(e)
	}
	callWorker(restarted)
	tokensMu.Lock()
	got := append([]string{}, tokens...)
	tokensMu.Unlock()
	want := []string{"Bearer first-private-token", "Bearer second-private-token", "Bearer second-private-token", "Bearer second-private-token"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatal("runtime hot update/retain/restart failed")
	}
	var stored sealedProviders
	if e = api.Store.View(ctx, func(tx *store.Tx) error { return tx.Get("settings", "providers", &stored) }); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(mustJSON(stored)), "private-token") {
		t.Fatal("database setting not encrypted")
	}
	// Separate pools racing the same revision must have exactly one winner.
	start := make(chan struct{})
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for _, app := range []*App{api, worker} {
		app := app
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			r := httptest.NewRequest("PUT", "http://localhost/api/v1/settings/providers", bytes.NewReader(mustJSON(providerBody(3, gateway.URL, ""))))
			r.Header.Set("Origin", "http://localhost")
			r.AddCookie(h.cookie)
			w := httptest.NewRecorder()
			app.Handler().ServeHTTP(w, r)
			codes <- w.Code
		}()
	}
	close(start)
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for code := range codes {
		counts[code]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatal("cross-pool revision race", counts)
	}
	wrongConfig := config
	wrongConfig.EncryptionKey = strings.Repeat("wrong-key", 8)
	wrong := newApp(wrongConfig)
	if e = wrong.Init(ctx); e != nil {
		t.Fatal(e)
	}
	badHarness := &harness{t: t, a: wrong, h: wrong.Handler(), cookie: h.cookie}
	failure := badHarness.request("GET", "/api/v1/settings/providers", nil, 503)
	if strings.Contains(failure.Body.String(), "private-token") {
		t.Fatal("wrong-key response leaked secret")
	}
	if _, e = wrong.cloud(ctx); e == nil {
		t.Fatal("wrong key fell back to environment")
	}
}
