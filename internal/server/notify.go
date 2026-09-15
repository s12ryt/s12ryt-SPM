package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"spm/internal/model"
	"time"
)

func Send(ctx context.Context, client *http.Client, config model.Notifications, channel, payload string) error {
	endpoint := config.WebhookURL
	body := []byte(payload)
	switch channel {
	case "webhook":
		if !config.WebhookEnabled {
			return nil
		}
	case "telegram":
		if !config.TelegramEnabled {
			return nil
		}
		var alert model.Alert
		if err := json.Unmarshal(body, &alert); err != nil {
			return err
		}
		endpoint = "https://api.telegram.org/bot" + config.TelegramToken + "/sendMessage"
		body, _ = json.Marshal(map[string]string{"chat_id": config.TelegramChat, "text": "SPM · " + alert.Message})
	default:
		return errors.New("unknown notification channel")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return errors.New("invalid notification URL")
	}
	req.Header.Set("Content-Type", "application/json")
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := copyClient.Do(req)
	if err != nil {
		return errors.New("notification connection failed")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("notification HTTP %d", res.StatusCode)
	}
	if channel == "telegram" {
		var result struct {
			OK bool `json:"ok"`
		}
		if err = json.NewDecoder(io.LimitReader(res.Body, 65536)).Decode(&result); err != nil || !result.OK {
			return errors.New("Telegram rejected notification")
		}
	} else {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 65536))
	}
	return nil
}
func (s *Server) Notify(ctx context.Context, client *http.Client) error {
	s.mu.Lock()
	config := s.settings.Notifications
	now := s.Now().UnixMilli()
	s.mu.Unlock()
	due, err := s.DB.Due(ctx, now)
	if err != nil {
		return err
	}
	var result error
	for _, d := range due {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		sendErr := Send(ctx, client, config, d.Channel, d.Payload)
		if err = s.DB.Delivered(ctx, d, now, sendErr == nil); err != nil {
			return err
		}
		if sendErr != nil {
			result = sendErr
		}
	}
	return result
}
func (s *Server) Prune(ctx context.Context) error {
	s.mu.Lock()
	before := s.Now().Add(-time.Duration(s.settings.RetentionDays) * 24 * time.Hour).UnixMilli()
	s.mu.Unlock()
	return s.DB.Prune(ctx, before)
}
