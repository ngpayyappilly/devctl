package authcmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"devctl/internal/auth"
	"devctl/internal/auth/providers/apikey"
	ldapprovider "devctl/internal/auth/providers/ldap"
	"devctl/internal/auth/providers/oidc"
	"devctl/internal/auth/providers/okta"
	"devctl/internal/auth/providers/ping"
	"devctl/pkg/config"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage devctl authentication",
	}
	cmd.AddCommand(loginCmd())
	cmd.AddCommand(logoutCmd())
	cmd.AddCommand(statusCmd())
	cmd.AddCommand(tokenCmd())
	return cmd
}

func loginCmd() *cobra.Command {
	var providerName string
	var token string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with the configured identity provider",
		RunE: func(cmd *cobra.Command, args []string) error {
			if providerName == "" {
				providerName = config.GetString(config.KeyAuthProvider, "")
			}
			if providerName == "" {
				return fmt.Errorf("no auth provider configured; set auth.provider in config or use --provider")
			}

			// Certain providers are built from config at login time rather
			// than pre-registered, because they require per-org config values.
			var p auth.Provider
			switch providerName {
			case "apikey":
				p = apikey.New(token)
			case "oidc":
				issuer := config.GetString(config.KeyOIDCIssuerURL, "")
				clientID := config.GetString(config.KeyOIDCClientID, "")
				if issuer == "" || clientID == "" {
					return fmt.Errorf("oidc provider requires auth.oidc.issuer_url and auth.oidc.client_id in config")
				}
				p = oidc.New(oidc.Config{
					IssuerURL:    issuer,
					ClientID:     clientID,
					ClientSecret: config.GetString(config.KeyOIDCClientSecret, ""),
				})
			case "okta":
				domain := config.GetString(config.KeyOktaDomain, "")
				clientID := config.GetString(config.KeyOktaClientID, "")
				if domain == "" || clientID == "" {
					return fmt.Errorf("okta provider requires auth.okta.domain and auth.okta.client_id in config")
				}
				p = okta.New(okta.Config{
					Domain:       domain,
					ClientID:     clientID,
					ClientSecret: config.GetString(config.KeyOktaClientSecret, ""),
				})
			case "ping":
				issuer := config.GetString(config.KeyPingIssuerURL, "")
				clientID := config.GetString(config.KeyPingClientID, "")
				if issuer == "" || clientID == "" {
					return fmt.Errorf("ping provider requires auth.ping.issuer_url and auth.ping.client_id in config")
				}
				p = ping.New(ping.Config{
					IssuerURL:    issuer,
					ClientID:     clientID,
					ClientSecret: config.GetString(config.KeyPingClientSecret, ""),
				})
			case "ldap":
				host := config.GetString(config.KeyLDAPHost, "")
				baseDN := config.GetString(config.KeyLDAPBaseDN, "")
				if host == "" || baseDN == "" {
					return fmt.Errorf("ldap provider requires auth.ldap.host and auth.ldap.base_dn in config")
				}
				p = ldapprovider.New(ldapprovider.Config{
					Host:       host,
					Port:       viper.GetInt(config.KeyLDAPPort),
					UseTLS:     viper.GetBool(config.KeyLDAPUseTLS),
					BaseDN:     baseDN,
					BindDN:     config.GetString(config.KeyLDAPBindDN, ""),
					UserFilter: config.GetString(config.KeyLDAPUserFilter, ""),
				})
			default:
				var err error
				p, err = auth.Lookup(providerName)
				if err != nil {
					return err
				}
			}

			session, err := p.Login(context.Background())
			if err != nil {
				return fmt.Errorf("login failed: %w", err)
			}

			if err := auth.DefaultStore.Save(providerName, session); err != nil {
				return fmt.Errorf("save session: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Logged in as %s via %s\n", session.Username, providerName)
			return nil
		},
	}
	cmd.Flags().StringVar(&providerName, "provider", "", "Auth provider to use (overrides auth.provider config)")
	cmd.Flags().StringVar(&token, "token", "", "Static API token (apikey provider only; overrides DEVCTL_TOKEN)")
	return cmd
}

func logoutCmd() *cobra.Command {
	var providerName string

	return &cobra.Command{
		Use:   "logout",
		Short: "Revoke and remove the current session",
		RunE: func(cmd *cobra.Command, args []string) error {
			if providerName == "" {
				providerName = config.GetString(config.KeyAuthProvider, "")
			}
			if providerName == "" {
				return fmt.Errorf("no auth provider configured; set auth.provider in config or use --provider")
			}

			session, err := auth.DefaultStore.Load(providerName)
			if err != nil && !errors.Is(err, auth.ErrNotAuthenticated) {
				return err
			}

			if session != nil {
				p, err := auth.Lookup(providerName)
				if err == nil {
					_ = p.Revoke(context.Background(), session) // best-effort
				}
			}

			if err := auth.DefaultStore.Delete(providerName); err != nil {
				return fmt.Errorf("delete session: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Logged out from %s\n", providerName)
			return nil
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			providerName := config.GetString(config.KeyAuthProvider, "")
			if providerName == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "Not configured — set auth.provider in config")
				return nil
			}

			session, err := auth.DefaultStore.Load(providerName)
			if errors.Is(err, auth.ErrNotAuthenticated) {
				fmt.Fprintf(cmd.OutOrStdout(), "Not authenticated (provider: %s)\n", providerName)
				return nil
			}
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Provider : %s\n", session.Provider)
			fmt.Fprintf(out, "Username : %s\n", session.Username)
			if session.ExpiresAt.IsZero() {
				fmt.Fprintf(out, "Expires  : never\n")
			} else if session.IsExpired() {
				fmt.Fprintf(out, "Expires  : EXPIRED (run 'devctl auth login' to re-authenticate)\n")
			} else {
				fmt.Fprintf(out, "Expires  : %s (in %s)\n",
					session.ExpiresAt.Local().Format("2006-01-02 15:04:05"),
					session.ExpiresIn().Round(1e9))
			}
			return nil
		},
	}
}

func tokenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "token",
		Short: "Print the current access token (for scripting)",
		RunE: func(cmd *cobra.Command, args []string) error {
			providerName := config.GetString(config.KeyAuthProvider, "")
			if providerName == "" {
				return fmt.Errorf("no auth provider configured")
			}

			session, err := auth.DefaultStore.Load(providerName)
			if errors.Is(err, auth.ErrNotAuthenticated) {
				fmt.Fprintln(os.Stderr, "Not authenticated — run 'devctl auth login'")
				os.Exit(1)
			}
			if err != nil {
				return err
			}

			if session.IsExpired() {
				fmt.Fprintln(os.Stderr, "Session expired — run 'devctl auth login'")
				os.Exit(1)
			}

			fmt.Fprintln(cmd.OutOrStdout(), session.AccessToken)
			return nil
		},
	}
}
