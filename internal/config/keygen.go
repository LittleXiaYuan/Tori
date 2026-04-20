package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateSecureKey generates a cryptographically secure random hex key of
// `byteLen` bytes (so the returned string is 2*byteLen characters).
//
// The old signature (`string` only, panicking on crypto/rand failure) is
// retained via MustGenerateSecureKey so that callers that truly cannot
// proceed without a key keep one-liner ergonomics. New code should prefer
// this error-returning variant and decide locally whether failure is
// fatal (e.g. startup) or recoverable (e.g. rotating a per-session token).
func GenerateSecureKey(byteLen int) (string, error) {
	if byteLen <= 0 {
		return "", fmt.Errorf("config: key length must be positive, got %d", byteLen)
	}
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failure is extraordinarily rare — it means the OS
		// CSPRNG is unavailable. Surface it to the caller so startup code
		// can log a structured slog record and exit cleanly instead of
		// producing a raw panic stack.
		return "", fmt.Errorf("config: crypto/rand unavailable: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// MustGenerateSecureKey preserves the old panic-on-failure contract for
// call sites where the process truly cannot continue without a key (for
// example, generating a one-shot JWT signing secret at boot). Prefer
// GenerateSecureKey in new code.
func MustGenerateSecureKey(byteLen int) string {
	key, err := GenerateSecureKey(byteLen)
	if err != nil {
		panic(err.Error())
	}
	return key
}
