package ldap

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	gldap "github.com/go-ldap/ldap/v3"
	"golang.org/x/term"

	"devctl/internal/auth"
)

const (
	name              = "ldap"
	defaultPort636    = 636
	defaultPort389    = 389
	defaultUserFilter = "(&(objectClass=user)(sAMAccountName=%s))"
	sessionTTL        = 8 * time.Hour
)

// Conn is the subset of *gldap.Conn that Provider uses, extracted for testing.
type Conn interface {
	Bind(username, password string) error
	Search(*gldap.SearchRequest) (*gldap.SearchResult, error)
	Close() error
}

// Config holds LDAP connection and directory search settings.
type Config struct {
	Host       string // e.g. "ldap.corp.example.com"
	Port       int    // 389 or 636; inferred from UseTLS when zero
	UseTLS     bool   // true → LDAPS (port 636); false → StartTLS on plain conn
	BaseDN     string // e.g. "DC=corp,DC=example,DC=com"
	BindDN     string // optional service-account DN for user search
	BindPass   string // falls back to DEVCTL_LDAP_BIND_PASSWORD env var if empty
	UserFilter string // default: (&(objectClass=user)(sAMAccountName=%s))

	// Injectable for testing.
	Dial     func(host string, port int, useTLS bool, cfg *tls.Config) (Conn, error)
	PromptFn func(prompt string) (string, error)
	Stderr   io.Writer
}

// Provider implements auth.Provider for LDAP / Active Directory.
type Provider struct {
	cfg Config
}

// New returns an LDAP provider with defaults applied.
func New(cfg Config) *Provider {
	if cfg.UserFilter == "" {
		cfg.UserFilter = defaultUserFilter
	}
	if cfg.Port == 0 {
		if cfg.UseTLS {
			cfg.Port = defaultPort636
		} else {
			cfg.Port = defaultPort389
		}
	}
	if cfg.Dial == nil {
		cfg.Dial = realDial
	}
	if cfg.PromptFn == nil {
		cfg.PromptFn = termReadLine
	}
	if cfg.Stderr == nil {
		cfg.Stderr = os.Stderr
	}
	return &Provider{cfg: cfg}
}

func (p *Provider) Name() string { return name }

func (p *Provider) Login(ctx context.Context) (*auth.Session, error) {
	if !p.cfg.UseTLS && !isLocalhost(p.cfg.Host) {
		fmt.Fprintf(p.cfg.Stderr,
			"WARNING: LDAP TLS is disabled for non-localhost host %q — "+
				"credentials will be transmitted in plaintext. "+
				"Set use_tls: true in config.\n", p.cfg.Host)
	}

	username, err := p.cfg.PromptFn("LDAP username: ")
	if err != nil {
		return nil, fmt.Errorf("read username: %w", err)
	}
	username = strings.TrimSpace(username)

	password, err := p.cfg.PromptFn("Password: ")
	if err != nil {
		return nil, fmt.Errorf("read password: %w", err)
	}

	tlsCfg := &tls.Config{ServerName: p.cfg.Host, MinVersion: tls.VersionTLS12}
	conn, err := p.cfg.Dial(p.cfg.Host, p.cfg.Port, p.cfg.UseTLS, tlsCfg)
	if err != nil {
		return nil, fmt.Errorf("connect to %s:%d: %w", p.cfg.Host, p.cfg.Port, err)
	}
	defer conn.Close()

	// Optional service-account bind for directory search.
	if p.cfg.BindDN != "" {
		bindPass := p.cfg.BindPass
		if bindPass == "" {
			bindPass = os.Getenv("DEVCTL_LDAP_BIND_PASSWORD")
		}
		if err := conn.Bind(p.cfg.BindDN, bindPass); err != nil {
			return nil, fmt.Errorf("service-account bind: %w", err)
		}
	}

	// Find the user's distinguished name.
	filter := fmt.Sprintf(p.cfg.UserFilter, gldap.EscapeFilter(username))
	sr, err := conn.Search(gldap.NewSearchRequest(
		p.cfg.BaseDN,
		gldap.ScopeWholeSubtree, gldap.NeverDerefAliases,
		1, 0, false,
		filter,
		[]string{"dn"},
		nil,
	))
	if err != nil {
		return nil, fmt.Errorf("user search: %w", err)
	}
	if len(sr.Entries) == 0 {
		return nil, fmt.Errorf("user %q not found in directory", username)
	}
	userDN := sr.Entries[0].DN

	// Validate credentials via user bind.
	if err := conn.Bind(userDN, password); err != nil {
		return nil, fmt.Errorf("authentication failed: invalid credentials")
	}

	return &auth.Session{
		Provider:  name,
		Username:  username,
		ExpiresAt: time.Now().Add(sessionTTL),
	}, nil
}

func (p *Provider) Refresh(_ context.Context, _ *auth.Session) (*auth.Session, error) {
	return nil, auth.ErrRefreshNotSupported
}

func (p *Provider) Revoke(_ context.Context, _ *auth.Session) error {
	return nil
}

func isLocalhost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func realDial(host string, port int, useTLS bool, cfg *tls.Config) (Conn, error) {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	if useTLS {
		return gldap.DialTLS("tcp", addr, cfg)
	}
	conn, err := gldap.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	if err := conn.StartTLS(cfg); err != nil {
		conn.Close()
		return nil, fmt.Errorf("StartTLS: %w", err)
	}
	return conn, nil
}

func termReadLine(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	if strings.Contains(strings.ToLower(prompt), "password") {
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	return strings.TrimRight(line, "\r\n"), err
}
