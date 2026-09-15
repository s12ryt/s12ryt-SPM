package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"spm/internal/model"
	"strings"
	"sync"
	"testing"
	"time"
)

type waitingWriter struct {
	*httptest.ResponseRecorder
	started, release chan struct{}
}

func (w *waitingWriter) Write(p []byte) (int, error) {
	close(w.started)
	<-w.release
	return w.ResponseRecorder.Write(p)
}

func TestSlowResponseDoesNotBlockIngest(t *testing.T) {
	for _, route := range []struct{ method, path, body string }{
		{"GET", "/api/nodes", ""},
		{"GET", "/api/settings", ""},
		{"PUT", "/api/nodes/node", `{"name":"host"}`},
	} {
		t.Run(route.method+route.path, func(t *testing.T) {
			s := setup(t)
			c := login(t, s)
			s.nodes["node"] = model.Node{ID: "node", TokenHash: hashToken("token")}
			w := &waitingWriter{httptest.NewRecorder(), make(chan struct{}), make(chan struct{})}
			r := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
			r.Header.Set("X-SPM-CSRF", "1")
			r.AddCookie(c)
			done := make(chan struct{})
			go func() { s.Handler().ServeHTTP(w, r); close(done) }()
			<-w.started
			ingested := make(chan int, 1)
			go func() {
				ingested <- request(s, "POST", "/api/ingest/node", model.Snapshot{Time: 1000, Hostname: "host", Cores: 1, MemoryTotal: 100}, nil, "token").Code
			}()
			blocked := false
			select {
			case status := <-ingested:
				if status != 200 {
					t.Errorf("ingest: %d", status)
				}
			case <-time.After(250 * time.Millisecond):
				blocked = true
			}
			close(w.release)
			<-done
			if blocked {
				<-ingested
				t.Fatal("slow dashboard reader blocks Agent uploads")
			}
		})
	}
}

func TestPasswordVerificationDoesNotBlockMonitoring(t *testing.T) {
	s := setup(t)
	started, release := make(chan struct{}), make(chan struct{})
	// Keep the real bcrypt verifier; this gate models a busy CPU without a timing benchmark.
	verify := s.verifyPassword
	var once sync.Once
	s.verifyPassword = func(hash, password []byte) error {
		once.Do(func() { close(started) })
		<-release
		return verify(hash, password)
	}
	done := make(chan struct{})
	go func() {
		request(s, "POST", "/api/login", map[string]string{"username": "admin", "password": "test-password-123"}, nil, "")
		close(done)
	}()
	<-started
	read := make(chan struct{})
	go func() { request(s, "GET", "/api/session", nil, nil, ""); close(read) }()
	second := make(chan int, 1)
	secondDone := make(chan struct{})
	go func() {
		second <- request(s, "POST", "/api/login", map[string]string{"username": "admin", "password": "test-password-123"}, nil, "").Code
		close(secondDone)
	}()
	blocked := false
	select {
	case <-read:
	case <-time.After(250 * time.Millisecond):
		blocked = true
	}
	select {
	case status := <-second:
		if status != 429 {
			t.Errorf("concurrent expensive login must be bounded: %d", status)
		}
	case <-time.After(250 * time.Millisecond):
		blocked = true
	}
	close(release)
	<-done
	<-read
	<-secondDone
	if blocked {
		t.Fatal("password verification holds the global monitoring lock")
	}
}

func TestPrivateRoutesAndExpiredSessions(t *testing.T) {
	s := setup(t)
	c := login(t, s)
	for _, cookie := range []bool{false, true} {
		if cookie {
			s.Now = func() time.Time { return time.UnixMilli(100000).Add(25 * time.Hour) }
		}
		for _, route := range []struct{ method, path, body string }{
			{"GET", "/api/settings", ""}, {"GET", "/api/nodes", ""}, {"GET", "/api/alerts", ""},
			{"GET", "/api/nodes/node/history?from=0&to=1&limit=1", ""},
			{"POST", "/api/nodes", `{ "name":"blocked" }`},
			{"PUT", "/api/settings", `{}`}, {"PUT", "/api/database", `{}`},
			{"PUT", "/api/nodes/node", `{}`}, {"DELETE", "/api/nodes/node", ""}, {"POST", "/api/nodes/node/token", ""},
		} {
			r := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
			r.Header.Set("X-SPM-CSRF", "1")
			if cookie {
				r.AddCookie(c)
			}
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, r)
			if w.Code != 401 {
				t.Errorf("expired=%v %s %s: %d", cookie, route.method, route.path, w.Code)
			}
		}
	}
}

func TestNotificationRedirectDoesNotForwardSecrets(t *testing.T) {
	requests := 0
	client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{StatusCode: 307, Header: http.Header{"Location": []string{"https://other.test/"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})}
	err := Send(context.Background(), client, model.Notifications{WebhookEnabled: true, WebhookURL: "https://hook.test/secret"}, "webhook", `{"message":"private host info"}`)
	if err == nil || requests != 1 || strings.Contains(err.Error(), "secret") {
		t.Fatal("notification followed redirect or exposed URL", err)
	}
}

func TestHTTPSProxyLoginUsesSecureCookie(t *testing.T) {
	s := setup(t)
	r := httptest.NewRequest("POST", "http://monitor.example/api/login", strings.NewReader(`{"username":"admin","password":"test-password-123"}`))
	r.Header.Set("Origin", "https://monitor.example")
	r.Header.Set("X-SPM-CSRF", "1")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly {
		t.Fatal("HTTPS browser behind reverse proxy received an insecure session cookie")
	}
}

func TestIngestAfterUnobservedGapResetsHold(t *testing.T) {
	s := setup(t)
	c := login(t, s)
	w := request(s, "POST", "/api/nodes", map[string]string{"name": "gap"}, c, "")
	var enrolled struct{ ID, Token string }
	if err := json.Unmarshal(w.Body.Bytes(), &enrolled); err != nil {
		t.Fatal(err)
	}
	snap := model.Snapshot{Time: 1000, Hostname: "host", Cores: 1, MemoryTotal: 100, MemoryUsed: 95}
	path := "/api/ingest/" + enrolled.ID
	if w = request(s, "POST", path, snap, nil, enrolled.Token); w.Code != 200 {
		t.Fatal(w.Body)
	}
	// Simulate a paused/restarted server: no Tick ran during this gap.
	s.Now = func() time.Time { return time.UnixMilli(140000) }
	snap.Time += 40000
	if w = request(s, "POST", path, snap, nil, enrolled.Token); w.Code != 200 {
		t.Fatal(w.Body)
	}
	events, err := s.DB.Alerts(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range events {
		if e.Kind == "memory" && e.Active {
			t.Fatal("unobserved offline gap counted toward memory hold")
		}
	}
}
