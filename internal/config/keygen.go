package config

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateSecureKey generates a cryptographically secure random hex key.
func GenerateSecureKey(byteLen int) string {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		panic("config: failed to generate secure key: " + err.Error())
	}
	return hex.EncodeToString(b)
}
