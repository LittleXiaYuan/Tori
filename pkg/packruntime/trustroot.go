package packruntime

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// TrustRoot resolves (publisherID, publicKeyID) to an ed25519 public key.
//
// Two layers contribute keys:
//  1. Embedded — compile-time keys baked into the binary, populated via
//     RegisterEmbeddedTrust. The release build seeds yunque-official's master
//     key here.
//  2. Disk — files under <root>/trust/*.pub, each containing one record of
//     the form "<publisherID> <publicKeyID> <base64(pub)>". Disk keys are
//     opt-in and only added when the user explicitly trusts a publisher
//     through the UI.
type TrustRoot struct {
	mu       sync.RWMutex
	embedded map[trustKey]ed25519.PublicKey
	disk     map[trustKey]ed25519.PublicKey
	diskRoot string
}

type trustKey struct {
	publisher string
	publicKey string
}

var (
	embeddedMu   sync.RWMutex
	embeddedKeys = map[trustKey]ed25519.PublicKey{}
)

// RegisterEmbeddedTrust adds a publisher public key to the process-wide
// embedded trust root. Called from init() in a generated registration file.
// Re-registering the same (publisherID, publicKeyID) replaces the previous
// value — useful for tests; release builds register exactly once.
func RegisterEmbeddedTrust(publisherID, publicKeyID string, pub ed25519.PublicKey) error {
	if strings.TrimSpace(publisherID) == "" || strings.TrimSpace(publicKeyID) == "" {
		return fmt.Errorf("trust: publisher id and publicKeyId are required")
	}
	if len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("trust: public key size must be %d, got %d", ed25519.PublicKeySize, len(pub))
	}
	embeddedMu.Lock()
	defer embeddedMu.Unlock()
	embeddedKeys[trustKey{publisherID, publicKeyID}] = append(ed25519.PublicKey(nil), pub...)
	return nil
}

// NewTrustRoot snapshots the embedded keys and prepares the disk loader.
// diskRoot is typically <appdir>/packs; disk keys are looked up under
// <diskRoot>/trust/*.pub.
func NewTrustRoot(diskRoot string) *TrustRoot {
	embeddedMu.RLock()
	snapshot := make(map[trustKey]ed25519.PublicKey, len(embeddedKeys))
	for k, v := range embeddedKeys {
		snapshot[k] = append(ed25519.PublicKey(nil), v...)
	}
	embeddedMu.RUnlock()
	return &TrustRoot{
		embedded: snapshot,
		disk:     map[trustKey]ed25519.PublicKey{},
		diskRoot: diskRoot,
	}
}

// LoadDisk reads every <diskRoot>/trust/*.pub file and registers the keys.
// Missing directory is treated as empty (not an error). Per-file errors are
// surfaced individually so a single malformed key doesn't poison the rest.
func (t *TrustRoot) LoadDisk() error {
	if t == nil || strings.TrimSpace(t.diskRoot) == "" {
		return nil
	}
	dir := filepath.Join(t.diskRoot, "trust")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("trust: read %s: %w", dir, err)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	var firstErr error
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".pub") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		records, err := readTrustFile(path)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for k, pub := range records {
			t.disk[k] = pub
		}
	}
	return firstErr
}

// AddDiskKey registers a single (publisherID, publicKeyID) → pub mapping in
// memory and writes <diskRoot>/trust/<publisherID>__<publicKeyID>.pub. Used
// by the UI's "Add Publisher" flow.
func (t *TrustRoot) AddDiskKey(publisherID, publicKeyID string, pub ed25519.PublicKey) error {
	if t == nil {
		return fmt.Errorf("trust: nil root")
	}
	if len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("trust: public key size must be %d, got %d", ed25519.PublicKeySize, len(pub))
	}
	t.mu.Lock()
	t.disk[trustKey{publisherID, publicKeyID}] = append(ed25519.PublicKey(nil), pub...)
	root := t.diskRoot
	t.mu.Unlock()
	if root == "" {
		return nil
	}
	dir := filepath.Join(root, "trust")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("trust: create dir: %w", err)
	}
	safeFile := safeArtifactSegment(publisherID) + "__" + safeArtifactSegment(publicKeyID) + ".pub"
	line := fmt.Sprintf("%s %s %s\n", publisherID, publicKeyID, base64.StdEncoding.EncodeToString(pub))
	return os.WriteFile(filepath.Join(dir, safeFile), []byte(line), 0o644)
}

// Resolve implements PublicKeyResolver. Embedded keys take precedence over
// disk keys with the same (publisherID, publicKeyID).
func (t *TrustRoot) Resolve(publisherID, publicKeyID string) (ed25519.PublicKey, error) {
	if t == nil {
		return nil, fmt.Errorf("trust: nil root")
	}
	key := trustKey{publisherID, publicKeyID}
	t.mu.RLock()
	defer t.mu.RUnlock()
	if pub, ok := t.embedded[key]; ok {
		return pub, nil
	}
	if pub, ok := t.disk[key]; ok {
		return pub, nil
	}
	return nil, fmt.Errorf("trust: no public key for publisher=%q keyId=%q", publisherID, publicKeyID)
}

func readTrustFile(path string) (map[trustKey]ed25519.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("trust: read %s: %w", path, err)
	}
	out := map[trustKey]ed25519.PublicKey{}
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return out, fmt.Errorf("trust: %s line %d: expected 3 fields, got %d", path, i+1, len(fields))
		}
		pub, err := decodeTrustKey(fields[2])
		if err != nil {
			return out, fmt.Errorf("trust: %s line %d: %w", path, i+1, err)
		}
		out[trustKey{fields[0], fields[1]}] = pub
	}
	return out, nil
}

// decodeTrustKey accepts base64 (standard) or hex.
func decodeTrustKey(s string) (ed25519.PublicKey, error) {
	if b, err := base64.StdEncoding.DecodeString(s); err == nil && len(b) == ed25519.PublicKeySize {
		return ed25519.PublicKey(b), nil
	}
	if b, err := hex.DecodeString(s); err == nil && len(b) == ed25519.PublicKeySize {
		return ed25519.PublicKey(b), nil
	}
	return nil, fmt.Errorf("invalid public key encoding")
}
