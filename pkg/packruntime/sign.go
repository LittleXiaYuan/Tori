package packruntime

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// CanonicalManifestBytes returns the canonical JSON encoding of m with the
// `signing` field stripped — this is the byte sequence the publisher signs.
//
// The strip is done at the JSON layer (not by clearing m.Signing) so that any
// forward-compatible extra fields the runtime didn't know about still
// participate in the signature material.
func CanonicalManifestBytes(m Manifest) ([]byte, error) {
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("signing: encode manifest: %w", err)
	}
	var generic map[string]interface{}
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, fmt.Errorf("signing: decode manifest: %w", err)
	}
	delete(generic, "signing")
	return CanonicalJSON(generic)
}

// SignManifest replaces m.Signing with a fresh ed25519 signature over the
// canonical manifest bytes. publisher / publicKeyID are stored in m.Publisher
// so the verifier can look up the public key from the trust root.
func SignManifest(m *Manifest, priv ed25519.PrivateKey, publisherID, publicKeyID string) error {
	if m == nil {
		return fmt.Errorf("signing: manifest is nil")
	}
	if len(priv) != ed25519.PrivateKeySize {
		return fmt.Errorf("signing: private key must be %d bytes, got %d", ed25519.PrivateKeySize, len(priv))
	}
	if strings.TrimSpace(publisherID) == "" || strings.TrimSpace(publicKeyID) == "" {
		return fmt.Errorf("signing: publisher id and publicKeyId are required")
	}
	m.Signing = nil
	m.Publisher = PublisherManifest{ID: publisherID, PublicKeyID: publicKeyID}
	canonical, err := CanonicalManifestBytes(*m)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(canonical)
	sig := ed25519.Sign(priv, canonical)
	m.Signing = &SigningManifest{
		Algorithm:      "ed25519",
		ManifestSHA256: hex.EncodeToString(digest[:]),
		Signature:      base64.StdEncoding.EncodeToString(sig),
	}
	return nil
}

// PublicKeyResolver maps (publisherID, publicKeyID) to an ed25519 public key.
// Implementations come from trustroot.go (embedded + on-disk).
type PublicKeyResolver interface {
	Resolve(publisherID, publicKeyID string) (ed25519.PublicKey, error)
}

// VerifyManifest checks m.Signing against the canonical manifest bytes using
// a public key resolved from m.Publisher via the supplied resolver.
//
// Returns nil iff:
//   - m.Signing is present and well-formed
//   - the signature verifies
//   - manifestSha256 matches the canonical bytes' actual sha256
func VerifyManifest(m Manifest, resolver PublicKeyResolver) error {
	if m.Signing == nil {
		return fmt.Errorf("signing: manifest is unsigned")
	}
	if m.Signing.Algorithm != "" && m.Signing.Algorithm != "ed25519" {
		return fmt.Errorf("signing: unsupported algorithm %q", m.Signing.Algorithm)
	}
	if strings.TrimSpace(m.Publisher.ID) == "" || strings.TrimSpace(m.Publisher.PublicKeyID) == "" {
		return fmt.Errorf("signing: manifest missing publisher.id / publisher.publicKeyId")
	}
	if resolver == nil {
		return fmt.Errorf("signing: no trust root configured")
	}
	pub, err := resolver.Resolve(m.Publisher.ID, m.Publisher.PublicKeyID)
	if err != nil {
		return fmt.Errorf("signing: resolve public key: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("signing: public key size mismatch (%d)", len(pub))
	}
	canonical, err := CanonicalManifestBytes(m)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(canonical)
	if !strings.EqualFold(hex.EncodeToString(digest[:]), m.Signing.ManifestSHA256) {
		return fmt.Errorf("signing: manifestSha256 mismatch")
	}
	sigBytes, err := base64.StdEncoding.DecodeString(m.Signing.Signature)
	if err != nil {
		return fmt.Errorf("signing: decode signature: %w", err)
	}
	if !ed25519.Verify(pub, canonical, sigBytes) {
		return fmt.Errorf("signing: ed25519 verify failed")
	}
	return nil
}
