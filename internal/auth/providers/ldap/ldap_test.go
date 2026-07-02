package ldap_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"strings"
	"testing"
	"time"

	gldap "github.com/go-ldap/ldap/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"devctl/internal/auth"
	ldapprovider "devctl/internal/auth/providers/ldap"
)

// mockConn implements ldapprovider.Conn with a response queue for Bind.
type mockConn struct {
	bindCalls     []struct{ user, pass string }
	bindResponses []error // consumed FIFO; nil is treated as success
	searchFn      func(*gldap.SearchRequest) (*gldap.SearchResult, error)
}

func (m *mockConn) Bind(user, pass string) error {
	m.bindCalls = append(m.bindCalls, struct{ user, pass string }{user, pass})
	if len(m.bindResponses) == 0 {
		return nil
	}
	resp := m.bindResponses[0]
	m.bindResponses = m.bindResponses[1:]
	return resp
}

func (m *mockConn) Search(req *gldap.SearchRequest) (*gldap.SearchResult, error) {
	if m.searchFn != nil {
		return m.searchFn(req)
	}
	return &gldap.SearchResult{
		Entries: []*gldap.Entry{
			{DN: "CN=John,DC=corp,DC=example,DC=com"},
		},
	}, nil
}

func (m *mockConn) Close() error { return nil }

func dialFn(conn ldapprovider.Conn, err error) func(string, int, bool, *tls.Config) (ldapprovider.Conn, error) {
	return func(_ string, _ int, _ bool, _ *tls.Config) (ldapprovider.Conn, error) {
		return conn, err
	}
}

func promptFn(username, password string) func(string) (string, error) {
	return func(prompt string) (string, error) {
		if strings.Contains(strings.ToLower(prompt), "password") {
			return password, nil
		}
		return username, nil
	}
}

func TestName(t *testing.T) {
	p := ldapprovider.New(ldapprovider.Config{Host: "ldap.example.com", BaseDN: "DC=example,DC=com"})
	assert.Equal(t, "ldap", p.Name())
}

