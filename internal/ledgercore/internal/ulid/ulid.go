package ulid

import (
	"crypto/rand"
	"encoding/binary"
	"sync"
	"time"
)

// ULID generates Universally Unique Lexicographically Sortable Identifiers.
// Format: 10-byte timestamp (ms) + 10-byte random = 26 chars Crockford base32.
//
// This is a minimal implementation sufficient for Ledger's needs.
// Time-ordered IDs ensure natural sort order matches creation order.

var (
	mu      sync.Mutex
	lastMs  int64
	lastRnd uint64
)

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// New generates a new ULID at the current time.
func New() string {
	return NewAt(time.Now())
}

// NewAt generates a new ULID at the given time.
func NewAt(t time.Time) string {
	ms := t.UnixMilli()

	mu.Lock()
	if ms == lastMs {
		lastRnd++
	} else {
		lastMs = ms
		var buf [8]byte
		rand.Read(buf[:])
		lastRnd = binary.BigEndian.Uint64(buf[:])
	}
	rnd := lastRnd
	mu.Unlock()

	var out [26]byte

	// Encode 48-bit timestamp into first 10 chars
	for i := 9; i >= 0; i-- {
		out[i] = crockford[ms&0x1F]
		ms >>= 5
	}

	// Encode 80-bit randomness into last 16 chars
	// We use only 64 bits of randomness here (sufficient for single-process)
	for i := 25; i >= 10; i-- {
		out[i] = crockford[rnd&0x1F]
		rnd >>= 5
	}

	return string(out[:])
}

// Timestamp extracts the millisecond timestamp from a ULID string.
// Returns zero time on invalid input.
func Timestamp(id string) time.Time {
	if len(id) != 26 {
		return time.Time{}
	}
	var ms int64
	for i := 0; i < 10; i++ {
		v := decodeChar(id[i])
		if v < 0 {
			return time.Time{}
		}
		ms = (ms << 5) | int64(v)
	}
	return time.UnixMilli(ms)
}

func decodeChar(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'A' && c <= 'H':
		return int(c-'A') + 10
	case c == 'J' || c == 'K':
		return int(c-'J') + 18
	case c >= 'M' && c <= 'N':
		return int(c-'M') + 20
	case c >= 'P' && c <= 'T':
		return int(c-'P') + 22
	case c >= 'V' && c <= 'Z':
		return int(c-'V') + 27
	default:
		return -1
	}
}

// Prefix generates a ULID prefix for efficient range-based queries.
// All ULIDs generated after the given time will be lexicographically greater.
func Prefix(t time.Time) string {
	id := NewAt(t)
	return id[:10] // timestamp portion only
}
