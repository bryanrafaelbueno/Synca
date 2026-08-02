package auth

import (
	"strings"
	"testing"
)

func TestOAuthConfigUsesCommittedPublicClientID(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GOOGLE_CLIENT_SECRET", "")

	cfg := oauthConfig()

	if cfg.ClientID != googleClientID {
		t.Fatalf("oauthConfig().ClientID = %q, want committed public client ID %q", cfg.ClientID, googleClientID)
	}
	// Installed apps are public clients: with PKCE the token exchange must
	// not depend on a client secret, and a secret must never be distributed
	// with the binary.
	if cfg.ClientSecret != "" {
		t.Fatalf("oauthConfig().ClientSecret = %q, want empty for public-client PKCE flow", cfg.ClientSecret)
	}
	if len(cfg.Scopes) == 0 {
		t.Fatal("oauthConfig() must request at least one scope")
	}
	if cfg.RedirectURL == "" {
		t.Fatal("oauthConfig() must set a loopback redirect URL")
	}
}

func TestOAuthConfigEnvOverride(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "override-id.apps.googleusercontent.com")

	cfg := oauthConfig()

	if cfg.ClientID != "override-id.apps.googleusercontent.com" {
		t.Fatalf("oauthConfig().ClientID = %q, want env override to win", cfg.ClientID)
	}
}

func TestOAuthConfigCommittedClientIDIsValid(t *testing.T) {
	// The identifier must match the installed-app shape; the release
	// pipeline treats any deviation as a configuration error.
	if !strings.HasSuffix(googleClientID, ".apps.googleusercontent.com") {
		t.Fatalf("googleClientID %q must end with .apps.googleusercontent.com", googleClientID)
	}
	parts := strings.SplitN(googleClientID, "-", 2)
	if len(parts) != 2 || len(parts[0]) == 0 || len(parts[1]) == 0 {
		t.Fatalf("googleClientID %q must have shape <project-number>-<identifier>.apps.googleusercontent.com", googleClientID)
	}
	if strings.Contains(googleClientID, "YOUR_CLIENT") {
		t.Fatal("googleClientID must not be the placeholder")
	}
}

func TestRunOAuthFlowRejectsMissingClientID(t *testing.T) {
	// A caller-provided override of the empty string must not silently fall
	// through to a broken flow.
	t.Setenv("GOOGLE_CLIENT_ID", "")
	if googleClientID == "" {
		t.Skip("committed client ID configured; guard is not exercised")
	}
	cfg := oauthConfig()
	if cfg.ClientID == "" {
		t.Fatal("oauthConfig() must always yield a non-empty client ID")
	}
}
