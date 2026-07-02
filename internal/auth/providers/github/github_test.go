package github_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"devctl/internal/auth"
	"devctl/internal/auth/providers/github"
)

// miniGitHub builds a mock GitHub server. tokenResponses are served in order;
// the last one is repeated. userLogin and userEmail are served from /user.
func miniGitHub(t *testing.T, tokenResponses []map[string]any, userLogin, userEmail string) *httptest.Server {
	t.Helper()
	var tokenIdx int

	mux := http.NewServeMux()
	mux.HandleFunc("/login/device/code", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"device_code":      "dev-code",
			"user_code":        "ABCD-1234",
			"verification_uri": "https://github.com/login/device",
			"expires_in":       900,
			"interval":         0,
		})
	})
	mux.HandleFunc("/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		idx := tokenIdx
		if idx >= len(tokenResponses) {
			idx = len(tokenResponses) - 1
		}
		tokenIdx++
		json.NewEncoder(w).Encode(tokenResponses[idx])
	})
	// /user serves as both github.com API and GHE /api/v3/user (registered separately below)
	userHandler := func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"login": userLogin,
			"email": userEmail,
		})
	}
	mux.HandleFunc("/user", userHandler)
	mux.HandleFunc("/api/v3/user", userHandler) // GHE path

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func successToken() map[string]any {
	return map[string]any{"access_token": "gho_abc123", "token_type": "bearer", "scope": "read:user,user:email"}
}

func TestName(t *testing.T) {
	p := github.New(github.Config{ClientID: "Iv1.abc"})
	assert.Equal(t, "github", p.Name())
}

func TestLogin_Success(t *testing.T) {
	srv := miniGitHub(t, []map[string]any{successToken()}, "octocat", "octocat@github.com")
	p := github.New(github.Config{
		ClientID:     "Iv1.abc",
		BaseURL:      srv.URL,
		HTTPClient:   srv.Client(),
		PollInterval: time.Millisecond,
	})

	s, err := p.Login(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "github", s.Provider)
	assert.Equal(t, "gho_abc123", s.AccessToken)
	assert.Equal(t, "octocat", s.Username)
	assert.Equal(t, "octocat@github.com", s.Extra["email"])
	assert.True(t, s.ExpiresAt.IsZero(), "GitHub tokens should not expire")
}

func TestLogin_NoEmail(t *testing.T) {
	srv := miniGitHub(t, []map[string]any{successToken()}, "ghost", "")
	p := github.New(github.Config{
		ClientID:     "Iv1.abc",
		BaseURL:      srv.URL,
		HTTPClient:   srv.Client(),
		PollInterval: time.Millisecond,
	})

	s, err := p.Login(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ghost", s.Username)
	assert.Nil(t, s.Extra, "Extra should be nil when email is empty")
}

func TestLogin_AuthorizationPending(t *testing.T) {
	responses := []map[string]any{
		{"error": "authorization_pending", "error_description": "still waiting"},
		{"error": "authorization_pending", "error_description": "still waiting"},
		successToken(),
	}
	srv := miniGitHub(t, responses, "octocat", "")
	p := github.New(github.Config{
		ClientID:     "Iv1.abc",
		BaseURL:      srv.URL,
		HTTPClient:   srv.Client(),
		PollInterval: time.Millisecond,
	})

	s, err := p.Login(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "gho_abc123", s.AccessToken)
}

func TestLogin_SlowDown(t *testing.T) {
	responses := []map[string]any{
		{"error": "slow_down", "error_description": "too fast", "interval": 10},
		successToken(),
	}
	srv := miniGitHub(t, responses, "octocat", "")
	p := github.New(github.Config{
		ClientID:     "Iv1.abc",
		BaseURL:      srv.URL,
		HTTPClient:   srv.Client(),
		PollInterval: time.Millisecond, // prevents 10s sleep in test
	})

	s, err := p.Login(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "gho_abc123", s.AccessToken)
}

func TestLogin_ExpiredToken(t *testing.T) {
	srv := miniGitHub(t, []map[string]any{
		{"error": "expired_token", "error_description": "The device code has expired."},
	}, "", "")
	p := github.New(github.Config{
		ClientID:     "Iv1.abc",
		BaseURL:      srv.URL,
		HTTPClient:   srv.Client(),
		PollInterval: time.Millisecond,
	})

	_, err := p.Login(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestLogin_AccessDenied(t *testing.T) {
	srv := miniGitHub(t, []map[string]any{
		{"error": "access_denied", "error_description": "The user denied access."},
	}, "", "")
	p := github.New(github.Config{
		ClientID:     "Iv1.abc",
		BaseURL:      srv.URL,
		HTTPClient:   srv.Client(),
		PollInterval: time.Millisecond,
	})

	_, err := p.Login(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "denied")
}

func TestLogin_GHEBaseURL(t *testing.T) {
	// GHE: api lives at <BaseURL>/api/v3/user, not at api.github.com
	var apiPath string
	srv := miniGitHub(t, []map[string]any{successToken()}, "gheuser", "gheuser@corp.com")
	// Wrap to capture which user path was hit.
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/user" {
			apiPath = r.URL.Path
		}
		srv.Config.Handler.ServeHTTP(w, r)
	})
	ghe := httptest.NewServer(wrapped)
	t.Cleanup(ghe.Close)

	p := github.New(github.Config{
		ClientID:     "Iv1.abc",
		BaseURL:      ghe.URL, // non-default → GHE mode
		HTTPClient:   ghe.Client(),
		PollInterval: time.Millisecond,
	})

	s, err := p.Login(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "gheuser", s.Username)
	assert.Equal(t, "/api/v3/user", apiPath, "GHE should use /api/v3/user path")
}

func TestLogin_DefaultScopes(t *testing.T) {
	var gotScope string
	mux := http.NewServeMux()
	mux.HandleFunc("/login/device/code", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if s, ok := body["scope"].(string); ok {
			gotScope = s
		}
		json.NewEncoder(w).Encode(map[string]any{
			"device_code": "dc", "user_code": "X",
			"verification_uri": "http://x", "expires_in": 900, "interval": 0,
		})
	})
	mux.HandleFunc("/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(successToken())
	})
	userHandler := func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"login": "u", "email": ""})
	}
	mux.HandleFunc("/user", userHandler)
	mux.HandleFunc("/api/v3/user", userHandler) // hit when BaseURL != github.com
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	p := github.New(github.Config{
		ClientID:     "Iv1.abc",
		BaseURL:      srv.URL,
		HTTPClient:   srv.Client(),
		PollInterval: time.Millisecond,
		// Scopes intentionally omitted → defaults applied
	})
	_, err := p.Login(context.Background())
	require.NoError(t, err)
	assert.Contains(t, gotScope, "read:user")
	assert.Contains(t, gotScope, "user:email")
}

func TestRefresh_ReturnsErrRefreshNotSupported(t *testing.T) {
	p := github.New(github.Config{ClientID: "Iv1.abc"})
	_, err := p.Refresh(context.Background(), &auth.Session{})
	assert.ErrorIs(t, err, auth.ErrRefreshNotSupported)
}

func TestRevoke_IsNoOp(t *testing.T) {
	p := github.New(github.Config{ClientID: "Iv1.abc"})
	assert.NoError(t, p.Revoke(context.Background(), &auth.Session{}))
}
