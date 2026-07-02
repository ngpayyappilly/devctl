package auth

import (
	"context"
	"errors"
)

// ErrRefreshNotSupported is returned by providers that don't support refresh.
var ErrRefreshNotSupported = errors.New("refresh not supported by this provider")

// ErrNotAuthenticated is returned when no session exists for the provider.
var ErrNotAuthenticated = errors.New("not authenticated; run 'devctl auth login'")

// Provider is the interface every auth backend must implement.
type Provider interface {
	// Name returns the unique string identifier (e.g. "okta", "ldap", "apikey").
	Name() string
	// Login performs authentication and returns a new Session.
	Login(ctx context.Context) (*Session, error)
	// Refresh attempts to renew a session without user interaction.
	Refresh(ctx context.Context, s *Session) (*Session, error)
	// Revoke invalidates the session server-side (best-effort).
	Revoke(ctx context.Context, s *Session) error
}

// TokenStore persists and retrieves sessions.
type TokenStore interface {
	Save(provider string, s *Session) error
	Load(provider string) (*Session, error)
	Delete(provider string) error
}
