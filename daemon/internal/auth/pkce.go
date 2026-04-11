package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// GenerateCodeVerifier creates a high-entropy cryptographic random string.
func GenerateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GenerateCodeChallenge creates a S256 challenge from the verifier.
func GenerateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// PKCEParams holds the verifier and challenge.
type PKCEParams struct {
	Verifier  string
	Challenge string
}

// NewPKCEParams initializes a new PKCE handshake.
func NewPKCEParams() (*PKCEParams, error) {
	v, err := GenerateCodeVerifier()
	if err != nil {
		return nil, fmt.Errorf("failed to generate code verifier: %w", err)
	}
	return &PKCEParams{
		Verifier:  v,
		Challenge: GenerateCodeChallenge(v),
	}, nil
}
