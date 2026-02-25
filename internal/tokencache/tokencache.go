package tokencache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/99designs/keyring"
	"github.com/charmbracelet/log"
)

const (
	serviceName = "eightctl"
	tokenKey    = "oauth-token"
)

type CachedToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	UserID    string    `json:"user_id,omitempty"`
}

// Identity describes the authentication context a token belongs to.
// Tokens are namespaced by base URL, client ID, and email so switching
// between accounts or environments doesn't reuse the wrong credentials.
type Identity struct {
	BaseURL  string
	ClientID string
	Email    string
}

var openKeyring = defaultOpenKeyring
var fallbackCachePathFunc = defaultFallbackCachePath

// SetOpenKeyringForTest swaps the keyring opener; it returns a restore func.
// Not safe for concurrent tests; intended for isolated test scenarios.
func SetOpenKeyringForTest(fn func() (keyring.Keyring, error)) (restore func()) {
	prev := openKeyring
	openKeyring = fn
	return func() { openKeyring = prev }
}

// SetFallbackPathForTest overrides the fallback token file path for tests.
func SetFallbackPathForTest(path string) (restore func()) {
	prev := fallbackCachePathFunc
	fallbackCachePathFunc = func() string { return path }
	return func() { fallbackCachePathFunc = prev }
}

func defaultOpenKeyring() (keyring.Keyring, error) {
	home, _ := os.UserHomeDir()
	return keyring.Open(keyring.Config{
		ServiceName: serviceName,
		AllowedBackends: []keyring.BackendType{
			keyring.FileBackend,
		},
		FileDir:          filepath.Join(home, ".config", "eightctl", "keyring"),
		FilePasswordFunc: filePassword,
	})
}

func filePassword(_ string) (string, error) {
	return serviceName + "-fallback", nil
}

func Save(id Identity, token string, expiresAt time.Time, userID string) error {
	cached := CachedToken{
		Token:     token,
		ExpiresAt: expiresAt,
		UserID:    userID,
	}
	data, err := json.Marshal(cached)
	if err != nil {
		return err
	}

	if ring, err := openKeyring(); err == nil {
		if err := ring.Set(keyring.Item{
			Key:   cacheKey(id),
			Label: serviceName + " token",
			Data:  data,
		}); err == nil {
			log.Debug("keyring saved token")
			return nil
		} else {
			log.Debug("keyring set failed", "error", err)
		}
	} else {
		log.Debug("keyring open failed (save)", "error", err)
	}

	if err := saveFallbackToken(cacheKey(id), cached); err != nil {
		return err
	}
	log.Debug("saved token to file fallback", "path", fallbackCachePath())
	return nil
}

func Load(id Identity, expectedUserID string) (*CachedToken, error) {
	if ring, err := openKeyring(); err == nil {
		key := cacheKey(id)
		item, err := ring.Get(key)
		if err == keyring.ErrKeyNotFound && id.Email == "" {
			// No email specified: attempt to find a single matching token for this base/client.
			if alt, findErr := findSingleForClient(ring, id); findErr == nil {
				key = alt
				item, err = ring.Get(key)
			} else {
				log.Debug("keyring wildcard lookup failed", "error", findErr)
			}
		}
		if err == nil {
			cached, parseErr := parseCachedToken(item.Data)
			if parseErr != nil {
				return nil, parseErr
			}
			if time.Now().After(cached.ExpiresAt) {
				_ = ring.Remove(key)
				return nil, keyring.ErrKeyNotFound
			}
			if expectedUserID != "" && cached.UserID != "" && cached.UserID != expectedUserID {
				return nil, keyring.ErrKeyNotFound
			}
			return cached, nil
		}
		log.Debug("keyring get failed", "error", err)
	} else {
		log.Debug("keyring open failed (load)", "error", err)
	}

	cached, err := loadFallbackToken(id, expectedUserID)
	if err != nil {
		return nil, err
	}
	return cached, nil
}

func Clear(id Identity) error {
	key := cacheKey(id)
	if ring, err := openKeyring(); err == nil {
		if err := ring.Remove(key); err != nil {
			if err != keyring.ErrKeyNotFound && !os.IsNotExist(err) {
				log.Debug("keyring remove failed", "error", err)
			}
		}
	} else {
		log.Debug("keyring open failed (clear)", "error", err)
	}
	if err := clearFallbackToken(key); err != nil {
		return err
	}
	return nil
}

func cacheKey(id Identity) string {
	base := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(id.BaseURL)), "/")
	email := strings.ToLower(strings.TrimSpace(id.Email))
	return tokenKey + ":" + base + "|" + id.ClientID + "|" + email
}

// findSingleForClient finds a single cached key for the given base/client when email is unknown.
// Returns ErrKeyNotFound if none or multiple exist.
func findSingleForClient(ring keyring.Keyring, id Identity) (string, error) {
	keys, err := ring.Keys()
	if err != nil {
		return "", err
	}
	prefix := tokenKey + ":" + strings.TrimSuffix(strings.ToLower(strings.TrimSpace(id.BaseURL)), "/") + "|" + id.ClientID + "|"
	matches := []string{}
	for _, k := range keys {
		if strings.HasPrefix(k, prefix) {
			matches = append(matches, k)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	return "", keyring.ErrKeyNotFound
}

func parseCachedToken(data []byte) (*CachedToken, error) {
	var cached CachedToken
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, err
	}
	return &cached, nil
}

func fallbackCachePath() string {
	return fallbackCachePathFunc()
}

func defaultFallbackCachePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "eightctl", "token-cache.json")
}

func loadFallbackMap() (map[string]CachedToken, error) {
	path := fallbackCachePath()
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]CachedToken{}, nil
		}
		return nil, err
	}
	out := map[string]CachedToken{}
	if len(b) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func saveFallbackMap(m map[string]CachedToken) error {
	path := fallbackCachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func saveFallbackToken(key string, token CachedToken) error {
	m, err := loadFallbackMap()
	if err != nil {
		return err
	}
	m[key] = token
	return saveFallbackMap(m)
}

func clearFallbackToken(key string) error {
	m, err := loadFallbackMap()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if _, ok := m[key]; !ok {
		return nil
	}
	delete(m, key)
	return saveFallbackMap(m)
}

func loadFallbackToken(id Identity, expectedUserID string) (*CachedToken, error) {
	m, err := loadFallbackMap()
	if err != nil {
		return nil, err
	}
	key := cacheKey(id)
	token, ok := m[key]
	if !ok && id.Email == "" {
		key, err = findSingleForClientInMap(m, id)
		if err != nil {
			return nil, err
		}
		token = m[key]
	}
	if !ok && id.Email != "" {
		return nil, keyring.ErrKeyNotFound
	}
	if time.Now().After(token.ExpiresAt) {
		delete(m, key)
		_ = saveFallbackMap(m)
		return nil, keyring.ErrKeyNotFound
	}
	if expectedUserID != "" && token.UserID != "" && token.UserID != expectedUserID {
		return nil, keyring.ErrKeyNotFound
	}
	return &token, nil
}

func findSingleForClientInMap(m map[string]CachedToken, id Identity) (string, error) {
	prefix := tokenKey + ":" + strings.TrimSuffix(strings.ToLower(strings.TrimSpace(id.BaseURL)), "/") + "|" + id.ClientID + "|"
	matches := []string{}
	for k := range m {
		if strings.HasPrefix(k, prefix) {
			matches = append(matches, k)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	return "", keyring.ErrKeyNotFound
}
