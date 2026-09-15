package collector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"spm/internal/model"
	"testing"
	"time"
)

func TestCollectMachine(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	s, err := Collect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Validate(); err != nil {
		t.Fatalf("invalid real sample: %v", err)
	}
	if len(s.Disks) == 0 || s.CPUTotal <= 0 {
		t.Fatal("missing disk or CPU counters")
	}
}
func TestUpload(t *testing.T) {
	called := false
	reply := `{"intervalSeconds":7}`
	status := 200
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != "POST" || r.URL.Path != "/api/ingest/node-1" || r.Header.Get("Authorization") != "Bearer private-token" {
			t.Error("invalid request")
		}
		var s model.Snapshot
		if json.NewDecoder(r.Body).Decode(&s) != nil || s.Hostname != "test" {
			t.Error("missing sample")
		}
		w.WriteHeader(status)
		w.Write([]byte(reply))
	}))
	defer srv.Close()
	send := func() (time.Duration, error) {
		return Upload(context.Background(), srv.Client(), srv.URL, "node-1", "private-token", model.Snapshot{Hostname: "test"})
	}
	interval, err := send()
	if err != nil || interval != 7*time.Second || !called {
		t.Fatalf("upload: %v %v called=%v", interval, err, called)
	}
	reply = `{"intervalSeconds":0}`
	if _, err = send(); err == nil {
		t.Error("accepted zero interval")
	}
	status = 401
	if _, err = send(); err == nil {
		t.Error("accepted unauthorized")
	}
	if _, err = Upload(context.Background(), srv.Client(), "file:///bad", "x", "x", model.Snapshot{}); err == nil {
		t.Error("accepted non HTTP URL")
	}
}
