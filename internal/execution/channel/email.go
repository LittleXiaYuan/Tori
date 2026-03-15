package channel

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

// EmailConfig holds SMTP configuration.
type EmailConfig struct {
	Host     string // SMTP server host
	Port     int    // SMTP server port (587 for TLS, 465 for SSL)
	Username string // SMTP username
	Password string // SMTP password
	From     string // sender email address
	UseTLS   bool   // use STARTTLS
}

// Email implements the Channel interface for email-based communication.
// Inbound: polls an IMAP/webhook source (simplified: accepts messages via Push method).
// Outbound: sends via SMTP.
type Email struct {
	cfg    EmailConfig
	msgCh  chan Message
	mu     sync.Mutex
	dialer func() (*smtp.Client, error)
}

// NewEmail creates an Email channel with the given SMTP config.
func NewEmail(cfg EmailConfig) *Email {
	if cfg.Port == 0 {
		cfg.Port = 587
	}
	if cfg.From == "" {
		cfg.From = cfg.Username
	}
	e := &Email{
		cfg:   cfg,
		msgCh: make(chan Message, 100),
	}
	e.dialer = e.defaultDialer
	return e
}

func (e *Email) Type() string { return "email" }

// Push injects an inbound email message for processing.
// Call this from a webhook handler or IMAP poller.
func (e *Email) Push(msg Message) {
	msg.ChannelType = "email"
	e.msgCh <- msg
}

func (e *Email) Start(ctx context.Context, handler func(Message) Reply) error {
	slog.Info("email channel started", "from", e.cfg.From)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-e.msgCh:
			go func(m Message) {
				reply := handler(m)
				if err := e.Send(ctx, m.Extra["reply_to"], reply); err != nil {
					slog.Error("email send failed", "err", err, "to", m.Extra["reply_to"])
				}
			}(msg)
		}
	}
}

func (e *Email) Send(_ context.Context, target string, reply Reply) error {
	if target == "" {
		return fmt.Errorf("email target (recipient) is required")
	}
	if reply.Content == "" {
		return nil
	}

	subject := "Reply from Yunque Agent"
	if s, ok := extractSubject(reply.Content); ok {
		subject = s
	}

	body := buildEmailBody(e.cfg.From, target, subject, reply.Content)

	client, err := e.dialer()
	if err != nil {
		return fmt.Errorf("email dial: %w", err)
	}
	defer client.Close()

	if err := client.Auth(smtp.PlainAuth("", e.cfg.Username, e.cfg.Password, e.cfg.Host)); err != nil {
		return fmt.Errorf("email auth: %w", err)
	}
	if err := client.Mail(e.cfg.From); err != nil {
		return fmt.Errorf("email from: %w", err)
	}
	if err := client.Rcpt(target); err != nil {
		return fmt.Errorf("email rcpt: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("email data: %w", err)
	}
	if _, err := w.Write([]byte(body)); err != nil {
		return fmt.Errorf("email write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("email close: %w", err)
	}

	return client.Quit()
}

func (e *Email) defaultDialer() (*smtp.Client, error) {
	addr := fmt.Sprintf("%s:%d", e.cfg.Host, e.cfg.Port)

	if e.cfg.UseTLS || e.cfg.Port == 587 {
		conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
		if err != nil {
			return nil, err
		}
		client, err := smtp.NewClient(conn, e.cfg.Host)
		if err != nil {
			return nil, err
		}
		tlsConfig := &tls.Config{ServerName: e.cfg.Host}
		if err := client.StartTLS(tlsConfig); err != nil {
			client.Close()
			return nil, fmt.Errorf("STARTTLS: %w", err)
		}
		return client, nil
	}

	// Direct TLS (port 465)
	tlsConfig := &tls.Config{ServerName: e.cfg.Host}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, tlsConfig)
	if err != nil {
		return nil, err
	}
	return smtp.NewClient(conn, e.cfg.Host)
}

func buildEmailBody(from, to, subject, content string) string {
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("\r\n")
	b.WriteString(content)
	return b.String()
}

// extractSubject tries to extract a subject line from the first line of content.
func extractSubject(content string) (string, bool) {
	first, _, ok := strings.Cut(content, "\n")
	if !ok {
		return "", false
	}
	first = strings.TrimSpace(first)
	if len(first) > 5 && len(first) < 100 && !strings.Contains(first, "\r") {
		return first, true
	}
	return "", false
}
