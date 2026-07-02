package oidc_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"devctl/internal/auth"
	"devctl/internal/auth/providers/oidc"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockIdP spins up an httptest.Server that implements the subset of OIDC
// needed for the Device Authorization Flow.
type mockIdP struct {
	server    *httptest.Server
	pollCount atomic.Int32 // number of token polls seen
	pendingN  int          // number of "authorization_pending" responses before success
}

func newMockIdP(t *testing.T, pendingRounds int) *mockIdP {
	t.Helper()
	m := &mockIdP{pendingN: pendingRounds}

	mux := http.NewServeMux()

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		base := m.server.URL
		json.NewEncoder(w).Encode(map[string]string{
			"issuer":                          base,
			"device_authorization_endpoint":   base + "/device/authorize",
			"token_endpoint":                  base + "/token",
			"revocation_endpoint":             base + "/revoke",
			"userinfo_endpoint":               base + "/userinfo",
		})
	})

	mux.HandleFunc("/device/authorize", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"device_code":      "dev-code-xyz",
			"user_code":        "ABCD-1234",
			"verification_uri": base(m) + "/activate",
			"expires_in":       300,
			"interval":         0, // use 0 so tests don't actually sleep; provider sets default 5s but tests override HTTPClient timing
		})
	})

	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		n := int(m.pollCount.Add(1))
		if n <= m.pendingN {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "acc-tok-123",
			"refresh_token": "ref-tok-456",
			"id_token":      "id-tok-789",
			"expires_in":    3600,
		})
	})

	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer acc-tok-123", r.Header.Get("Authorization"))
		json.NewEncoder(w).Encode(map[string]string{
			"sub":   "user-001",
			"email": "alice@example.com",
			"name":  "Alice",
		})
	})

	mux.HandleFunc("/revoke", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	m.server = httptest.NewServer(mux)
	t.Cleanup(m.server.Close)
	return m
}

func base(m *mockIdP) string { return m.server.URL }

// fastClient returns an http.Client that bypasses the poll interval by
// running immediately (the mock interval is 0, but the provider enforces
// a minimum via time.After — we keep interval small enough for tests).
func fastClient(s *httptest.Server) *http.Client {
	return s.Client()
}

// newProvider builds an OIDCProvider pointed at the mock IdP.
func newProvider(m *mockIdP) *oidc.OIDCProvider {
	return oidc.New(oidc.Config{
		IssuerURL:    m.server.URL,
		ClientID:     "test-client",
		Scopes:       []string{"openid", "email"},
		HTTPClient:   fastClient(m.server),
		PollInterval: time.Millisecond, // skip the 5-second RFC 8628 floor in tests
	})
}

// --- tests ---

func TestLogin_HappyPath(t *testing.T) {
	m := newMockIdP(t, 0) // no pending rounds — approve immediately
	p := newProvider(m)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s, err := p.Login(ctx)
	require.NoError(t, err)

	assert.Equal(t, "oidc", s.Provider)
	assert.Equal(t, "acc-tok-123", s.AccessToken)
	assert.Equal(t, "ref-tok-456", s.RefreshToken)
	assert.Equal(t, "id-tok-789", s.IDToken)
	assert.Equal(t, "alice@example.com", s.Username)
	assert.False(t, s.ExpiresAt.IsZero())
	assert.False(t, s.IsExpired())
}

func TestLogin_PendingThenApproved(t *testing.T) {
	m := newMockIdP(t, 2) // 2 pending rounds before approval
	p := newProvider(m)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s, err := p.Login(ctx)
	require.NoError(t, err)
	assert.Equal(t, "acc-tok-123", s.AccessToken)
	assert.Equal(t, int32(3), m.pollCount.Load()) // 2 pending + 1 success
}

func TestLogin_AccessDenied(t *testing.T) {
	// Build a standalone mini-IdP that returns access_denied on the token endpoint.
	var srvURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"device_authorization_endpoint": srvURL + "/device/authorize",
			"token_endpoint":                srvURL + "/token",
		})
	})
	mux.HandleFunc("/device/authorize", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"device_code": "dc", "user_code": "XXXX",
			"verification_uri": "http://example.com", "expires_in": 60, "interval": 0,
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "access_denied"})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	srvURL = srv.URL

	p := oidc.New(oidc.Config{
		IssuerURL:    srv.URL,
		ClientID:     "c",
		HTTPClient:   srv.Client(),
		PollInterval: time.Millisecond,
	})
	_, err := p.Login(context.Background())
	assert.ErrorContains(t, err, "denied")
}

func TestLogin_DiscoveryMissingDeviceEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Discovery doc without device_authorization_endpoint
		json.NewEncoder(w).Encode(map[string]string{"token_endpoint": "http://example.com/token"})
	}))
	t.Cleanup(srv.Close)

	p := oidc.New(oidc.Config{IssuerURL: srv.URL, ClientID: "c", HTTPClient: srv.Client()})
	_, err := p.Login(context.Background())
	assert.ErrorContains(t, err, "device_authorization_endpoint")
}

func TestRefresh_Success(t *testing.T) {
	m := newMockIdP(t, 0)
	p := newProvider(m)

	existing := &auth.Session{
		Provider:     "oidc",
		AccessToken:  "old-acc",
		RefreshToken: "ref-tok-456",
		Username:     "alice@example.com",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s, err := p.Refresh(ctx, existing)
	require.NoError(t, err)
	assert.Equal(t, "acc-tok-123", s.AccessToken)
	assert.Equal(t, "alice@example.com", s.Username) // preserved from original
}

func TestRefresh_NoRefreshToken(t *testing.T) {
	m := newMockIdP(t, 0)
	p := newProvider(m)

	_, err := p.Refresh(context.Background(), &auth.Session{})
	assert.ErrorIs(t, err, auth.ErrRefreshNotSupported)
}

func TestRevoke_BestEffort(t *testing.T) {
	m := newMockIdP(t, 0)
	p := newProvider(m)

	// Revoke should never return an error, even if IdP returns 500
	err := p.Revoke(context.Background(), &auth.Session{AccessToken: "tok"})
	assert.NoError(t, err)
}

func TestName(t *testing.T) {
	p := oidc.New(oidc.Config{IssuerURL: "https://example.com", ClientID: "c"})
	assert.Equal(t, "oidc", p.Name())
}
