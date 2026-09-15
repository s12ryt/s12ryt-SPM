package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"spm/internal/model"
	"strings"
	"testing"
)

func TestWebhookPayloadAndFailure(t *testing.T) {
	called := false
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		b, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || !strings.Contains(string(b), `"nodeId":"a"`) {
			t.Error("wrong webhook request")
		}
		w.WriteHeader(503)
	}))
	defer remote.Close()
	err := Send(context.Background(), remote.Client(), model.Notifications{WebhookEnabled: true, WebhookURL: remote.URL}, "webhook", `{"nodeId":"a","message":"offline"}`)
	if err == nil || !called {
		t.Fatal("webhook failure ignored")
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestTelegramAndDisabledChannels(t *testing.T) {
	called := 0
	client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		called++
		b, _ := io.ReadAll(r.Body)
		if r.URL.Host != "api.telegram.org" || r.URL.Path != "/bot123:secret/sendMessage" || !strings.Contains(string(b), `"chat_id":"-42"`) || !strings.Contains(string(b), "CPU triggered") {
			t.Error("wrong Telegram payload", string(b))
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true}`)), Header: make(http.Header)}, nil
	})}
	config := model.Notifications{TelegramEnabled: true, TelegramToken: "123:secret", TelegramChat: "-42"}
	if err := Send(context.Background(), client, config, "telegram", `{"message":"CPU triggered"}`); err != nil || called != 1 {
		t.Fatal("Telegram not sent", err)
	}
	config.TelegramEnabled = false
	if err := Send(context.Background(), client, config, "telegram", `{}`); err != nil || called != 1 {
		t.Fatal("disabled channel sent")
	}
}
