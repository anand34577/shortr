package server

import (
	"encoding/json"
	"fmt"
	"image/color"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"shortr/internal/auth"
	"shortr/internal/store"
)

var hexColorRE = regexp.MustCompile(`^#?([0-9a-fA-F]{6})$`)

// parseHexColor accepts "rrggbb" or "#rrggbb"; anything else falls back to def.
func parseHexColor(v string, def color.RGBA) color.RGBA {
	m := hexColorRE.FindStringSubmatch(v)
	if m == nil {
		return def
	}
	n, _ := strconv.ParseUint(m[1], 16, 32)
	return color.RGBA{R: uint8(n >> 16), G: uint8(n >> 8), B: uint8(n), A: 255}
}

func hexString(c color.RGBA) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

func qrParams(r *http.Request) (size int, fg, bg color.RGBA) {
	size = 256
	if v := r.URL.Query().Get("size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 64 && n <= 2048 {
			size = n
		}
	}
	fg = parseHexColor(r.URL.Query().Get("fg"), color.RGBA{A: 255})
	bg = parseHexColor(r.URL.Query().Get("bg"), color.RGBA{R: 255, G: 255, B: 255, A: 255})
	return
}

// handleLinkQR serves the PNG QR code (?size=&fg=&bg=).
func (s *Server) handleLinkQR(w http.ResponseWriter, r *http.Request, u *store.User, id string) {
	l := s.getOwnedLink(w, r, u, id)
	if l == nil {
		return
	}
	size, fg, bg := qrParams(r)
	q, err := qrcode.New(s.links.ShortURL(l.Code), qrcode.Medium)
	if err != nil {
		respondError(w, r, err)
		return
	}
	q.ForegroundColor, q.BackgroundColor = fg, bg
	png, err := q.PNG(size)
	if err != nil {
		respondError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = w.Write(png)
}

// handleLinkQRSVG serves the same QR code as a scalable SVG.
func (s *Server) handleLinkQRSVG(w http.ResponseWriter, r *http.Request, u *store.User, id string) {
	l := s.getOwnedLink(w, r, u, id)
	if l == nil {
		return
	}
	size, fg, bg := qrParams(r)
	q, err := qrcode.New(s.links.ShortURL(l.Code), qrcode.Medium)
	if err != nil {
		respondError(w, r, err)
		return
	}
	bm := q.Bitmap()
	n := len(bm)
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" shape-rendering="crispEdges">`, size, size, n, n)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="%s"/><path fill="%s" d="`, n, n, hexString(bg), hexString(fg))
	for y, row := range bm {
		for x, on := range row {
			if on {
				fmt.Fprintf(&b, "M%d %dh1v1h-1z", x, y)
			}
		}
	}
	b.WriteString(`"/></svg>`)
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = w.Write([]byte(b.String()))
}

// exportClicksJSON streams the click log as a JSON array (no buffering of
// the whole result).
func (s *Server) exportClicksJSON(w http.ResponseWriter, r *http.Request, l *store.Link) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+l.Code+`-clicks.json"`)
	_, _ = w.Write([]byte("["))
	first := true
	err := s.store.StreamClicks(r.Context(), l.ID, func(c *store.Click) error {
		row, err := json.Marshal(clickDTO{
			ID: c.ID, LinkID: c.LinkID, TS: c.TS, IP: c.IP, Country: c.Country, Region: c.Region, City: c.City,
			Referrer: c.Referrer, ReferrerHost: c.ReferrerHost, Device: c.Device, OS: c.OS, Browser: c.Browser,
			IsBot: c.IsBot, Lang: c.Lang,
			UTM: utmDTO{c.UTMSource, c.UTMMedium, c.UTMCampaign, c.UTMTerm, c.UTMContent},
		})
		if err != nil {
			return err
		}
		if !first {
			_, _ = w.Write([]byte(","))
		}
		first = false
		_, err = w.Write(row)
		return err
	})
	if err != nil {
		s.log.Warn("click export interrupted", "error", err)
	}
	_, _ = w.Write([]byte("]"))
}

// validTZ reports whether tz is empty or a known IANA zone name.
func validTZ(tz string) bool {
	if tz == "" {
		return true
	}
	_, err := time.LoadLocation(tz)
	return err == nil
}

// handleSudo re-verifies the caller's password and starts a fresh sudo
// window, so sensitive actions work for long-lived sessions.
func (s *Server) handleSudo(w http.ResponseWriter, r *http.Request, u *store.User) {
	var req struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, 2048, &req); err != nil {
		respondError(w, r, err)
		return
	}
	if !s.rlAuth.Allow("sudo:" + u.ID) {
		respondError(w, r, ErrLockedOut)
		return
	}
	if u.PasswordHash == nil {
		respondError(w, r, NewAPIError(http.StatusBadRequest, "BAD_REQUEST", "this account has no password"))
		return
	}
	ok, _, err := auth.VerifyPassword(req.Password, *u.PasswordHash)
	if err != nil || !ok {
		respondError(w, r, NewAPIError(http.StatusUnauthorized, "UNAUTHENTICATED", "incorrect password"))
		return
	}
	sess := sessionFromContext(r.Context())
	if sess == nil {
		respondError(w, r, ErrUnauthenticated)
		return
	}
	s.sudoMu.Lock()
	if s.sudoUntil == nil {
		s.sudoUntil = map[string]time.Time{}
	}
	s.sudoUntil[sess.ID] = time.Now().Add(10 * time.Minute)
	s.sudoMu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}
