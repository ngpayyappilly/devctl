package ping_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"devctl/internal/auth"
	"devctl/internal/auth/providers/ping"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func miniIdP(t *testing.T) *httptest.Server {
	t.Helper()
	var srvURL string
	mux := http.NewServeMux()

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"device_authorization_endpoint": srvURL + "/device/authorize",
			"token_endpoint":                srvURL + "/token",
			"userinfo_endpoint":             srvURL + "/userinfo",
		})
	})
	mux.HandleFunc("/device/authorize", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"device_code": "dc", "user_code": "PING-5678",
			"verification_uri": srvURL + "/activate",
			"expires_in": 300, "interval": 0,
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "ping-acc", "refresh_token": "ping-ref",
			"id_token": "ping-id", "expires_in": 3600,
		})
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"email": "carol@corp.example.com"})
	})

	srv := httptest.NewServer(mux)
	srvURL = srv.URL
	t.Cleanup(srv.Close)
	return srv
}

func TestName(t *testing.T) {
	p := ping.New(ping.Config{IssuerURL: "https://ping.example.com/as", ClientID: "c"})
	assert.Equal(t, "ping", p.Name())
}

func TestLogin_SessionProviderIsPing(t *testing.T) {
	srv := miniIdP(t)

	p := ping.New(ping.Config{
		IssuerURL:    srv.URL,
		ClientID:     "test-client",
		HTTPClient:   srv.Client(),
		PollInterval: time.Millisecond,
	})

	s, err := p.Login(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "ping", s.Provider)
	assert.Equal(t, "ping-acc", s.AccessToken)
	assert.Equal(t, "carol@corp.example.com", s.Username)
	assert.False(t, s.IsExpired())
}

func TestRefresh_DelegatesAndPreservesProvider(t *testing.T) {
	srv := miniIdP(t)
	p := ping.New(ping.Config{
		IssuerURL:    srv.URL,
		ClientID:     "c",
		HTTPClient:   srv.Client(),
		PollInterval: time.Millisecond,
	})

	existing := &auth.Session{Provider: "ping", RefreshToken: "old-ref", Username: "carol@corp.example.com"}
	s, err := p.Refresh(context.Background(), existing)
	require.NoError(t, err)
	assert.Equal(t, "ping-acc", s.AccessToken)
}

func TestRevoke_IsNoError(t *testing.T) {
	p := ping.New(ping.Config{IssuerURL: "https://ping.example.com/as", ClientID: "c"})
	assert.NoError(t, p.Revoke(context.Background(), &auth.Session{}))
}
