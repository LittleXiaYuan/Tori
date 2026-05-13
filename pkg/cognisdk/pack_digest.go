package cognisdk

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// CanonicalPackBundle validates and returns a deterministic copy of a bundle
// for hashing, review evidence, and reproducible automation logs.
func CanonicalPackBundle(bundle PackBundle) (PackBundle, error) {
	if err := ValidatePackBundle(bundle); err != nil {
		return PackBundle{}, err
	}
	canonical := bundle
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
