package auth

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/synca/daemon/internal/config"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
	netproxy "golang.org/x/net/proxy"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// googleClientID is the public OAuth client identifier of the installed
// Synca application. Installed applications are public clients: this
// identifier is embedded in every distributed binary, is not confidential,
// and must never be treated as a secret. Development builds may override it
// through GOOGLE_CLIENT_ID in a local .env file.
const googleClientID = "781406235807-pj42r8r51ou6k0pubvaghvs3sfq0p68m.apps.googleusercontent.com"

const (
	localRedirectPort = "9373"
	localRedirectURL  = "http://localhost:9373/oauth/callback"
)

func configDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "synca")
}

// loadEnv loads a physical .env file for development overrides. Release
// builds carry the public client identifier as a constant and never embed
// secret material.
func loadEnv() {
	dirs := []string{".", "..", "../.."}
	for _, dir := range dirs {
		envPath := filepath.Join(dir, ".env")
		if _, err := os.Stat(envPath); err == nil {
			_ = godotenv.Load(envPath)
			log.Debug().Str("path", envPath).Msg("Loaded environment from file")
			return
		}
	}
}

func oauthConfig() *oauth2.Config {
	loadEnv()

	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	if clientID == "" {
		clientID = googleClientID
	}

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: "",
		Endpoint:     google.Endpoint,
		Scopes:       []string{drive.DriveScope},
		RedirectURL:  localRedirectURL,
	}
}

// RunOAuthFlow handles browser login + token exchange using PKCE
func RunOAuthFlow() error {
	cfg := oauthConfig()

	if cfg.ClientID == "" || cfg.ClientID == "YOUR_CLIENT_ID_HERE.apps.googleusercontent.com" {
		return fmt.Errorf("GOOGLE_CLIENT_ID is missing. Set it in a local .env file or rebuild with a valid public client ID")
	}

	// Generate a cryptographically secure random state to protect against CSRF attacks
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Errorf("failed to generate secure state: %w", err)
	}
	state := hex.EncodeToString(b)

	// Generate PKCE params
	pkce, err := NewPKCEParams()
	if err != nil {
		return err
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	srv := &http.Server{Addr: ":" + localRedirectPort, Handler: mux}
	defer srv.Shutdown(context.Background())

	mux.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		cbState := r.URL.Query().Get("state")
		if cbState != state {
			errCh <- fmt.Errorf("OAuth state mismatch: potential CSRF")
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no code in OAuth callback")
			http.Error(w, "Authentication failed", http.StatusBadRequest)
			return
		}

		fmt.Fprintf(w, `<html><body style="font-family:sans-serif;padding:40px;text-align:center">
		<h2 style="color:#0F6E56">✓ Synca connected!</h2>
		<p>You can close this window and return to the app.</p>
		</body></html>`)

		codeCh <- code
	})

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Build auth URL with PKCE challenge and random state
	authURL := cfg.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce,
		oauth2.SetAuthURLParam("code_challenge", pkce.Challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)

	log.Info().Msg("Opening browser for Google Drive authentication (PKCE)...")
	openBrowser(authURL)

	var code string
	select {
	case code = <-codeCh:
	case err = <-errCh:
		return err
	}

	ctx := context.Background()
	// Exchange with PKCE verifier
	token, err := cfg.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", pkce.Verifier))
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}

	if err := saveToken(token); err != nil {
		return err
	}

	log.Info().Msg("✓ Authenticated with Google Drive successfully")
	return nil
}

// saveToken persists token.json
func saveToken(token *oauth2.Token) error {
	dir := configDir()
	_ = os.MkdirAll(dir, 0700)

	tokenFile := filepath.Join(dir, "token.json")

	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(tokenFile, data, 0600)
}

// LoadToken reads stored token
func LoadToken() (*oauth2.Token, error) {
	tokenFile := filepath.Join(configDir(), "token.json")

	data, err := os.ReadFile(tokenFile)
	if err != nil {
		return nil, fmt.Errorf("not authenticated — run: synca connect google-drive")
	}

	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

// NewDriveService creates authenticated Drive client
func NewDriveService(ctx context.Context, proxy config.ProxySettings) (*drive.Service, error) {
	cfg := oauthConfig()

	token, err := LoadToken()
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{}
	switch proxy.Mode {
	case config.ProxyModeSystem:
		transport.Proxy = http.ProxyFromEnvironment
	case config.ProxyModeManual:
		if proxy.Type == config.ProxyTypeSOCKS && proxy.InsecureSkipVerify {
			transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		}
		if proxy.Type == config.ProxyTypeHTTP {
			proxyURL, err := config.ManualHTTPProxyURL(proxy)
			if err != nil {
				return nil, err
			}
			transport.Proxy = http.ProxyURL(proxyURL)
		} else {
			addr, err := config.ManualSOCKSAddress(proxy)
			if err != nil {
				return nil, err
			}
			var auth *netproxy.Auth
			if proxy.Username != "" {
				auth = &netproxy.Auth{
					User:     proxy.Username,
					Password: proxy.Password,
				}
			}
			dialer, err := netproxy.SOCKS5("tcp", addr, auth, netproxy.Direct)
			if err != nil {
				return nil, err
			}
			transport.DialContext = socksDialContext(dialer)
		}
	}

	baseClient := &http.Client{Transport: transport}
	proxyCtx := context.WithValue(ctx, oauth2.HTTPClient, baseClient)
	client := &http.Client{
		Transport: &oauth2.Transport{
			Base:   transport,
			Source: cfg.TokenSource(proxyCtx, token),
		},
	}

	return drive.NewService(ctx, option.WithHTTPClient(client))
}

func socksDialContext(dialer netproxy.Dialer) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		type dialContext interface {
			DialContext(context.Context, string, string) (net.Conn, error)
		}
		if d, ok := dialer.(dialContext); ok {
			return d.DialContext(ctx, network, address)
		}

		connCh := make(chan net.Conn, 1)
		errCh := make(chan error, 1)
		go func() {
			conn, err := dialer.Dial(network, address)
			if err != nil {
				errCh <- err
				return
			}
			connCh <- conn
		}()

		select {
		case conn := <-connCh:
			return conn, nil
		case err := <-errCh:
			return nil, err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}
