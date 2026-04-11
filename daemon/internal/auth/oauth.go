package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

const (
	localRedirectPort = "9373"
	localRedirectURL  = "http://localhost:9373/oauth/callback"
)

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "synca")
}

// oauthConfig loads credentials.json
func oauthConfig() (*oauth2.Config, error) {
	credFile := filepath.Join(configDir(), "credentials.json")

	data, err := os.ReadFile(credFile)
	if err != nil {
		return nil, fmt.Errorf(
			"credentials.json not found at %s — download it from Google Cloud Console (OAuth 2.0 Client ID for Desktop app)",
				       credFile,
		)
	}

	cfg, err := google.ConfigFromJSON(data, drive.DriveScope)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials.json: %w", err)
	}

	cfg.RedirectURL = localRedirectURL
	return cfg, nil
}

// RunOAuthFlow handles browser login + token exchange
func RunOAuthFlow() error {
	cfg, err := oauthConfig()
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

		fmt.Fprintf(w, `<html><body style="font-family:sans-serif;padding:40px">
		<h2>✓ Synca conectado ao Google Drive!</h2>
		<p>Você pode fechar esta janela.</p>
		</body></html>`)

		codeCh <- code
	})

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	authURL := cfg.AuthCodeURL("synca-state", oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	log.Info().Str("url", authURL).Msg("Opening browser for Google Drive authentication...")

	openBrowser(authURL)

	fmt.Printf("\nIf the browser didn't open, visit:\n%s\n\n", authURL)

	var code string
	select {
		case code = <-codeCh:
		case err = <-errCh:
			return err
	}

	ctx := context.Background()
	token, err := cfg.Exchange(ctx, code)
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
	cfg, err := oauthConfig()
	if err != nil {
		return nil, err
	}

	token, err := LoadToken()
	if err != nil {
		return nil, err
	}

	client := cfg.Client(ctx, token)

	return drive.NewService(ctx, option.WithHTTPClient(client))
}
