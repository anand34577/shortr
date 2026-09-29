package notify

import (
	"bufio"
	"encoding/base64"
	"net"
	"strings"
	"testing"
)

// fakeRelay is a minimal plain-text SMTP server (no STARTTLS), like a LAN
// relay. It records the AUTH PLAIN payload and the DATA it received.
func fakeRelay(t *testing.T) (port int, got chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	got = make(chan string, 4)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				r := bufio.NewReader(c)
				say := func(s string) { _, _ = c.Write([]byte(s + "\r\n")) }
				say("220 relay ready")
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					cmd := strings.ToUpper(strings.TrimSpace(line))
					switch {
					case strings.HasPrefix(cmd, "EHLO"):
						say("250-relay")
						say("250 AUTH PLAIN")
					case strings.HasPrefix(cmd, "AUTH PLAIN "):
						raw, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(line)[len("AUTH PLAIN "):])
						got <- "auth:" + string(raw)
						say("235 ok")
					case strings.HasPrefix(cmd, "MAIL"), strings.HasPrefix(cmd, "RCPT"):
						got <- strings.TrimSpace(line)
						say("250 ok")
					case cmd == "DATA":
						say("354 go on")
						var body strings.Builder
						for {
							l, err := r.ReadString('\n')
							if err != nil || l == ".\r\n" {
								break
							}
							body.WriteString(l)
						}
						got <- "data:" + body.String()
						say("250 queued")
					case cmd == "QUIT":
						say("221 bye")
						return
					default:
						say("250 ok")
					}
				}
			}(conn)
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, got
}

func TestSMTPUnencryptedRelayWithAuth(t *testing.T) {
	port, got := fakeRelay(t)
	s := NewSMTPSender(SMTPConfig{Enabled: true, Host: "127.0.0.1", Port: port, User: "u", Pass: "p",
		From: "Shortr <noreply@home.lan>", TLSMode: TLSNone})
	if err := s.Send("me@home.lan", "Grüße", "hello\nworld"); err != nil {
		t.Fatalf("send over plain relay: %v", err)
	}
	var all []string
	for len(all) < 4 {
		all = append(all, <-got)
	}
	joined := strings.Join(all, "\n")
	for _, want := range []string{"auth:\x00u\x00p", "MAIL FROM:<noreply@home.lan>", "RCPT TO:<me@home.lan>", "hello\r\nworld", "Subject: =?utf-8?q?"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
}

func TestSMTPStartTLSRefusesDowngrade(t *testing.T) {
	port, _ := fakeRelay(t)
	s := NewSMTPSender(SMTPConfig{Enabled: true, Host: "127.0.0.1", Port: port, User: "u", Pass: "p",
		From: "noreply@home.lan", TLSMode: TLSStartTLS})
	err := s.Send("me@home.lan", "x", "y")
	if err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatalf("expected a clear STARTTLS error, got %v", err)
	}
}
