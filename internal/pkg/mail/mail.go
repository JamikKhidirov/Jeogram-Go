package mail

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/jeogram/messenger/internal/config"
	"github.com/rs/zerolog/log"
)

// Mailer отправляет письма (коды подтверждения, сброс пароля).
// Если SMTP не настроен, код выводится в лог (режим разработки),
// чтобы функционал можно было проверить без реальной почты.
type Mailer struct {
	cfg config.SMTPConfig
}

// New создаёт Mailer. При cfg.Enabled=false письма не уходят, а логируются.
func New(cfg config.SMTPConfig) *Mailer {
	return &Mailer{cfg: cfg}
}

// Send отправляет письмо получателю.
func (m *Mailer) Send(ctx context.Context, to, subject, body string) error {
	if !m.cfg.Enabled {
		log.Info().
			Str("to", to).
			Str("subject", subject).
			Str("body", body).
			Msg("mail (dev mode): письмо не отправлено, вывод в лог")
		return nil
	}
	return m.sendSMTP(to, subject, body)
}

// SendCode отправляет код подтверждения указанного назначения.
func (m *Mailer) SendCode(ctx context.Context, to, code, purpose string) error {
	subject := "Код подтверждения Jeogram"
	body := fmt.Sprintf("Ваш код для «%s»: %s\nКод действителен 10 минут.", purpose, code)
	return m.Send(ctx, to, subject, body)
}

func (m *Mailer) sendSMTP(to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)
	var auth smtp.Auth
	if m.cfg.User != "" {
		auth = smtp.PlainAuth("", m.cfg.User, m.cfg.Password, m.cfg.Host)
	}
	msg := strings.Builder{}
	msg.WriteString("From: " + m.cfg.From + "\r\n")
	msg.WriteString("To: " + to + "\r\n")
	msg.WriteString("Subject: " + subject + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	msg.WriteString(body)
	return smtp.SendMail(addr, auth, m.cfg.From, []string{to}, []byte(msg.String()))
}
