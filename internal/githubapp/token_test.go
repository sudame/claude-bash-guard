package githubapp

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testKeyPEM(t *testing.T) ([]byte, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	return pemBytes, key
}

func TestMintJWTStructureAndSignature(t *testing.T) {
	// Given a generated RSA key and a fixed time
	pemBytes, key := testKeyPEM(t)
	now := time.Unix(1_700_000_000, 0)

	// When a JWT is minted
	token, err := MintJWT(42, pemBytes, now)
	require.NoError(t, err)

	// Then it has three segments with a verifiable RS256 signature
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)

	signingInput := parts[0] + "." + parts[1]
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	require.NoError(t, err)
	assert.NoError(t, rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, digest[:], sig))
}

func TestMintJWTClaims(t *testing.T) {
	// Given a generated RSA key and a fixed time
	pemBytes, _ := testKeyPEM(t)
	now := time.Unix(1_700_000_000, 0)

	// When a JWT is minted
	token, err := MintJWT(42, pemBytes, now)
	require.NoError(t, err)

	// Then the claims carry the issuer and a back-dated iat within a ~10m window
	var claims struct {
		Iat int64 `json:"iat"`
		Exp int64 `json:"exp"`
		Iss int64 `json:"iss"`
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.Split(token, ".")[1])
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(payload, &claims))

	assert.Equal(t, int64(42), claims.Iss)
	assert.Equal(t, now.Unix()-30, claims.Iat)
	assert.Equal(t, now.Add(9*time.Minute).Unix(), claims.Exp)
}

func TestMintJWTRejectsBadPEM(t *testing.T) {
	// Given input that is not a PEM block
	// When a JWT is minted
	_, err := MintJWT(42, []byte("not a pem"), time.Unix(0, 0))

	// Then it errors
	assert.Error(t, err)
}

func TestInstallationToken(t *testing.T) {
	// Given a stub GitHub API returning a token
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/app/installations/99/access_tokens", r.URL.Path)
		assert.Equal(t, "Bearer jwt-here", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"token":"ghs_secret"}`))
	}))
	defer srv.Close()

	orig := apiBaseURL
	apiBaseURL = srv.URL
	defer func() { apiBaseURL = orig }()

	// When an installation token is requested
	token, err := InstallationToken(context.Background(), srv.Client(), "jwt-here", 99)

	// Then the token from the response is returned
	require.NoError(t, err)
	assert.Equal(t, "ghs_secret", token)
}

func TestInstallationTokenErrorStatus(t *testing.T) {
	// Given a stub GitHub API returning 401
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"bad jwt"}`))
	}))
	defer srv.Close()

	orig := apiBaseURL
	apiBaseURL = srv.URL
	defer func() { apiBaseURL = orig }()

	// When an installation token is requested
	_, err := InstallationToken(context.Background(), srv.Client(), "jwt-here", 99)

	// Then it errors with the upstream status
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}
