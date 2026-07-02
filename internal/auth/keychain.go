package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

const keychainService = "devctl"

// KeychainStore saves sessions to the OS keychain (macOS Keychain, Linux
// Secret Service, Windows DPAPI). When the keychain is unavailable (CI,
// headless, no daemon), it falls back to a JSON file at
// ~/.devctl/sessions/<provider>.json (mode 0600).
type KeychainStore struct{}

// DefaultStore is the TokenStore used by authcmd commands.
var DefaultStore TokenStore = &KeychainStore{}

func (ks *KeychainStore) Save(provider string, s *Session) error {
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	if err := keyring.Set(keychainService, provider, string(data)); err != nil {
		return ks.saveToFile(provider, data)
	}
	return nil
}

func (ks *KeychainStore) Load(provider string) (*Session, error) {
	raw, err := keyring.Get(keychainService, provider)
	if err != nil {
		return ks.loadFromFile(provider)
	}
	var s Session
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	return &s, nil
}

func (ks *KeychainStore) Delete(provider string) error {
	// best-effort on both paths; always attempt to clean up the file too
	_ = keyring.Delete(keychainService, provider)
	_ = ks.deleteFile(provider)
	return nil
}

// --- file fallback ---

func sessionFilePath(provider string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".devctl", "sessions", provider+".json"), nil
}

func (ks *KeychainStore) saveToFile(provider string, data []byte) error {
	path, err := sessionFilePath(provider)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create sessions dir: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

func (ks *KeychainStore) loadFromFile(provider string) (*Session, error) {
	path, err := sessionFilePath(provider)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotAuthenticated
		}
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("unmarshal session from file: %w", err)
	}
	return &s, nil
}

func (ks *KeychainStore) deleteFile(provider string) error {
	path, err := sessionFilePath(provider)
	if err != nil {
		return err
	}
	if err := os.Remove(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else {
		return err
	}
}
