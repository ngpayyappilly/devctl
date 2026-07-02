package auth_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"devctl/internal/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Session tests ---

func TestSession_IsExpired_ZeroNeverExpires(t *testing.T) {
	s := &auth.Session{}
	assert.False(t, s.IsExpired())
}

func TestSession_IsExpired_Future(t *testing.T) {
	s := &auth.Session{ExpiresAt: time.Now().Add(time.Hour)}
	assert.False(t, s.IsExpired())
}

func TestSession_IsExpired_Past(t *testing.T) {
	s := &auth.Session{ExpiresAt: time.Now().Add(-time.Second)}
	assert.True(t, s.IsExpired())
}

func TestSession_ExpiresIn_ZeroIsZero(t *testing.T) {
	s := &auth.Session{}
	assert.Equal(t, time.Duration(0), s.ExpiresIn())
}

func TestSession_ExpiresIn_AlreadyExpired(t *testing.T) {
	s := &auth.Session{ExpiresAt: time.Now().Add(-time.Second)}
	assert.Equal(t, time.Duration(0), s.ExpiresIn())
}

func TestSession_ExpiresIn_Future(t *testing.T) {
	s := &auth.Session{ExpiresAt: time.Now().Add(time.Hour)}
	d := s.ExpiresIn()
	assert.Greater(t, d, time.Duration(0))
	assert.LessOrEqual(t, d, time.Hour)
}

// --- File-based TokenStore ---
// fileTokenStore is a test-only TokenStore that stores JSON files in a temp
// directory. It exercises the same logic as KeychainStore's file fallback path.

type fileTokenStore struct{ dir string }

func newFileStore(t *testing.T) auth.TokenStore {
	t.Helper()
	return &fileTokenStore{dir: t.TempDir()}
}

func (f *fileTokenStore) Save(provider string, s *auth.Session) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(f.dir, provider+".json"), data, 0o600)
}

func (f *fileTokenStore) Load(provider string) (*auth.Session, error) {
	data, err := os.ReadFile(filepath.Join(f.dir, provider+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, auth.ErrNotAuthenticated
		}
		return nil, err
	}
	var s auth.Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (f *fileTokenStore) Delete(provider string) error {
	err := os.Remove(filepath.Join(f.dir, provider+".json"))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func TestFileStore_SaveAndLoad(t *testing.T) {
	store := newFileStore(t)
	s := &auth.Session{
		Provider:    "test",
		AccessToken: "tok-abc",
		Username:    "alice",
		ExpiresAt:   time.Now().Add(time.Hour).Truncate(time.Second),
	}

	require.NoError(t, store.Save("test", s))

	got, err := store.Load("test")
	require.NoError(t, err)
	assert.Equal(t, s.Provider, got.Provider)
	assert.Equal(t, s.AccessToken, got.AccessToken)
	assert.Equal(t, s.Username, got.Username)
	assert.Equal(t, s.ExpiresAt.UTC(), got.ExpiresAt.UTC())
}

func TestFileStore_LoadMissing(t *testing.T) {
	store := newFileStore(t)
	_, err := store.Load("nonexistent")
	assert.ErrorIs(t, err, auth.ErrNotAuthenticated)
}

func TestFileStore_Delete(t *testing.T) {
	store := newFileStore(t)
	s := &auth.Session{Provider: "test", AccessToken: "tok"}

	require.NoError(t, store.Save("test", s))
	require.NoError(t, store.Delete("test"))

	_, err := store.Load("test")
	assert.ErrorIs(t, err, auth.ErrNotAuthenticated)
}

func TestFileStore_DeleteMissingIsNoError(t *testing.T) {
	store := newFileStore(t)
	assert.NoError(t, store.Delete("does-not-exist"))
}

func TestFileStore_PermissionsAre0600(t *testing.T) {
	f := newFileStore(t).(*fileTokenStore)
	require.NoError(t, f.Save("test", &auth.Session{AccessToken: "x"}))

	info, err := os.Stat(filepath.Join(f.dir, "test.json"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
