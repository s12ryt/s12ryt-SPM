package collector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRunUploadsAndStops(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	called := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"intervalSeconds":3600}`))
		called <- struct{}{}
		cancel()
	}))
	defer srv.Close()
	Run(ctx, srv.Client(), srv.URL, "node", "secret", func(err error) {})
	select {
	case <-called:
	default:
		t.Fatal("agent never uploaded")
	}
}
