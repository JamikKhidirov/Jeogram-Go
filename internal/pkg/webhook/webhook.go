package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// Dispatcher delivers outbound event notifications to configured subscriber URLs.
// It is intentionally transport-agnostic: each event is a JSON POST request.
type Dispatcher struct {
	urls    []string
	timeout time.Duration
	client  *http.Client
}

// New creates a Dispatcher for the given subscriber URLs.
func New(urls []string, timeout time.Duration) *Dispatcher {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Dispatcher{
		urls:    urls,
		timeout: timeout,
		client:  &http.Client{Timeout: timeout},
	}
}

// Enabled reports whether any subscribers are configured.
func (d *Dispatcher) Enabled() bool {
	return d != nil && len(d.urls) > 0
}

// Event is the envelope delivered to subscribers.
type Event struct {
	Type      string      `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Payload   interface{} `json:"payload"`
}

// Send delivers an event to all subscribers asynchronously. Failures are logged
// but never block the caller (webhooks must not affect the main request path).
func (d *Dispatcher) Send(eventType string, payload interface{}) {
	if !d.Enabled() {
		return
	}
	body, err := json.Marshal(Event{Type: eventType, Timestamp: time.Now(), Payload: payload})
	if err != nil {
		return
	}
	for _, url := range d.urls {
		go d.post(url, body)
	}
}

func (d *Dispatcher) post(url string, body []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Warn().Err(err).Str("url", url).Msg("webhook: bad request")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Jeogram-Event", "1")
	resp, err := d.client.Do(req)
	if err != nil {
		log.Warn().Err(err).Str("url", url).Msg("webhook: delivery failed")
		return
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Warn().Int("status", resp.StatusCode).Str("url", url).Msg("webhook: non-2xx response")
	}
}
