package ratelimit

import (
	"testing"
	"time"
)

func TestParseRate(t *testing.T) {
	r, err := ParseRate("200/10s")
	if err != nil || r.N != 200 || r.Interval != 10*time.Second {
		t.Fatalf("got %+v err=%v", r, err)
	}
	if _, err := ParseRate("bad"); err == nil {
		t.Fatal("expected error")
	}
}

func TestAllowBurstThenBlock(t *testing.T) {
	l := New(Rate{N: 3, Interval: time.Minute})
	for i := 0; i < 3; i++ {
		if !l.Allow("k") {
			t.Fatalf("expected allow on request %d", i)
		}
	}
	if l.Allow("k") {
		t.Fatal("expected 4th request to be blocked")
	}
}

func TestAllowRefills(t *testing.T) {
	l := New(Rate{N: 2, Interval: 100 * time.Millisecond})
	l.Allow("k")
	l.Allow("k")
	if l.Allow("k") {
		t.Fatal("expected block after burst")
	}
	time.Sleep(120 * time.Millisecond)
	if !l.Allow("k") {
		t.Fatal("expected refill to allow again")
	}
}

func TestAllowDisabledWhenZero(t *testing.T) {
	l := New(Rate{N: 0, Interval: time.Second})
	for i := 0; i < 100; i++ {
		if !l.Allow("k") {
			t.Fatal("rate 0 should mean unlimited")
		}
	}
}

func TestDifferentKeysIndependent(t *testing.T) {
	l := New(Rate{N: 1, Interval: time.Minute})
	if !l.Allow("a") || !l.Allow("b") {
		t.Fatal("different keys should have independent buckets")
	}
	if l.Allow("a") {
		t.Fatal("key a should be exhausted")
	}
}

func TestGC(t *testing.T) {
	l := New(Rate{N: 1, Interval: time.Minute})
	l.Allow("a")
	if l.Len() != 1 {
		t.Fatalf("expected 1 bucket, got %d", l.Len())
	}
	l.GC(0) // idle=0 evicts everything immediately
	if l.Len() != 0 {
		t.Fatalf("expected GC to clear buckets, got %d", l.Len())
	}
}
