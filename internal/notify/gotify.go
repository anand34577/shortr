package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type GotifyConfig struct {
	Enabled bool
	URL     string // e.g. https://gotify.example.com
	Token   string // application token
}

type GotifySender struct {
	cfg    GotifyConfig
	client *http.Client
}

func NewGotifySender(cfg GotifyConfig) *GotifySender {
	return &GotifySender{cfg: cfg, client: &http.Client{Timeout: 5 * time.Second}}
}

func (g *GotifySender) Enabled() bool { return g.cfg.Enabled }

type gotifyPriority int

const (
	PriorityLow    gotifyPriority = 2
	PriorityNormal gotifyPriority = 5
	PriorityHigh   gotifyPriority = 8
)

// Send pushes a message to the user's self-hosted Gotify server. Gotify is
// opt-in and self-hosted by the admin — no data goes to any third party
// unless the admin explicitly points SHORTR_GOTIFY_URL at one.
func (g *GotifySender) Send(ctx context.Context, title, message string, priority gotifyPriority) error {
	if !g.cfg.Enabled {
		return nil
	}
	body, _ := json.Marshal(map[string]any{
		"title": title, "message": message, "priority": int(priority),
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.cfg.URL+"/message?token="+g.cfg.Token, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("gotify: request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("gotify: unexpected status %d", resp.StatusCode)
	}
	return nil
}
