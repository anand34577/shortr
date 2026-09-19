package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	h, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	ok, rehash, err := VerifyPassword("correct horse battery staple", h)
	if err != nil || !ok {
		t.Fatalf("expected match: ok=%v err=%v", ok, err)
	}
	if rehash {
		t.Fatal("freshly hashed password should not need rehash")
	}
	ok, _, err = VerifyPassword("wrong password", h)
	if err != nil || ok {
		t.Fatalf("expected mismatch: ok=%v err=%v", ok, err)
	}
}

func TestVerifyPasswordInvalidHash(t *testing.T) {
	_, _, err := VerifyPassword("x", "not-a-hash")
	if err != ErrInvalidHash {
		t.Fatalf("expected ErrInvalidHash, got %v", err)
	}
}
