// Package notify fans a notification event out to the channels a user (or
// the system, for admin alerts) has enabled: an in-app row (always, for the
// bell / browser-Notification-API feed the frontend polls), optional email
// (SMTP), and optional Gotify push. All are best-effort: a delivery failure
// on one channel never blocks another or the caller. Both SMTP and Gotify are
// entirely optional and
// self-configured, never required for core function.
package notify

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// sendTimeout bounds every SMTP network operation (dial, auth, data) so a
// hung or blackholed mail host can't leak a goroutine+socket per send.
const sendTimeout = 10 * time.Second

type SMTPConfig struct {
	Enabled  bool
	Host     string
	Port     int
	User     string
	Pass     string
	From     string
	UseTLS   bool
	Insecure bool
}

type SMTPSender struct {
	cfg SMTPConfig
}

func NewSMTPSender(cfg SMTPConfig) *SMTPSender { return &SMTPSender{cfg: cfg} }

func (s *SMTPSender) Enabled() bool { return s.cfg.Enabled }

// Send delivers a plain-text email. It never touches the redirect hot path;
// callers should invoke it from a goroutine or background job.
func (s *SMTPSender) Send(to, subject, body string) error {
	if !s.cfg.Enabled {
		return nil
	}
	addr := net.JoinHostPort(s.cfg.Host, fmt.Sprint(s.cfg.Port))
	msg := buildMessage(s.cfg.From, to, subject, body)

	var auth smtp.Auth
	if s.cfg.User != "" {
		auth = smtp.PlainAuth("", s.cfg.User, s.cfg.Pass, s.cfg.Host)
	}

	if s.cfg.UseTLS {
		return s.sendSTARTTLS(addr, auth, to, msg)
	}
	return s.sendPlain(addr, auth, to, msg)
}

// sendPlain mirrors smtp.SendMail but over a connection with a hard
// deadline, since smtp.SendMail itself has no timeout mechanism.
func (s *SMTPSender) sendPlain(addr string, auth smtp.Auth, to string, msg []byte) error {
	conn, err := net.DialTimeout("tcp", addr, sendTimeout)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	_ = conn.SetDeadline(time.Now().Add(sendTimeout))
	c, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer c.Close()

	if auth != nil {
		if ok, _ := c.Extension("AUTH"); ok {
			if err := c.Auth(auth); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
	}
	return s.deliver(c, to, msg)
}

func (s *SMTPSender) sendSTARTTLS(addr string, auth smtp.Auth, to string, msg []byte) error {
	conn, err := net.DialTimeout("tcp", addr, sendTimeout)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	_ = conn.SetDeadline(time.Now().Add(sendTimeout))
	c, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer c.Close()

	if ok, _ := c.Extension("STARTTLS"); ok {
		tlsCfg := &tls.Config{ServerName: s.cfg.Host, InsecureSkipVerify: s.cfg.Insecure} //nolint:gosec
		if err := c.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}
	if auth != nil {
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	return s.deliver(c, to, msg)
}

func (s *SMTPSender) deliver(c *smtp.Client, to string, msg []byte) error {
	if err := c.Mail(s.cfg.From); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

func buildMessage(from, to, subject, body string) []byte {
	var sb strings.Builder
	sb.WriteString("From: " + from + "\r\n")
	sb.WriteString("To: " + to + "\r\n")
	sb.WriteString("Subject: " + subject + "\r\n")
	sb.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(body)
	return []byte(sb.String())
}
