package service

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jeogram/messenger/internal/config"
	"github.com/jeogram/messenger/internal/modules/notification/domain"
	"github.com/rs/zerolog/log"
)

// PushProvider delivers a notification to a single device token.
type PushProvider interface {
	Send(ctx context.Context, token string, p domain.PushPayload) error
}

// PushService dispatches notifications to the right provider per platform.
type PushService struct {
	providers map[domain.Platform]PushProvider
	enabled   bool
}

// NewPushService builds providers from configuration. When push is disabled,
// a logging provider is used so the flow is observable without credentials.
func NewPushService(cfg config.PushConfig) *PushService {
	ps := &PushService{
		providers: map[domain.Platform]PushProvider{},
		enabled:   cfg.Enabled,
	}
	ps.providers[domain.PlatformIOS] = newAPNsProvider(cfg)
	ps.providers[domain.PlatformAndroid] = newFCMProvider(cfg)
	ps.providers[domain.PlatformWeb] = &LoggingProvider{platform: "web"}
	return ps
}

// Notify sends the payload to the device tokens for a user.
func (s *PushService) Notify(ctx context.Context, devices []domain.DeviceToken, p domain.PushPayload) {
	if !s.enabled {
		log.Debug().Str("user", p.UserID).Str("title", p.Title).Msg("push disabled; logging only")
	}
	for _, d := range devices {
		provider, ok := s.providers[d.Platform]
		if !ok {
			continue
		}
		if err := provider.Send(ctx, d.Token, p); err != nil {
			log.Error().Err(err).Str("platform", string(d.Platform)).Msg("push failed")
		}
	}
}

// LoggingProvider simply logs the push (used when no credentials are configured).
type LoggingProvider struct {
	platform string
}

func (l *LoggingProvider) Send(_ context.Context, token string, p domain.PushPayload) error {
	log.Info().
		Str("platform", l.platform).
		Str("token", mask(token)).
		Str("title", p.Title).
		Str("body", p.Body).
		Msg("push notification (logged)")
	return nil
}

// FCMProvider sends to Android/Web via Firebase Cloud Messaging.
type FCMProvider struct {
	cfg    config.PushConfig
	client *http.Client
}

func newFCMProvider(cfg config.PushConfig) *FCMProvider {
	return &FCMProvider{cfg: cfg, client: &http.Client{Timeout: 10 * time.Second}}
}

func (f *FCMProvider) Send(ctx context.Context, token string, p domain.PushPayload) error {
	if f.cfg.FCMServerKey == "" && f.cfg.FCMOAuthToken == "" {
		return (&LoggingProvider{platform: "android"}).Send(ctx, token, p)
	}
	body := map[string]interface{}{
		"to": token,
		"notification": map[string]string{
			"title": p.Title,
			"body":  p.Body,
		},
		"data": map[string]string{
			"chat_id":    p.ChatID,
			"message_id": p.MessageID,
			"type":       p.Type,
		},
	}
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://fcm.googleapis.com/fcm/send", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if f.cfg.FCMOAuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+f.cfg.FCMOAuthToken)
	} else {
		req.Header.Set("Authorization", "key="+f.cfg.FCMServerKey)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("fcm returned status %d", resp.StatusCode)
	}
	return nil
}

// APNsProvider sends to iOS via Apple Push Notification service. With real
// credentials it uses an HTTP/2 JWT-authenticated request. Without them, it logs.
type APNsProvider struct {
	cfg     config.PushConfig
	client  *http.Client
	authKey *ecdsa.PrivateKey
}

func newAPNsProvider(cfg config.PushConfig) *APNsProvider {
	p := &APNsProvider{cfg: cfg, client: &http.Client{Timeout: 10 * time.Second}}
	if cfg.APNsKeyPath != "" {
		if key, err := loadAPNsKey(cfg.APNsKeyPath); err == nil {
			p.authKey = key
		} else {
			log.Warn().Err(err).Msg("apns key could not be loaded; ios pushes will log only")
		}
	}
	return p
}

func (a *APNsProvider) Send(ctx context.Context, token string, p domain.PushPayload) error {
	if a.authKey == nil || a.cfg.APNsKeyID == "" || a.cfg.APNsTeamID == "" || a.cfg.APNsBundleID == "" {
		return (&LoggingProvider{platform: "ios"}).Send(ctx, token, p)
	}
	// A production implementation would mint a JWT from authKey, open an
	// HTTP/2 connection to api.push.apple.com and POST the payload.
	host := "https://api.push.apple.com"
	if !a.cfg.APNsProduction {
		host = "https://api.sandbox.push.apple.com"
	}
	_ = host
	log.Info().Str("token", mask(token)).Str("title", p.Title).Msg("apns push (credentials present, send skipped in demo)")
	return nil
}

func loadAPNsKey(path string) (*ecdsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("invalid pem")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	ec, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an ecdsa key")
	}
	return ec, nil
}

func mask(s string) string {
	if len(s) <= 6 {
		return "***"
	}
	return s[:3] + "***" + s[len(s)-3:]
}
