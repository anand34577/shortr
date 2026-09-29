// Package notify fans a notification event out to the channels a user (or
// the system, for admin alerts) has enabled: an in-app row (always, for the
// bell / browser-Notification-API feed the frontend polls), optional email
// (SMTP), and optional Gotify push. All are best-effort: a delivery failure
// on one channel never blocks another or the caller. Both SMTP and Gotify are
// entirely optional and
// self-configured, never required for core function.
package notify

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"time"
)

// sendTimeout bounds every SMTP network operation (dial, auth, data) so a
// hung or blackholed mail host can't leak a goroutine+socket per send.
const sendTimeout = 10 * time.Second

// TLS modes for SMTPConfig.TLSMode.
const (
	TLSStartTLS = "starttls" // plain connect, then STARTTLS (required) — usually port 587
	TLSImplicit = "tls"      // TLS from the first byte — usually port 465
	TLSNone     = "none"     // no encryption at all, e.g. a LAN relay on port 25
)

type SMTPConfig struct {
	Enabled  bool
	Host     string
	Port     int
	User     string
	Pass     string
	From     string
	TLSMode  string // starttls | tls | none
	Insecure bool   // skip certificate verification (self-signed relay)
}

type SMTPSender struct {
	cfg SMTPConfig
}

func NewSMTPSender(cfg SMTPConfig) *SMTPSender {
	if cfg.TLSMode == "" {
		cfg.TLSMode = TLSStartTLS
	}
	return &SMTPSender{cfg: cfg}
}

func (s *SMTPSender) Enabled() bool { return s.cfg.Enabled }

// Send delivers a plain-text email. It never touches the redirect hot path;
// callers should invoke it from a goroutine or background job.
func (s *SMTPSender) Send(to, subject, body string) error {
	if !s.cfg.Enabled {
		return nil
	}
	addr := net.JoinHostPort(s.cfg.Host, fmt.Sprint(s.cfg.Port))
	tlsCfg := &tls.Config{ServerName: s.cfg.Host, InsecureSkipVerify: s.cfg.Insecure} //nolint:gosec // opt-in for self-signed relays

	var conn net.Conn
	var err error
	dialer := &net.Dialer{Timeout: sendTimeout}
	if s.cfg.TLSMode == TLSImplicit {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsCfg)
	} else {
		conn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("smtp dial %s: %w", addr, err)
	}
	_ = conn.SetDeadline(time.Now().Add(sendTimeout))
	c, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer c.Close()
	if err := c.Hello(heloName()); err != nil {
		return fmt.Errorf("smtp hello: %w", err)
	}

	if s.cfg.TLSMode == TLSStartTLS {
		// Refuse to fall back to plain text: that would let anyone on the
		// path strip STARTTLS and read the password. Use TLSNone on purpose.
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return errors.New("smtp: server does not offer STARTTLS; set SHORTR_SMTP_TLS=none for an unencrypted relay")
		}
		if err := c.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}

	if s.cfg.User != "" {
		if ok, _ := c.Extension("AUTH"); ok {
			var auth smtp.Auth = smtp.PlainAuth("", s.cfg.User, s.cfg.Pass, s.cfg.Host)
			if s.cfg.TLSMode == TLSNone {
				auth = plainAuthNoTLS{user: s.cfg.User, pass: s.cfg.Pass}
			}
			if err := c.Auth(auth); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
	}
	return s.deliver(c, to, buildMessage(s.cfg.From, to, subject, body))
}

// plainAuthNoTLS is AUTH PLAIN without net/smtp's "only over TLS or to
// localhost" guard. Only used when the admin chose SHORTR_SMTP_TLS=none,
// i.e. a trusted LAN relay.
type plainAuthNoTLS struct{ user, pass string }

func (a plainAuthNoTLS) Start(*smtp.ServerInfo) (string, []byte, error) {
	return "PLAIN", []byte("\x00" + a.user + "\x00" + a.pass), nil
}

func (a plainAuthNoTLS) Next(_ []byte, more bool) ([]byte, error) {
	if more {
		return nil, errors.New("smtp: unexpected server challenge")
	}
	return nil, nil
}

func (s *SMTPSender) deliver(c *smtp.Client, to string, msg []byte) error {
	from := s.cfg.From
	if a, err := mail.ParseAddress(from); err == nil {
		from = a.Address // "Shortr <noreply@x>" -> envelope wants the bare address
	}
	if err := c.Mail(from); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("smtp RCPT TO: %w", err)
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

func heloName() string {
	if h, err := os.Hostname(); err == nil && h != "" && !strings.ContainsAny(h, " \r\n") {
		return h
	}
	return "localhost"
}

func buildMessage(from, to, subject, body string) []byte {
	domain := "shortr.local"
	if a, err := mail.ParseAddress(from); err == nil {
		if i := strings.LastIndexByte(a.Address, '@'); i >= 0 {
			domain = a.Address[i+1:]
		}
	}
	id := make([]byte, 12)
	_, _ = rand.Read(id)

	var sb strings.Builder
	sb.WriteString("From: " + from + "\r\n")
	sb.WriteString("To: " + to + "\r\n")
	sb.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n")
	sb.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	sb.WriteString("Message-ID: <" + hex.EncodeToString(id) + "@" + domain + ">\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	sb.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\n", "\r\n"))
	return []byte(sb.String())
}
