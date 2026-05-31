// Package githubapp mints GitHub App JWTs and exchanges them for installation
// access tokens, using only the standard library.
package githubapp

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// MintJWT builds an RS256-signed JWT for the given App ID, valid for ~9 minutes.
// The issued-at time is back-dated 30s to tolerate clock drift, per GitHub's guidance.
func MintJWT(appID int64, pemKey []byte, now time.Time) (string, error) {
	key, err := parseRSAPrivateKey(pemKey)
	if err != nil {
		return "", err
	}

	header := map[string]string{"alg": "RS256", "typ": "JWT"}
	claims := map[string]any{
		"iat": now.Add(-30 * time.Second).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": appID,
	}

	signingInput, err := joinSegments(header, claims)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("signing JWT: %w", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func joinSegments(parts ...any) (string, error) {
	encoded := make([]string, len(parts))
	for i, p := range parts {
		b, err := json.Marshal(p)
		if err != nil {
			return "", fmt.Errorf("encoding JWT segment: %w", err)
		}
		encoded[i] = base64.RawURLEncoding.EncodeToString(b)
	}
	return strings.Join(encoded, "."), nil
}

func parseRSAPrivateKey(pemKey []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemKey)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in private key")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing private key (tried PKCS1 and PKCS8): %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not RSA (got %T)", parsed)
	}
	return key, nil
}

// apiBaseURL is overridable in tests.
var apiBaseURL = "https://api.github.com"

// InstallationToken exchanges an App JWT for a short-lived installation access token.
func InstallationToken(ctx context.Context, client *http.Client, jwt string, installationID int64) (string, error) {
	url := fmt.Sprintf("%s/app/installations/%d/access_tokens", apiBaseURL, installationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("requesting installation token: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("installation token request failed: %s: %s", resp.Status, bytes.TrimSpace(body))
	}

	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("decoding installation token response: %w", err)
	}
	if out.Token == "" {
		return "", fmt.Errorf("installation token response contained no token")
	}
	return out.Token, nil
}
