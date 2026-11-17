package platform

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"
)

// Mailer mengirim email via SMTP (SMTP/SES/Resend — PRD 7.2). Dev memakai Mailpit
// (deploy/docker-compose.dev.yml, port 1025, tanpa TLS/auth).
type Mailer struct {
	Host string
	Port int
	User string
	Pass string
	From string
}

func NewMailerFromEnv() *Mailer {
	return &Mailer{
		Host: env("SMTP_HOST", "localhost"),
		Port: envInt("SMTP_PORT", 1025),
		User: os.Getenv("SMTP_USER"),
		Pass: os.Getenv("SMTP_PASS"),
		From: env("SMTP_FROM", "FlowCBT <no-reply@flowcbt.local>"),
	}
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n := 0
	for _, c := range v {
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// Send mengirim satu email HTML. Timeout dipimpin context.
func (m *Mailer) Send(_ context.Context, to, subject, html string) error {
	addr := net.JoinHostPort(m.Host, strconv.Itoa(m.Port))

	headers := strings.Join([]string{
		"From: " + m.From,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"", // baris kosong pemisah header/body
	}, "\r\n")
	msg := []byte(headers + html + "\r\n")

	d := net.Dialer{Timeout: 10 * time.Second}
	conn, err := d.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	host := m.Host
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer c.Close()

	// TLS eksplisit (465) atau StartTLS bila ditawarkan server.
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err = c.StartTLS(&tls.Config{ServerName: host}); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	} else if m.Port == 465 {
		return fmt.Errorf("SMTP 465 butuh TLS eksplisit: ganti pakai StartTLS (587) atau SMTP lokal")
	}

	if m.User != "" && m.Pass != "" {
		auth := smtp.PlainAuth("", m.User, m.Pass, host)
		if ok, _ := c.Extension("AUTH"); ok {
			if err = c.Auth(auth); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
	}

	if err = c.Mail(m.From); err != nil {
		return err
	}
	if err = c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write(msg); err != nil {
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	return c.Quit()
}
