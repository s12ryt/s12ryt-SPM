package server

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"spm/internal/model"
	"testing"
	"time"
)

type waitingBody struct {
	started chan struct{}
	release chan struct{}
}

func (b *waitingBody) Read(p []byte) (int, error) { close(b.started); <-b.release; return 0, io.EOF }
func (b *waitingBody) Close() error               { return nil }

func TestSlowBodiesDoNotBlockMonitoring(t *testing.T) {
	for _, path := range []string{"/api/nodes", "/api/nodes/node", "/api/settings", "/api/database", "/api/ingest/node"} {
		t.Run(path, func(t *testing.T) {
			s := setup(t)
			cookie := login(t, s)
			s.nodes["node"] = model.Node{ID: "node", TokenHash: hashToken("token")}
			body := &waitingBody{make(chan struct{}), make(chan struct{})}
			method := "PUT"
			if path == "/api/nodes" || path == "/api/ingest/node" {
				method = "POST"
			}
			r := httptest.NewRequest(method, path, body)
			r.AddCookie(cookie)
			r.Header.Set("X-SPM-CSRF", "1")
			r.Header.Set("Authorization", "Bearer token")
			done := make(chan struct{})
			go func() { s.Handler().ServeHTTP(httptest.NewRecorder(), r); close(done) }()
			<-body.started
			read := make(chan struct{})
			go func() { request(s, "GET", "/api/session", nil, nil, ""); close(read) }()
			blocked := false
			select {
			case <-read:
			case <-time.After(250 * time.Millisecond):
				blocked = true
			}
			close(body.release)
			<-done
			<-read
			if blocked {
				t.Fatal("slow request body blocks monitoring reads")
			}
		})
	}
}

func TestNodeListStableOrder(t *testing.T) {
	s := setup(t)
	c := login(t, s)
	for _, name := range []string{"Zulu", "Alpha", "Beta"} {
		request(s, "POST", "/api/nodes", map[string]string{"name": name}, c, "")
	}
	for range 30 {
		w := request(s, "GET", "/api/nodes", nil, c, "")
		var nodes []model.Node
		json.Unmarshal(w.Body.Bytes(), &nodes)
		if nodes[0].Name != "Alpha" || nodes[2].Name != "Zulu" {
			t.Fatal("node list order changes across refreshes")
		}
	}
}

func TestTokenRotationAndLogout(t *testing.T) {
	s := setup(t)
	c := login(t, s)
	w := request(s, "POST", "/api/nodes", map[string]string{"name": "test"}, c, "")
	var v map[string]string
	json.Unmarshal(w.Body.Bytes(), &v)
	id, token := v["id"], v["token"]
	w = request(s, "POST", "/api/nodes/"+id+"/token", nil, c, "")
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	if w = request(s, "POST", "/api/ingest/"+id, model.Snapshot{}, nil, token); w.Code != 401 {
		t.Fatal("old token still valid")
	}
	request(s, "POST", "/api/logout", nil, c, "")
	if w = request(s, "GET", "/api/settings", nil, c, ""); w.Code != 401 {
		t.Fatal("logged-out session still valid")
	}
}
