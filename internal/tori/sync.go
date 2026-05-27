package tori

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// SyncPayload is a single data item to be synced.
type SyncPayload struct {
	Key       string `json:"key"`
	Data      []byte `json:"data"`
	Version   int64  `json:"version"`
	Timestamp int64  `json:"timestamp"`
}

// SyncManifest describes the local data state for incremental sync.
type SyncManifest struct {
	Items []SyncManifestItem `json:"items"`
}

type SyncManifestItem struct {
	Key     string `json:"key"`
	Version int64  `json:"version"`
	Size    int    `json:"size"`
}

// SyncClient handles encrypted data synchronization with a Tori instance.
type SyncClient struct {
	baseURL    string
	apiKey     string
	encryptKey []byte // derived from user passphrase
}

// NewSyncClient creates a sync client. The passphrase is used to derive
// an AES-256 key for client-side encryption — Tori never sees plaintext.
func NewSyncClient(toriBaseURL, apiKey, passphrase string) *SyncClient {
	key := sha256.Sum256([]byte(passphrase))
	return &SyncClient{
		baseURL:    strings.TrimRight(toriBaseURL, "/"),
		apiKey:     apiKey,
		encryptKey: key[:],
	}
}

// Push encrypts and uploads a data payload to Tori.
func (sc *SyncClient) Push(ctx context.Context, key string, data []byte, version int64) error {
	encrypted, err := sc.encrypt(data)
	if err != nil {
		return fmt.Errorf("sync encrypt: %w", err)
	}

	payload := SyncPayload{
		Key:       key,
		Data:      encrypted,
		Version:   version,
		Timestamp: time.Now().Unix(),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/sync/push", sc.baseURL)
	req, err := jsonSafeRequest(ctx, http.MethodPost, url, body, 15*time.Second)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+sc.apiKey)

	resp, err := doSafeRequest(req, 15*time.Second)
	if err != nil {
		return fmt.Errorf("sync push: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sync push: status %d", resp.StatusCode)
	}
	return nil
}

// Pull downloads and decrypts a data payload from Tori.
func (sc *SyncClient) Pull(ctx context.Context, key string) ([]byte, int64, error) {
	url := fmt.Sprintf("%s/api/sync/pull?key=%s", sc.baseURL, key)
	req, err := jsonSafeRequest(ctx, http.MethodGet, url, nil, 15*time.Second)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+sc.apiKey)

	resp, err := doSafeRequest(req, 15*time.Second)
	if err != nil {
		return nil, 0, fmt.Errorf("sync pull: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, 0, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("sync pull: status %d", resp.StatusCode)
	}

	var payload SyncPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, 0, fmt.Errorf("sync decode: %w", err)
	}

	plaintext, err := sc.decrypt(payload.Data)
	if err != nil {
		return nil, 0, fmt.Errorf("sync decrypt: %w", err)
	}

	return plaintext, payload.Version, nil
}

// GetManifest fetches the remote manifest to determine what needs syncing.
func (sc *SyncClient) GetManifest(ctx context.Context) (*SyncManifest, error) {
	url := fmt.Sprintf("%s/api/sync/status", sc.baseURL)
	req, err := jsonSafeRequest(ctx, http.MethodGet, url, nil, 15*time.Second)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+sc.apiKey)

	resp, err := doSafeRequest(req, 15*time.Second)
	if err != nil {
		return nil, fmt.Errorf("sync status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sync status: status %d", resp.StatusCode)
	}

	var manifest SyncManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func (sc *SyncClient) encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(sc.encryptKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func (sc *SyncClient) decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(sc.encryptKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	return gcm.Open(nil, ciphertext[:nonceSize], ciphertext[nonceSize:], nil)
}

// SyncScheduler runs periodic background sync.
type SyncScheduler struct {
	client   *SyncClient
	interval time.Duration
	getItems func() []SyncPayload // callback to get local data items
}

func NewSyncScheduler(client *SyncClient, interval time.Duration, getItems func() []SyncPayload) *SyncScheduler {
	return &SyncScheduler{
		client:   client,
		interval: interval,
		getItems: getItems,
	}
}

func (s *SyncScheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.syncOnce(ctx); err != nil {
				slog.Warn("sync: push failed", "err", err)
			}
		}
	}
}

func (s *SyncScheduler) syncOnce(ctx context.Context) error {
	items := s.getItems()
	for _, item := range items {
		if err := s.client.Push(ctx, item.Key, item.Data, item.Version); err != nil {
			return fmt.Errorf("push %s: %w", item.Key, err)
		}
	}
	slog.Debug("sync: pushed items", "count", len(items))
	return nil
}
