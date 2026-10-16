// Package resend sends email through the Resend API (https://resend.com).
package resend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/mail/app"
)

type Sender struct {
	apiKey string
	url    string
	http   *http.Client
}

var _ app.Sender = (*Sender)(nil)

func New(apiKey string) *Sender {
	return &Sender{apiKey: apiKey, url: "https://api.resend.com/emails", http: &http.Client{Timeout: 15 * time.Second}}
}

type tag struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (s *Sender) Send(ctx context.Context, m app.Outgoing) (string, error) {
	body := map[string]any{"from": m.From, "to": []string{m.To}, "subject": m.Subject, "html": m.HTML, "text": m.Text}
	if m.ReplyTo != "" {
		body["reply_to"] = m.ReplyTo
	}
	var tags []tag
	for k, v := range m.Tags {
		tags = append(tags, tag{k, v})
	}
	body["tags"] = tags
	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	// Resend dedupes retries of the same message for 24h.
	req.Header.Set("Idempotency-Key", m.IdempotencyKey)
	resp, err := s.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("resend: %w", err)
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var out struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(rb, &out)
		return out.ID, nil
	}
	err = fmt.Errorf("resend: status %d: %s", resp.StatusCode, rb)
	// 4xx other than rate limiting won't succeed on retry (bad address, unverified domain, ...).
	if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
		return "", &app.PermanentError{Err: err}
	}
	return "", err
}
