package auth

import (
	"encoding/base32"
	"testing"
	"time"
)

// RFC 6238 appendix B, SHA-1 vectors (8 digits there; we compare the last 6).
func TestTOTPRFCVectors(t *testing.T) {
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("12345678901234567890"))
	for ts, want := range map[int64]string{59: "287082", 1111111109: "081804", 1234567890: "005924", 2000000000: "279037"} {
		got, err := TOTPCode(secret, ts/30)
		if err != nil || got != want {
			t.Errorf("t=%d: got %s want %s (%v)", ts, got, want, err)
		}
	}
}

func TestVerifyTOTPWindowAndSealing(t *testing.T) {
	secret, _ := NewTOTPSecret()
	now := time.Unix(1_800_000_000, 0)
	prev, _ := TOTPCode(secret, TOTPStep(now)-1)
	if step, ok := VerifyTOTP(secret, prev, now); !ok || step != TOTPStep(now)-1 {
		t.Fatal("code from the previous step should be accepted with its step")
	}
	old, _ := TOTPCode(secret, TOTPStep(now)-3)
	if _, ok := VerifyTOTP(secret, old, now); ok {
		t.Fatal("stale code accepted")
	}

	key := []byte("0123456789abcdef0123456789abcdef")
	sealed, err := SealSecret(key, secret)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := OpenSecret(key, sealed); err != nil || got != secret {
		t.Fatalf("round trip: %v", err)
	}
	if _, err := OpenSecret([]byte("another-key-another-key-another!!"), sealed); err == nil {
		t.Fatal("opened with the wrong key")
	}
}

func TestRecoveryCodes(t *testing.T) {
	codes, hashes, err := NewRecoveryCodes(10)
	if err != nil || len(codes) != 10 || len(codes[0]) != 14 {
		t.Fatalf("codes: %v %v", codes, err)
	}
	if HashRecoveryCode(" "+codes[3][:4]+" "+codes[3][5:]+" ") != hashes[3] && HashRecoveryCode(codes[3]) != hashes[3] {
		t.Fatal("hash mismatch")
	}
	if HashRecoveryCode("K7MQ-P2XD-W9FH") != HashRecoveryCode("k7mqp2xdw9fh") {
		t.Fatal("normalisation")
	}
}
