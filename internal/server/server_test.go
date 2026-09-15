package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"spm/internal/model"
	"spm/internal/store"
	"strings"
	"testing"
	"time"
)

func setup(t *testing.T) *Server {
	t.Helper()
	db, r, e := store.Load(context.Background(), t.TempDir(), "")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	s, e := New(context.Background(), db, r, "admin", "test-password-123")
	if e != nil {
		t.Fatal(e)
	}
	s.Now = func() time.Time { return time.UnixMilli(100000) }
	return s
}
func request(s *Server, method, path string, body any, cookie *http.Cookie, token string) *httptest.ResponseRecorder {
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-SPM-CSRF", "1")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}
func login(t *testing.T, s *Server) *http.Cookie {
	t.Helper()
	w := request(s, "POST", "/api/login", map[string]string{"username": "admin", "password": "test-password-123"}, nil, "")
	if w.Code != 200 {
		t.Fatalf("login %d %s", w.Code, w.Body)
	}
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("missing session cookie")
	}
	return cookies[0]
}
func TestPrivatePublicAndCSRF(t *testing.T) {
	s := setup(t)
	if w := request(s, "GET", "/api/nodes", nil, nil, ""); w.Code != 401 {
		t.Fatalf("private read = %d", w.Code)
	}
	cookie := login(t, s)
	settings := model.DefaultSettings()
	settings.Public = true
	settings.Notifications.TelegramToken = "private-secret"
	if w := request(s, "PUT", "/api/settings", settings, cookie, ""); w.Code != 200 {
		t.Fatalf("settings %d %s", w.Code, w.Body)
	}
	if w := request(s, "GET", "/api/nodes", nil, nil, ""); w.Code != 200 || strings.Contains(w.Body.String(), "private-secret") {
		t.Fatal("public read failed or leaked secret")
	}
	if w := request(s, "GET", "/api/settings", nil, nil, ""); w.Code != 401 {
		t.Fatal("public settings exposed")
	}
	r := httptest.NewRequest("PUT", "/api/settings", strings.NewReader(`{}`))
	r.AddCookie(cookie)
	r.Header.Set("Origin", "https://evil.test")
	r.Header.Set("X-SPM-CSRF", "1")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross-origin write accepted")
	}
}
func TestEnrollIngestIntervalHistoryOfflineAndTokenIsolation(t *testing.T) {
	s := setup(t)
	cookie := login(t, s)
	w := request(s, "POST", "/api/nodes", map[string]string{"name": "Tokyo"}, cookie, "")
	if w.Code != 201 {
		t.Fatalf("enroll: %d %s", w.Code, w.Body)
	}
	var enrolled struct{ ID, Token string }
	json.Unmarshal(w.Body.Bytes(), &enrolled)
	if enrolled.Token == "" {
		t.Fatal("missing enrollment token")
	}
	snap := model.Snapshot{Time: 1000, Hostname: "host", OS: "linux", Cores: 2, MemoryTotal: 100, MemoryUsed: 50, CPUTotal: 100, CPUIdle: 50}
	path := "/api/ingest/" + enrolled.ID
	if w = request(s, "POST", path, snap, nil, "bad"); w.Code != 401 {
		t.Fatal("invalid token accepted")
	}
	rules := model.DefaultSettings().Rules
	rules.IntervalSeconds = 10
	if w = request(s, "PUT", "/api/nodes/"+enrolled.ID, map[string]any{"name": "Tokyo", "rules": rules}, cookie, ""); w.Code != 200 {
		t.Fatal(w.Body)
	}
	w = request(s, "POST", path, snap, nil, enrolled.Token)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"intervalSeconds":10`) {
		t.Fatalf("ingest interval: %d %s", w.Code, w.Body)
	}
	if w = request(s, "POST", path, snap, nil, enrolled.Token); w.Code != 409 {
		t.Fatalf("out of order not rejected %d", w.Code)
	}
	w = request(s, "GET", "/api/nodes", nil, cookie, "")
	if strings.Contains(w.Body.String(), enrolled.Token) || strings.Contains(w.Body.String(), "tokenHash") {
		t.Fatal("token exposed")
	}
	w = request(s, "GET", "/api/nodes/"+enrolled.ID+"/history?from=0&to=200000&limit=10", nil, cookie, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"memory":50`) {
		t.Fatal("missing history", w.Body)
	}
	s.Now = func() time.Time { return time.UnixMilli(134000) }
	if err := s.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	events, _ := s.DB.Alerts(context.Background(), 10)
	if len(events) != 1 || events[0].Kind != "offline" {
		t.Fatalf("offline dedup failed %+v", events)
	}
	snap.Time = 2000
	if w = request(s, "POST", path, snap, nil, enrolled.Token); w.Code != 200 {
		t.Fatal(w.Body)
	}
	events, _ = s.DB.Alerts(context.Background(), 10)
	if len(events) != 2 || events[0].Active {
		t.Fatalf("online recovery failed %+v", events)
	}
}
func TestMalformedLoginAndSettings(t *testing.T) {
	s := setup(t)
	if w := request(s, "POST", "/api/login", map[string]string{"username": "admin", "password": "wrong"}, nil, ""); w.Code != 401 {
		t.Fatalf("bad login: %d", w.Code)
	}
	c := login(t, s)
	bad := model.DefaultSettings()
	bad.Rules.IntervalSeconds = 0
	if w := request(s, "PUT", "/api/settings", bad, c, ""); w.Code != 400 {
		t.Fatal("bad rules accepted")
	}
	if w := request(s, "PUT", "/api/database", map[string]string{"url": "host=localhost"}, c, ""); w.Code != 400 {
		t.Fatal("non URL accepted")
	}
}
