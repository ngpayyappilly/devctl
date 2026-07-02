package auth

import "time"

// Session holds the authentication state for a single provider login.
type Session struct {
	Provider     string            `json:"provider"`
	AccessToken  string            `json:"access_token"`
	RefreshToken string            `json:"refresh_token,omitempty"`
	IDToken      string            `json:"id_token,omitempty"`
	ExpiresAt    time.Time         `json:"expires_at"`
	Username     string            `json:"username"`
	Extra        map[string]string `json:"extra,omitempty"`
}

// IsExpired reports whether the session has passed its expiry time.
// A zero ExpiresAt means the token never expires (e.g. API keys).
func (s *Session) IsExpired() bool {
	if s.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(s.ExpiresAt)
}

// ExpiresIn returns the duration until the session expires.
// Returns 0 for sessions that never expire or have already expired.
func (s *Session) ExpiresIn() time.Duration {
	if s.ExpiresAt.IsZero() {
		return 0
	}
	d := time.Until(s.ExpiresAt)
	if d < 0 {
		return 0
	}
	return d
}
