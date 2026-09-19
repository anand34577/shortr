// Package ulid generates 26-char, lexicographically sortable, unique ids
// (Crockford base32 timestamp(48 bit ms) + randomness(80 bit)).
// hand-rolled instead of github.com/oklog/ulid — ~40 lines, no dep.
package ulid

import (
	"crypto/rand"
	"strings"
	"sync"
	"time"
)

const encoding = "0123456789ABCDEFGHJKMNPQRSTVWXYZ" // Crockford base32, no I L O U

var (
	mu       sync.Mutex
	lastMS   int64
	lastRand [10]byte
)

// New returns a new ULID string. Monotonic within the same millisecond
// (increments the random part) so ids created in a tight loop still sort.
func New() string {
	mu.Lock()
	defer mu.Unlock()

	ms := time.Now().UnixMilli()
	if ms == lastMS {
		// increment random bytes as a big-endian counter
		for i := 9; i >= 0; i-- {
			lastRand[i]++
			if lastRand[i] != 0 {
				break
			}
		}
	} else {
		lastMS = ms
		if _, err := rand.Read(lastRand[:]); err != nil {
			panic("ulid: crypto/rand unavailable: " + err.Error())
		}
	}

	var b [16]byte
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)
	copy(b[6:], lastRand[:])

	return encode(b)
}

func encode(b [16]byte) string {
	var sb strings.Builder
	sb.Grow(26)
	// 128 bits -> 26 base32 chars (5 bits each, last char uses 3 bits)
	var bits uint64
	var bitCount int
	bi := 0
	for sb.Len() < 26 {
		for bitCount < 5 && bi < 16 {
			bits = bits<<8 | uint64(b[bi])
			bitCount += 8
			bi++
		}
		if bitCount < 5 {
			sb.WriteByte(encoding[(bits<<(5-bitCount))&0x1F])
			bitCount = 0
			continue
		}
		bitCount -= 5
		sb.WriteByte(encoding[(bits>>bitCount)&0x1F])
	}
	return sb.String()
}
