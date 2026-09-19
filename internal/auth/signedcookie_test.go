package auth

import (
	"testing"
	"time"
)

func TestSealOpenRoundTrip(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	token := SealValue(secret, `{"a":1}`, time.Minute)
	got, err := OpenValue(secret, token)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if got != `{"a":1}` {
		t.Fatalf("got %q", got)
	}
}

func TestSealExpired(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	token := SealValue(secret, "x", -time.Second)
	if _, err := OpenValue(secret, token); err != ErrSealInvalid {
		t.Fatalf("expected expired error, got %v", err)
	}
}

func TestSealTampered(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	token := SealValue(secret, "x", time.Minute)
	tampered := token[:len(token)-2] + "zz"
	if _, err := OpenValue(secret, tampered); err != ErrSealInvalid {
		t.Fatalf("expected tamper detection, got %v", err)
	}
}

func TestSealWrongSecret(t *testing.T) {
	token := SealValue([]byte("0123456789abcdef0123456789abcdef"), "x", time.Minute)
	if _, err := OpenValue([]byte("ffffffffffffffffffffffffffffffff"), token); err != ErrSealInvalid {
		t.Fatalf("expected invalid with wrong secret, got %v", err)
	}
}
