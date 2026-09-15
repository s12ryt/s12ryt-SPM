package web

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmbeddedDashboard(t *testing.T) {
	w := httptest.NewRecorder()
	Handler().ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "<div id=\"app\">") {
		t.Fatalf("dashboard unavailable: %d", w.Code)
	}
	w = httptest.NewRecorder()
	Handler().ServeHTTP(w, httptest.NewRequest("GET", "/missing.js", nil))
	if w.Code != 404 {
		t.Fatal("missing asset must be 404")
	}
}

func TestFavicon(t *testing.T) {
	w := httptest.NewRecorder()
	Handler().ServeHTTP(w, httptest.NewRequest("GET", "/favicon.svg", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "<svg") {
		t.Fatalf("favicon unavailable: %d", w.Code)
	}
}
