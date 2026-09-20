package server

import (
	"context"
	"html"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"shortr/internal/validate"
)

var titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// fetchTitle retrieves a target page's <title> for link previews. It is
// SSRF-guarded: DNS resolution happens inside DialContext so we check the
// IP actually being connected to (defeats DNS-rebinding), not just the
// hostname string; redirects are followed up to 3 times, each re-checked;
// only a bounded amount of text/html is read.
// Best-effort: any failure returns empty strings, never an error
// to the caller — this must never block link creation.
func fetchTitle(ctx context.Context, rawURL string, allowPrivate bool) (title, finalURL string) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	dialer := &net.Dialer{Timeout: 3 * time.Second}
	client := &http.Client{
		Timeout: 3 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return http.ErrUseLastResponse
			}
			return nil
		},
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				conn, err := dialer.DialContext(ctx, network, addr)
				if err != nil {
					return nil, err
				}
				if !allowPrivate {
					host, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
					if ip := net.ParseIP(host); ip != nil && validate.IsPrivateIP(ip) {
						conn.Close()
						return nil, errPrivateTarget
					}
				}
				return conn, nil
			},
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", ""
	}
	req.Header.Set("User-Agent", "Shortr-LinkPreview/1.0 (+https://github.com/)")
	req.Header.Set("Accept", "text/html")

	resp, err := client.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") && ct != "" {
		return "", resp.Request.URL.String()
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	m := titleRe.FindSubmatch(body)
	if m == nil {
		return "", resp.Request.URL.String()
	}
	t := strings.ToValidUTF8(string(m[1]), "")
	t = strings.Join(strings.Fields(html.UnescapeString(t)), " ")
	if r := []rune(t); len(r) > 200 {
		t = string(r[:200])
	}
	return t, resp.Request.URL.String()
}

type privateTargetErr struct{}

func (privateTargetErr) Error() string { return "target resolves to a private/internal address" }

var errPrivateTarget = privateTargetErr{}