func TestLogin_Success(t *testing.T) {
	conn := &mockConn{}
	p := ldapprovider.New(ldapprovider.Config{
		Host:     "ldap.corp.example.com",
		BaseDN:   "DC=corp,DC=example,DC=com",
		BindDN:   "CN=svc,DC=corp,DC=example,DC=com",
		BindPass: "svcpass",
		UseTLS:   true,
		Dial:     dialFn(conn, nil),
		PromptFn: promptFn("john", "secret"),
	})

	s, err := p.Login(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ldap", s.Provider)
	assert.Equal(t, "john", s.Username)
	assert.False(t, s.IsExpired())
	assert.WithinDuration(t, time.Now().Add(8*time.Hour), s.ExpiresAt, 5*time.Second)

	// bind[0] = service-account, bind[1] = user DN
	require.Len(t, conn.bindCalls, 2)
	assert.Equal(t, "CN=svc,DC=corp,DC=example,DC=com", conn.bindCalls[0].user)
	assert.Equal(t, "svcpass", conn.bindCalls[0].pass)
	assert.Equal(t, "CN=John,DC=corp,DC=example,DC=com", conn.bindCalls[1].user)
	assert.Equal(t, "secret", conn.bindCalls[1].pass)
}

func TestLogin_NoServiceBind(t *testing.T) {
	conn := &mockConn{}
	p := ldapprovider.New(ldapprovider.Config{
		Host:     "ldap.corp.example.com",
		BaseDN:   "DC=corp,DC=example,DC=com",
		UseTLS:   true,
		Dial:     dialFn(conn, nil),
		PromptFn: promptFn("alice", "pass"),
	})

	s, err := p.Login(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "alice", s.Username)
	assert.Len(t, conn.bindCalls, 1) // only user DN bind
}

func TestLogin_UserNotFound(t *testing.T) {
	conn := &mockConn{
		searchFn: func(_ *gldap.SearchRequest) (*gldap.SearchResult, error) {
			return &gldap.SearchResult{}, nil
		},
	}
	p := ldapprovider.New(ldapprovider.Config{
		Host:     "ldap.corp.example.com",
		BaseDN:   "DC=corp,DC=example,DC=com",
		UseTLS:   true,
		Dial:     dialFn(conn, nil),
		PromptFn: promptFn("ghost", "pass"),
	})

	_, err := p.Login(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLogin_WrongPassword(t *testing.T) {
	conn := &mockConn{
		bindResponses: []error{nil, errors.New("LDAP Result Code 49 \"Invalid Credentials\"")},
	}
	p := ldapprovider.New(ldapprovider.Config{
		Host:     "ldap.corp.example.com",
		BaseDN:   "DC=corp,DC=example,DC=com",
		BindDN:   "CN=svc,DC=corp,DC=example,DC=com",
		BindPass: "svcpass",
		UseTLS:   true,
		Dial:     dialFn(conn, nil),
		PromptFn: promptFn("john", "wrong"),
	})

	_, err := p.Login(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication failed")
}

func TestLogin_ServiceBindError(t *testing.T) {
	conn := &mockConn{
		bindResponses: []error{errors.New("service account locked")},
	}
	p := ldapprovider.New(ldapprovider.Config{
		Host:     "ldap.corp.example.com",
		BaseDN:   "DC=corp,DC=example,DC=com",
		BindDN:   "CN=svc,DC=corp,DC=example,DC=com",
		BindPass: "pass",
		UseTLS:   true,
		Dial:     dialFn(conn, nil),
		PromptFn: promptFn("john", "pass"),
	})

	_, err := p.Login(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service-account bind")
}

func TestLogin_DialError(t *testing.T) {
	p := ldapprovider.New(ldapprovider.Config{
		Host:     "ldap.corp.example.com",
		BaseDN:   "DC=corp,DC=example,DC=com",
		UseTLS:   true,
		Dial:     dialFn(nil, errors.New("connection refused")),
		PromptFn: promptFn("john", "pass"),
	})

	_, err := p.Login(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connect to")
}

func TestLogin_SearchError(t *testing.T) {
	conn := &mockConn{
		searchFn: func(_ *gldap.SearchRequest) (*gldap.SearchResult, error) {
			return nil, errors.New("LDAP search timed out")
		},
	}
	p := ldapprovider.New(ldapprovider.Config{
		Host:     "ldap.corp.example.com",
		BaseDN:   "DC=corp,DC=example,DC=com",
		UseTLS:   true,
		Dial:     dialFn(conn, nil),
		PromptFn: promptFn("john", "pass"),
	})

	_, err := p.Login(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user search")
}

func TestLogin_TLSWarningOnNonLocalhost(t *testing.T) {
	var stderr bytes.Buffer
	conn := &mockConn{}
	p := ldapprovider.New(ldapprovider.Config{
		Host:     "ldap.corp.example.com",
		BaseDN:   "DC=corp,DC=example,DC=com",
		UseTLS:   false, // plaintext on non-localhost → warning
		Dial:     dialFn(conn, nil),
		PromptFn: promptFn("john", "pass"),
		Stderr:   &stderr,
	})

	_, _ = p.Login(context.Background())
	assert.Contains(t, stderr.String(), "WARNING")
	assert.Contains(t, stderr.String(), "plaintext")
}

func TestLogin_NoTLSWarningOnLocalhost(t *testing.T) {
	var stderr bytes.Buffer
	conn := &mockConn{}
	p := ldapprovider.New(ldapprovider.Config{
		Host:     "localhost",
		BaseDN:   "DC=corp,DC=example,DC=com",
		UseTLS:   false,
		Dial:     dialFn(conn, nil),
		PromptFn: promptFn("dev", "pass"),
		Stderr:   &stderr,
	})

	_, _ = p.Login(context.Background())
	assert.Empty(t, stderr.String())
}

func TestRefresh_ReturnsErrRefreshNotSupported(t *testing.T) {
	p := ldapprovider.New(ldapprovider.Config{Host: "ldap.example.com", BaseDN: "DC=example,DC=com"})
	_, err := p.Refresh(context.Background(), &auth.Session{})
	assert.ErrorIs(t, err, auth.ErrRefreshNotSupported)
}

func TestRevoke_IsNoOp(t *testing.T) {
	p := ldapprovider.New(ldapprovider.Config{Host: "ldap.example.com", BaseDN: "DC=example,DC=com"})
	assert.NoError(t, p.Revoke(context.Background(), &auth.Session{}))
}
