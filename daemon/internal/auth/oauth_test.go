package auth

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func TestOAuthConfigUsesCommittedPublicClientID(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GOOGLE_CLIENT_SECRET", "")
	restore := setBuildTimeSecret("")
	defer restore()

	cfg := oauthConfig()

	if cfg.ClientID != googleClientID {
		t.Fatalf("oauthConfig().ClientID = %q, want committed public client ID %q", cfg.ClientID, googleClientID)
	}
	// Plain builds (and tests) carry no client secret: the value is injected
	// only by release builds through -ldflags from the protected
	// GOOGLE_CLIENT_SECRET Actions secret, or overridden at runtime.
	if cfg.ClientSecret != "" {
		t.Fatalf("oauthConfig().ClientSecret = %q, want empty without a build-time or runtime secret", cfg.ClientSecret)
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
	t.Setenv("GOOGLE_CLIENT_SECRET", "")

	cfg := oauthConfig()

	if cfg.ClientID != "override-id.apps.googleusercontent.com" {
		t.Fatalf("oauthConfig().ClientID = %q, want env override to win", cfg.ClientID)
	}
}

func TestOAuthConfigReadsClientSecretFromEnv(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GOOGLE_CLIENT_SECRET", "test-client-secret")
	restore := setBuildTimeSecret("build-time-secret")
	defer restore()

	cfg := oauthConfig()

	if cfg.ClientSecret != "test-client-secret" {
		t.Fatalf("oauthConfig().ClientSecret = %q, want runtime secret from environment", cfg.ClientSecret)
	}
}

func TestOAuthConfigFallsBackToBuildTimeSecret(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GOOGLE_CLIENT_SECRET", "")
	restore := setBuildTimeSecret("build-time-secret")
	defer restore()

	cfg := oauthConfig()

	if cfg.ClientSecret != "build-time-secret" {
		t.Fatalf("oauthConfig().ClientSecret = %q, want build-time injected secret", cfg.ClientSecret)
	}
}

// setBuildTimeSecret sets the build-time injected package variable for the
// duration of a test and restores the previous value on return.
func setBuildTimeSecret(value string) func() {
	previous := googleClientSecret
	googleClientSecret = value
	return func() { googleClientSecret = previous }
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

func TestValidateClientID(t *testing.T) {
	cases := []struct {
		name string
		id   string
		ok   bool
	}{
		{"committed public client id", googleClientID, true},
		{"env override with valid shape", "override-id.apps.googleusercontent.com", true},
		{"empty", "", false},
		{"placeholder shape", "YOUR_CLIENT_ID_HERE.apps.googleusercontent.com", false},
		{"not a google client id", "not-a-client-id", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateClientID(tc.id)
			if (err == nil) != tc.ok {
				t.Fatalf("validateClientID(%q) error = %v, want ok=%v", tc.id, err, tc.ok)
			}
		})
	}
}

func TestExplainTokenErrorMissingClientSecret(t *testing.T) {
	re := &oauth2.RetrieveError{
		ErrorCode:        "invalid_request",
		ErrorDescription: "client_secret is missing.",
	}

	err := ExplainTokenError(re)

	if err == nil {
		t.Fatal("ExplainTokenError returned nil for a missing-client-secret rejection")
	}
	if !strings.Contains(err.Error(), "GOOGLE_CLIENT_SECRET") {
		t.Fatalf("ExplainTokenError() = %q, want actionable GOOGLE_CLIENT_SECRET guidance", err)
	}
	if !errors.Is(err, re) {
		t.Fatal("ExplainTokenError() must preserve the original error in the chain")
	}
}

func TestExplainTokenErrorOtherFailuresPassThrough(t *testing.T) {
	re := &oauth2.RetrieveError{
		ErrorCode:        "invalid_grant",
		ErrorDescription: "Bad Request",
	}

	if got := ExplainTokenError(re); got != re {
		t.Fatalf("ExplainTokenError() = %v, want original error unchanged for non-secret failures", got)
	}
	if got := ExplainTokenError(fmt.Errorf("network down")); got == nil || !strings.Contains(got.Error(), "network down") {
		t.Fatalf("ExplainTokenError() = %v, want unrelated errors passed through", got)
	}
	if got := ExplainTokenError(nil); got != nil {
		t.Fatalf("ExplainTokenError(nil) = %v, want nil", got)
	}
}
