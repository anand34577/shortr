package click

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"time"
)

// ApplyIPMode transforms a raw client IP per the configured privacy mode
// before it is ever written to storage. GeoIP lookups must happen on the
// *raw* IP before this runs (geo.go / writer.go call order enforces that).
func ApplyIPMode(mode string, ip net.IP, secret []byte) string {
	if ip == nil {
		return ""
	}
	switch mode {
	case "full":
		return ip.String()
	case "none":
		return ""
	case "hash":
		day := time.Now().UTC().Format("2006-01-02")
		m := hmac.New(sha256.New, secret)
		m.Write([]byte(day))
		m.Write(ip)
		return hex.EncodeToString(m.Sum(nil))[:32]
	case "anonymize":
		fallthrough
	default:
		if v4 := ip.To4(); v4 != nil {
			return net.IPv4(v4[0], v4[1], v4[2], 0).String() // zero last octet: /24
		}
		v6 := ip.To16()
		if v6 == nil {
			return ""
		}
		masked := make(net.IP, 16)
		copy(masked, v6[:6]) // keep /48, zero the rest
		return net.IP(masked).String()
	}
}

func IPVersion(ip net.IP) int {
	if ip.To4() != nil {
		return 4
	}
	return 6
}
