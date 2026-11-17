package authn

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coreos/go-oidc/v3/oidc"

	"energyhub-universal/backend/internal/devoidc"
)

const devAudience = "https://api.energyhub.local"

// newDevIssuer starts the development issuer behind a loopback server. The
// issuer is advertised with the server URL so discovery and token validation
// agree on the issuer value.
func newDevIssuer(t *testing.T, useTLS bool) (*httptest.Server, *devoidc.Issuer) {
	t.Helper()

	var handler http.Handler
	var server *httptest.Server
	if useTLS {
		server = httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			handler.ServeHTTP(writer, request)
		}))
	} else {
		server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			handler.ServeHTTP(writer, request)
		}))
	}
	t.Cleanup(server.Close)

	issuer, err := devoidc.New(devoidc.Config{
		Issuer:       server.URL,
		Audience:     devAudience,
		ClientID:     "energyhub-dev",
		ClientSecret: "dev-secret",
		Users: []devoidc.User{
			{Subject: "dev|user-1", Username: "user1", Password: "user1-pass"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	handler = issuer.Handler()
	return server, issuer
}

func requestDevToken(t *testing.T, server *httptest.Server) string {
	t.Helper()
	payload, err := json.Marshal(map[string]string{
		"client_id":     "energyhub-dev",
		"client_secret": "dev-secret",
		"username":      "user1",
		"password":      "user1-pass",
		"audience":      devAudience,
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Client().Post(server.URL+"/oauth/token", "application/json", strings.NewReader(string(payload)))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("token status = %d, want 200", response.StatusCode)
	}
	var decoded struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.AccessToken == "" {
		t.Fatal("token response has no access_token")
	}
	return decoded.AccessToken
}

func TestNewAuth0VerifierDiscoversJWKSAndVerifiesToken(t *testing.T) {
	server, _ := newDevIssuer(t, false)

	config := Auth0Config{
		Issuer:   server.URL,
		Audience: devAudience,
	}
	verifier, err := NewAuth0Verifier(context.Background(), config, server.Client())
	if err != nil {
		t.Fatal(err)
	}

	principal, err := verifier.Verify(context.Background(), requestDevToken(t, server))
	if err != nil {
		t.Fatal(err)
	}
	if principal.Subject != "dev|user-1" {
		t.Fatalf("subject = %q, want dev|user-1", principal.Subject)
	}
	if principal.Organizations != nil {
		t.Fatalf("organizations = %#v, want nil", principal.Organizations)
	}
}

func TestNewAuth0VerifierVerifiesTokenFromLoopbackTLSIssuer(t *testing.T) {
	server, _ := newDevIssuer(t, true)

	config := Auth0Config{
		Issuer:   server.URL,
		Audience: devAudience,
	}
	verifier, err := NewAuth0Verifier(context.Background(), config, server.Client())
	if err != nil {
		t.Fatal(err)
	}

	principal, err := verifier.Verify(context.Background(), requestDevToken(t, server))
	if err != nil {
		t.Fatal(err)
	}
	if principal.Subject != "dev|user-1" {
		t.Fatalf("subject = %q, want dev|user-1", principal.Subject)
	}
}

func TestAuth0VerifierRejectsTokenSignedForAnotherAudience(t *testing.T) {
	server, issuer := newDevIssuer(t, false)

	verifier, err := NewAuth0Verifier(
		context.Background(),
		Auth0Config{Issuer: server.URL, Audience: devAudience},
		server.Client(),
	)
	if err != nil {
		t.Fatal(err)
	}

	rawToken, err := issuer.IssueToken(context.Background(), "dev|user-1", "https://api.other.local")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.Verify(context.Background(), rawToken); err == nil {
		t.Fatal("err = nil, want audience mismatch rejection")
	}
}

var _ oidcTokenVerifier = (*oidc.IDTokenVerifier)(nil)
