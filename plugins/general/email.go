package general

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"strings"
	"time"

	"yunque-agent/pkg/skills"
)

// ──────────────────────────────────────────────
// SendEmailSkill — send emails via SMTP
// Supports TLS, auth, attachments (text-only for now).
// ──────────────────────────────────────────────

type SendEmailSkill struct{}

func NewSendEmailSkill() *SendEmailSkill { return &SendEmailSkill{} }

func (s *SendEmailSkill) Name() string { return "send_email" }
func (s *SendEmailSkill) Description() string {
	return "发送电子邮件。支持 SMTP 服务器配置，可发送纯文本或 HTML 邮件。需要配置环境变量 SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS"
}

func (s *SendEmailSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"to": map[string]any{
				"type":        "string",
				"description": "收件人地址，多个地址用逗号分隔",
			},
			"subject": map[string]any{
				"type":        "string",
				"description": "邮件主题",
			},
			"body": map[string]any{
				"type":        "string",
				"description": "邮件正文（支持纯文本或 HTML）",
			},
			"html": map[string]any{
				"type":        "boolean",
				"description": "是否发送 HTML 格式邮件（默认 false，发送纯文本）",
			},
			"cc": map[string]any{
				"type":        "string",
				"description": "抄送地址（可选），多个地址用逗号分隔",
			},
			"from_name": map[string]any{
				"type":        "string",
				"description": "发件人显示名称（可选，默认使用 SMTP_USER）",
			},
		},
		"required": []string{"to", "subject", "body"},
	}
}

func (s *SendEmailSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	to, _ := args["to"].(string)
	subject, _ := args["subject"].(string)
	body, _ := args["body"].(string)
	isHTML, _ := args["html"].(bool)
	cc, _ := args["cc"].(string)
	fromName, _ := args["from_name"].(string)

	if to == "" || subject == "" || body == "" {
		return "", fmt.Errorf("to, subject, and body are required")
	}

	// Read SMTP config from environment
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")
	user := os.Getenv("SMTP_USER")
	pass := os.Getenv("SMTP_PASS")

	if host == "" {
		return "", fmt.Errorf("SMTP_HOST environment variable not set. Please configure SMTP settings")
	}
	if port == "" {
		port = "587" // default TLS port
	}
	if user == "" {
		return "", fmt.Errorf("SMTP_USER environment variable not set")
	}
	if pass == "" {
		return "", fmt.Errorf("SMTP_PASS environment variable not set")
	}
	if fromName == "" {
		fromName = "Yunque Agent"
	}

	// Parse recipients
	toAddrs := parseEmailList(to)
	ccAddrs := parseEmailList(cc)
	allRecipients := append(toAddrs, ccAddrs...)

	if len(toAddrs) == 0 {
		return "", fmt.Errorf("no valid recipients in 'to' field")
	}

	// Build MIME message
	from := user
	msg := buildMIMEMessage(from, fromName, toAddrs, ccAddrs, subject, body, isHTML)

	// Send via SMTP with TLS
	if err := sendSMTP(ctx, host, port, user, pass, from, allRecipients, msg); err != nil {
		return "", fmt.Errorf("send failed: %w", err)
	}

	result := fmt.Sprintf("邮件发送成功！\n收件人: %s\n主题: %s", strings.Join(toAddrs, ", "), subject)
	if len(ccAddrs) > 0 {
		result += fmt.Sprintf("\n抄送: %s", strings.Join(ccAddrs, ", "))
	}
	return result, nil
}

func parseEmailList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var addrs []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" && strings.Contains(p, "@") {
			addrs = append(addrs, p)
		}
	}
	return addrs
}

func buildMIMEMessage(from, fromName string, to, cc []string, subject, body string, isHTML bool) []byte {
	var sb strings.Builder

	// Headers
	sb.WriteString(fmt.Sprintf("From: %s <%s>\r\n", fromName, from))
	sb.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ", ")))
	if len(cc) > 0 {
		sb.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(cc, ", ")))
	}
	sb.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	sb.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	sb.WriteString("MIME-Version: 1.0\r\n")

	if isHTML {
		sb.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	} else {
		sb.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	}
	sb.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(body)

	return []byte(sb.String())
}

func sendSMTP(ctx context.Context, host, port, user, pass, from string, to []string, msg []byte) error {
	addr := net.JoinHostPort(host, port)

	// Try TLS first (port 465), then STARTTLS (port 587), then plain
	if port == "465" {
		return sendSMTPImplicitTLS(addr, host, user, pass, from, to, msg)
	}
	return sendSMTPStartTLS(addr, host, user, pass, from, to, msg)
}

func sendSMTPStartTLS(addr, host, user, pass, from string, to []string, msg []byte) error {
	c, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}
	defer c.Close()

	// STARTTLS
	tlsConfig := &tls.Config{ServerName: host}
	if err := c.StartTLS(tlsConfig); err != nil {
		// Continue without TLS if not supported
		_ = err
	}

	// Auth
	auth := smtp.PlainAuth("", user, pass, host)
	if err := c.Auth(auth); err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	// Send
	if err := c.Mail(from); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}
	for _, rcpt := range to {
		if err := c.Rcpt(rcpt); err != nil {
			return fmt.Errorf("RCPT TO %s: %w", rcpt, err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close DATA: %w", err)
	}
	return c.Quit()
}

func sendSMTPImplicitTLS(addr, host, user, pass, from string, to []string, msg []byte) error {
	tlsConfig := &tls.Config{ServerName: host}
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS dial %s: %w", addr, err)
	}

	c, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("new client: %w", err)
	}
	defer c.Close()

	auth := smtp.PlainAuth("", user, pass, host)
	if err := c.Auth(auth); err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	if err := c.Mail(from); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}
	for _, rcpt := range to {
		if err := c.Rcpt(rcpt); err != nil {
			return fmt.Errorf("RCPT TO %s: %w", rcpt, err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close DATA: %w", err)
	}
	return c.Quit()
}
