package okta_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"devctl/internal/auth"
	"devctl/internal/auth/providers/okta"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// miniIdP is a minimal OIDC Device Flow server for testing.
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
			"device_code": "dc", "user_code": "XXXX-1234",
			"verification_uri": srvURL + "/activate",
			"expires_in": 300, "interval": 0,
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "okta-acc", "refresh_token": "okta-ref",
			"id_token": "okta-id", "expires_in": 3600,
		})
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"email": "bob@myorg.okta.com"})
	})

	srv := httptest.NewServer(mux)
	srvURL = srv.URL
	t.Cleanup(srv.Close)
	return srv
}

func TestName(t *testing.T) {
	p := okta.New(okta.Config{Domain: "myorg.okta.com", ClientID: "c"})
	assert.Equal(t, "okta", p.Name())
}

func TestLogin_SessionProviderIsOkta(t *testing.T) {
	srv := miniIdP(t)

	p := okta.New(okta.Config{
		Domain:       "myorg.okta.com",
		IssuerURL:    srv.URL, // overrides https://<Domain> for the test server
		ClientID:     "test-client",
		HTTPClient:   srv.Client(),
		PollInterval: time.Millisecond,
	})

	s, err := p.Login(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "okta", s.Provider)
	assert.Equal(t, "okta-acc", s.AccessToken)
	assert.Equal(t, "bob@myorg.okta.com", s.Username)
	assert.False(t, s.IsExpired())
}

func TestLogin_ScopesIncludeGroups(t *testing.T) {
	var gotScopes string
	var srvURL string

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"device_authorization_endpoint": srvURL + "/device/authorize",
			"token_endpoint":                srvURL + "/token",
		})
	})
	mux.HandleFunc("/device/authorize", func(w http.ResponseWriter, r *http.Request) {
		gotScopes = r.FormValue("scope")
		json.NewEncoder(w).Encode(map[string]any{
			"device_code": "dc", "user_code": "X",
			"verification_uri": "http://x", "expires_in": 300, "interval": 0,
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"access_token": "t", "expires_in": 3600})
	})
	srv := httptest.NewServer(mux)
	srvURL = srv.URL
	t.Cleanup(srv.Close)

	p := okta.New(okta.Config{
		Domain:       "myorg.okta.com",
		IssuerURL:    srv.URL,
		ClientID:     "c",
		HTTPClient:   srv.Client(),
		PollInterval: time.Millisecond,
	})
	p.Login(context.Background()) //nolint:errcheck

	assert.Contains(t, gotScopes, "groups", "Okta provider must request groups scope")
	assert.Contains(t, gotScopes, "openid")
	assert.Contains(t, gotScopes, "email")
}

func TestRefresh_DelegatesAndPreservesProvider(t *testing.T) {
	srv := miniIdP(t)
	p := okta.New(okta.Config{
		Domain:       "myorg.okta.com",
		IssuerURL:    srv.URL,
		ClientID:     "c",
		HTTPClient:   srv.Client(),
		PollInterval: time.Millisecond,
	})

	existing := &auth.Session{Provider: "okta", RefreshToken: "old-ref", Username: "bob@myorg.okta.com"}
	s, err := p.Refresh(context.Background(), existing)
	require.NoError(t, err)
	assert.Equal(t, "okta-acc", s.AccessToken)
}

func TestRevoke_IsNoError(t *testing.T) {
	p := okta.New(okta.Config{Domain: "myorg.okta.com", ClientID: "c"})
	assert.NoError(t, p.Revoke(context.Background(), &auth.Session{}))
}
