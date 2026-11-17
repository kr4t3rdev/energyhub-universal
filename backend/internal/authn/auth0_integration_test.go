package authn

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	jose "github.com/go-jose/go-jose/v4"
)

func TestNewAuth0VerifierDiscoversJWKSAndVerifiesToken(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewTLSServer(nil)
	defer server.Close()
	server.Config.Handler = http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"issuer":                                server.URL,
				"jwks_uri":                              server.URL + "/keys",
				"id_token_signing_alg_values_supported": []string{"RS256"},
			})
		case "/keys":
			_ = json.NewEncoder(writer).Encode(jose.JSONWebKeySet{
				Keys: []jose.JSONWebKey{{
					Key:       &privateKey.PublicKey,
					KeyID:     "test-key",
					Algorithm: string(jose.RS256),
					Use:       "sig",
				}},
			})
		default:
			http.NotFound(writer, request)
		}
	})

	config := Auth0Config{
		Issuer:   server.URL,
		Audience: "https://api.energyhub.example",
	}
	verifier, err := NewAuth0Verifier(context.Background(), config, server.Client())
	if err != nil {
		t.Fatal(err)
	}

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: privateKey},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test-key"),
	)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"iss": server.URL,
		"sub": "auth0|user-1",
		"aud": config.Audience,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	signed, err := signer.Sign(payload)
	if err != nil {
		t.Fatal(err)
	}
	rawToken, err := signed.CompactSerialize()
	if err != nil {
		t.Fatal(err)
	}

	principal, err := verifier.Verify(context.Background(), rawToken)
	if err != nil {
		t.Fatal(err)
	}
	if principal.Subject != "auth0|user-1" {
		t.Fatalf("subject = %q, want auth0|user-1", principal.Subject)
	}
	if principal.Organizations != nil {
		t.Fatalf("organizations = %#v, want nil", principal.Organizations)
	}
}

var _ oidcTokenVerifier = (*oidc.IDTokenVerifier)(nil)
