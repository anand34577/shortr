package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

// SealValue produces a compact signed, expiring token suitable for a cookie
// value: base64(payload).base64(expiryUnix).base64(hmac). Used for the OIDC
// state cookie and the link-password cookie — stateless, no server-side
// storage, tamper-evident.
func SealValue(secret []byte, payload string, ttl time.Duration) string {
	exp := time.Now().Add(ttl).Unix()
	expStr := strconv.FormatInt(exp, 10)
	mac := computeMAC(secret, payload, expStr)
	return b64(payload) + "." + b64(expStr) + "." + b64(mac)
}

var ErrSealInvalid = errors.New("auth: invalid or expired sealed value")

func OpenValue(secret []byte, token string) (string, error) {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return "", ErrSealInvalid
	}
	payload, err1 := unb64(parts[0])
	expStr, err2 := unb64(parts[1])
	mac, err3 := unb64(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return "", ErrSealInvalid
	}
	want := computeMAC(secret, payload, expStr)
	if !ConstantTimeEqual(mac, want) {
		return "", ErrSealInvalid
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return "", ErrSealInvalid
	}
	return payload, nil
}

func computeMAC(secret []byte, payload, expStr string) string {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(payload))
	m.Write([]byte{0})
	m.Write([]byte(expStr))
	return string(m.Sum(nil))
}

func b64(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
func unb64(s string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	return string(b), err
}
