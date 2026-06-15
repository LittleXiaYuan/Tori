package cognisdk

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"
)

// CanonicalPackBundle validates and returns a deterministic copy of a bundle
// for hashing, review evidence, and reproducible automation logs.
func CanonicalPackBundle(bundle PackBundle) (PackBundle, error) {
	if err := ValidatePackBundle(bundle); err != nil {
		return PackBundle{}, err
	}
	canonical := bundle
	// CreatedAt is wall-clock metadata, not bundle content. Two bundles built
	// from identical packs moments apart would otherwise hash differently
	// (flaky, e.g. TestDigestPackBundleIsStableAcrossPackOrder) and the digest
	// would not be reproducible. Zero it so the digest depends only on content.
	canonical.CreatedAt = time.Time{}
	canonical.Packs = append([]PackManifest(nil), bundle.Packs...)
	sort.Slice(canonical.Packs, func(i, j int) bool { return canonical.Packs[i].ID < canonical.Packs[j].ID })
	canonical.EnabledPacks = append([]string(nil), bundle.EnabledPacks...)
	sort.Strings(canonical.EnabledPacks)
	return canonical, nil
}

// DigestPackBundle returns a stable sha256 digest for a validated bundle.
func DigestPackBundle(bundle PackBundle) (string, error) {
	canonical, err := CanonicalPackBundle(bundle)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// VerifyPackBundleDigest compares a bundle with an expected digest without
// mutating the bundle or host state.
func VerifyPackBundleDigest(bundle PackBundle, expected string) (PackBundleDigestCheck, error) {
	actual, err := DigestPackBundle(bundle)
	if err != nil {
		return PackBundleDigestCheck{}, err
	}
	check := PackBundleDigestCheck{
		BundleID: bundle.ID,
		Expected: expected,
		Actual:   actual,
		Match:    actual == expected,
	}
	return check, nil
}
