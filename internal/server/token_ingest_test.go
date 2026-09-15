package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"spm/internal/model"
	"testing"
	"time"
)

func tokenUpload(s *Server, token string, sample model.Snapshot) *httptest.ResponseRecorder {
	data, _ := json.Marshal(sample)
	r := httptest.NewRequest("POST", "/api/ingest", bytes.NewReader(data))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}

func TestTokenOnlyIngestLifecycle(t *testing.T) {
	s := setup(t)
	cookie := login(t, s)
	credentials := make([]struct{ ID, Token string }, 2)
	for i := range credentials {
		w := request(s, "POST", "/api/nodes", map[string]string{"name": "same-name"}, cookie, "")
		if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &credentials[i]) != nil {
			t.Fatalf("enroll: %d %s", w.Code, w.Body)
		}
	}
	sample := model.Snapshot{Time: 1000, Hostname: "same-host", Cores: 1, MemoryTotal: 100, MemoryUsed: 25}
	for _, token := range []string{"", "unknown-token"} {
		if w := tokenUpload(s, token, sample); w.Code != 401 {
			t.Fatalf("unauthorized upload: %d %s", w.Code, w.Body)
		}
	}
	for i, c := range credentials {
		sample.MemoryUsed = uint64(25 + i*25)
		if w := tokenUpload(s, c.Token, sample); w.Code != 200 {
			t.Fatalf("token-only upload: %d %s", w.Code, w.Body)
		}
		points, err := s.DB.History(context.Background(), c.ID, 0, 200000, 10)
		if err != nil || len(points) != 1 || points[0].Memory != float64(sample.MemoryUsed) {
			t.Fatalf("history not isolated: %+v, %v", points, err)
		}
	}
	if w := request(s, "POST", "/api/ingest/"+credentials[0].ID, sample, nil, credentials[1].Token); w.Code != 401 {
		t.Fatal("legacy path accepted another host's token")
	}
	restarted, err := New(context.Background(), s.DB, s.Runtime, "admin", "test-password-123")
	if err != nil {
		t.Fatal(err)
	}
	restarted.Now = func() time.Time { return time.UnixMilli(103000) }
	sample.Time = 4000
	if w := tokenUpload(restarted, credentials[0].Token, sample); w.Code != 200 {
		t.Fatalf("token mapping lost after restart: %d %s", w.Code, w.Body)
	}
	cookie = login(t, restarted)
	w := request(restarted, "POST", "/api/nodes/"+credentials[0].ID+"/token", nil, cookie, "")
	var rotated struct{ ID, Token string }
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &rotated) != nil || rotated.ID != credentials[0].ID {
		t.Fatalf("rotate: %d %s", w.Code, w.Body)
	}
	sample.Time = 7000
	restarted.Now = func() time.Time { return time.UnixMilli(106000) }
	if w := tokenUpload(restarted, credentials[0].Token, sample); w.Code != 401 {
		t.Fatal("rotated token remains valid")
	}
	if w := tokenUpload(restarted, rotated.Token, sample); w.Code != 200 {
		t.Fatalf("new token not bound: %d %s", w.Code, w.Body)
	}
	if w := request(restarted, "DELETE", "/api/nodes/"+rotated.ID, nil, cookie, ""); w.Code != 200 {
		t.Fatal("delete failed")
	}
	sample.Time = 10000
	if w := tokenUpload(restarted, rotated.Token, sample); w.Code != 401 {
		t.Fatal("deleted host still accepts reports")
	}
	if w := tokenUpload(restarted, credentials[1].Token, sample); w.Code != 200 {
		t.Fatalf("other host affected by delete: %d %s", w.Code, w.Body)
	}
}
