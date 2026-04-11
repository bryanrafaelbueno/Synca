package auth

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

//go:embed .env.embedded
var embeddedEnv string

const (
	localRedirectPort = "9373"
	localRedirectURL  = "http://localhost:9373/oauth/callback"
)

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "synca")
}

// loadEnv attempts to load .env from current dir or parent dirs,
// falling back to embedded credentials if available.
func loadEnv() {
	// 1. Try physical .env files (Dev mode)
	dirs := []string{".", "..", "../.."}
	for _, dir := range dirs {
		envPath := filepath.Join(dir, ".env")
		if _, err := os.Stat(envPath); err == nil {
			_ = godotenv.Load(envPath)
			log.Debug().Str("path", envPath).Msg("Loaded environment from file")
			return
		}
	}

	// 2. Try embedded .env (Production/Release mode)
	if embeddedEnv != "" {
		env, err := godotenv.Unmarshal(embeddedEnv)
		if err == nil {
			for k, v := range env {
				if os.Getenv(k) == "" {
					os.Setenv(k, v)
				}
			}
			log.Debug().Msg("Loaded environment from embedded fallback")
			return
		}
	}
}

// Embedded Google OAuth2 Credentials fallback
var (
	googleClientID     = os.Getenv("GOOGLE_CLIENT_ID")
	googleClientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
)

func oauthConfig() *oauth2.Config {
	loadEnv()
	
	// Re-read env after loading .env file
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	if clientID == "" {
		clientID = googleClientID // Fallback
	}
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	if clientSecret == "" {
		clientSecret = googleClientSecret // Fallback
	}

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{drive.DriveScope},
		RedirectURL:  localRedirectURL,
	}
}

// RunOAuthFlow handles browser login + token exchange using PKCE
func RunOAuthFlow() error {
	cfg := oauthConfig()

	if cfg.ClientID == "" || cfg.ClientID == "YOUR_CLIENT_ID.apps.googleusercontent.com" {
		return fmt.Errorf("GOOGLE_CLIENT_ID is missing. Please set it in your .env file at the project root")
	}

	// Generate PKCE params
	pkce, err := NewPKCEParams()
	if err != nil {
		return err
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	srv := &http.Server{Addr: ":" + localRedirectPort, Handler: mux}

	mux.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
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

	// Build auth URL with PKCE challenge
	authURL := cfg.AuthCodeURL("synca-state", 
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

	_ = srv.Shutdown(ctx)

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
func NewDriveService(ctx context.Context) (*drive.Service, error) {
	cfg := oauthConfig()

	token, err := LoadToken()
	if err != nil {
		return nil, err
	}

	client := cfg.Client(ctx, token)

	return drive.NewService(ctx, option.WithHTTPClient(client))
}
