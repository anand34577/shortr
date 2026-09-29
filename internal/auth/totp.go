package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // RFC 6238 default; every authenticator app expects SHA-1
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// TOTP parameters: the defaults every authenticator app understands.
const (
	totpPeriod = 30
	totpDigits = 6
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewTOTPSecret returns a random 160-bit secret, base32 encoded.
func NewTOTPSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return b32.EncodeToString(b), nil
}

// TOTPURI builds the otpauth:// link that authenticator apps scan.
func TOTPURI(issuer, account, secret string) string {
	label := url.PathEscape(issuer + ":" + account)
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprint(totpDigits))
	q.Set("period", fmt.Sprint(totpPeriod))
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// TOTPCode computes the code for a 30-second step.
func TOTPCode(secret string, step int64) (string, error) {
	key, err := b32.DecodeString(strings.ToUpper(strings.TrimRight(secret, "=")))
	if err != nil {
		return "", err
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step))
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	v := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", totpDigits, v%1_000_000), nil
}

// TOTPStep is the current 30-second step number.
func TOTPStep(t time.Time) int64 { return t.Unix() / totpPeriod }

// VerifyTOTP checks code against the steps around now (±1, about ±30s of
// clock drift) and returns the matching step so the caller can refuse its
// reuse. Comparison is constant time.
func VerifyTOTP(secret, code string, now time.Time) (int64, bool) {
	code = strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	if len(code) != totpDigits {
		return 0, false
	}
	cur := TOTPStep(now)
	for _, step := range []int64{cur, cur - 1, cur + 1} {
		want, err := TOTPCode(secret, step)
		if err == nil && hmac.Equal([]byte(want), []byte(code)) {
			return step, true
		}
	}
	return 0, false
}

// --- secret sealing -------------------------------------------------------

// mfaKey derives a dedicated AES-256 key from the instance secret, so a
// database dump alone does not reveal anyone's TOTP secret.
func mfaKey(instanceSecret []byte) []byte {
	sum := sha256.Sum256(append([]byte("shortr-mfa-v1:"), instanceSecret...))
	return sum[:]
}

func SealSecret(instanceSecret []byte, plaintext string) (string, error) {
	block, err := aes.NewCipher(mfaKey(instanceSecret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(plaintext), nil)), nil
}

func OpenSecret(instanceSecret []byte, sealed string) (string, error) {
	raw, err := base64.RawStdEncoding.DecodeString(sealed)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(mfaKey(instanceSecret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("mfa: sealed secret too short")
	}
	pt, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", errors.New("mfa: cannot decrypt secret (was SHORTR_SECRET_KEY changed?)")
	}
	return string(pt), nil
}

// --- recovery codes -------------------------------------------------------

const recoveryAlphabet = "abcdefghjkmnpqrstuvwxyz23456789" // no 0/o, 1/l/i

// NewRecoveryCodes returns n codes like "k7mq-p2xd-w9fh" (about 59 bits
// each) and their hashes for storage.
func NewRecoveryCodes(n int) (codes, hashes []string, err error) {
	for i := 0; i < n; i++ {
		buf := make([]byte, 12)
		if _, err := rand.Read(buf); err != nil {
			return nil, nil, err
		}
		var sb strings.Builder
		for j, b := range buf {
			if j > 0 && j%4 == 0 {
				sb.WriteByte('-')
			}
			sb.WriteByte(recoveryAlphabet[int(b)%len(recoveryAlphabet)])
		}
		c := sb.String()
		codes = append(codes, c)
		hashes = append(hashes, HashRecoveryCode(c))
	}
	return codes, hashes, nil
}

// HashRecoveryCode normalises (case, dashes, spaces) and hashes a code.
func HashRecoveryCode(code string) string {
	c := strings.ToLower(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(code)))
	sum := sha256.Sum256([]byte("shortr-recovery:" + c))
	return hex.EncodeToString(sum[:])
}
