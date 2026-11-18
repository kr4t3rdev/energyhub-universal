package authn

import (
	"context"
	"errors"
	"testing"

	"github.com/coreos/go-oidc/v3/oidc"
)

func TestLoadAuth0ConfigRequiresIssuerAndAudience(t *testing.T) {
	config, err := LoadAuth0Config(func(key string) string {
		switch key {
		case "AUTH0_ISSUER":
			return "https://energyhub.eu.auth0.com/"
		case "AUTH0_AUDIENCE":
			return "https://api.energyhub.example"
		default:
			return ""
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if config.Issuer != "https://energyhub.eu.auth0.com/" {
		t.Fatalf("issuer = %q", config.Issuer)
	}
	if config.Audience != "https://api.energyhub.example" {
		t.Fatalf("audience = %q", config.Audience)
	}
}

func TestLoadAuth0ConfigRejectsInvalidIssuer(t *testing.T) {
	_, err := LoadAuth0Config(func(key string) string {
		if key == "AUTH0_ISSUER" {
			return "http://auth.example.test"
		}
		if key == "AUTH0_AUDIENCE" {
			return "api"
		}
		return ""
	})
	if !errors.Is(err, ErrInvalidAuth0Config) {
		t.Fatalf("err = %v, want ErrInvalidAuth0Config", err)
	}
}

func TestLoadAuth0ConfigRejectsMissingAudience(t *testing.T) {
	_, err := LoadAuth0Config(func(key string) string {
		if key == "AUTH0_ISSUER" {
			return "https://auth.example.test/"
		}
		return ""
	})
	if !errors.Is(err, ErrInvalidAuth0Config) {
		t.Fatalf("err = %v, want ErrInvalidAuth0Config", err)
	}
}

func TestLoadAuth0ConfigRejectsNilEnvironmentReader(t *testing.T) {
	_, err := LoadAuth0Config(nil)
	if !errors.Is(err, ErrInvalidAuth0Config) {
		t.Fatalf("err = %v, want ErrInvalidAuth0Config", err)
	}
}

func TestAuth0ConfigRejectsUnsafeIssuerAndAudienceValues(t *testing.T) {
	tests := []struct {
		name     string
		issuer   string
		audience string
	}{
		{name: "missing host", issuer: "https://", audience: "api"},
		{name: "credentials", issuer: "https://user:pass@example.test", audience: "api"},
		{name: "query", issuer: "https://auth.example.test?tenant=one", audience: "api"},
		{name: "fragment", issuer: "https://auth.example.test/#tenant", audience: "api"},
		{name: "malformed URL", issuer: "https://auth.example.test/%zz", audience: "api"},
		{name: "audience whitespace", issuer: "https://auth.example.test", audience: "api energyhub"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := (Auth0Config{Issuer: test.issuer, Audience: test.audience}).Validate()
			if !errors.Is(err, ErrInvalidAuth0Config) {
				t.Fatalf("err = %v, want ErrInvalidAuth0Config", err)
			}
		})
	}
}

func TestAuth0ConfigAcceptsLoopbackIssuerForDevelopment(t *testing.T) {
	tests := []struct {
		name   string
		issuer string
	}{
		{name: "ipv4 loopback", issuer: "http://127.0.0.1:8089"},
		{name: "ipv6 loopback", issuer: "http://[::1]:8089"},
		{name: "loopback over tls", issuer: "https://127.0.0.1:8443"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := Auth0Config{Issuer: test.issuer, Audience: "https://api.energyhub.local"}
			if err := config.Validate(); err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
		})
	}
}

func TestAuth0ConfigRejectsLocalhostNameForPlainHTTP(t *testing.T) {
	config := Auth0Config{Issuer: "http://localhost:8089", Audience: "https://api.energyhub.local"}
	if err := config.Validate(); !errors.Is(err, ErrInvalidAuth0Config) {
		t.Fatalf("err = %v, want ErrInvalidAuth0Config", err)
	}
}

func TestAuth0ConfigDetectsTheLoopbackDevelopmentIssuer(t *testing.T) {
	loopback := Auth0Config{Issuer: "http://127.0.0.1:8089", Audience: "api"}
	if !loopback.UsesLoopbackHTTPIssuer() {
		t.Fatal("loopback HTTP issuer not detected")
	}

	secure := Auth0Config{Issuer: "https://energyhub.eu.auth0.com/", Audience: "api"}
	if secure.UsesLoopbackHTTPIssuer() {
		t.Fatal("secure issuer reported as loopback development issuer")
	}
}

func TestAuth0ConfigRejectsNonLoopbackPlainHTTPIssuer(t *testing.T) {
	tests := []string{
		"http://auth.example.test",
		"http://10.0.0.5:8089",
		"http://192.168.1.10:8089",
		"http://energyhub.local:8089",
	}

	for _, issuer := range tests {
		t.Run(issuer, func(t *testing.T) {
			err := (Auth0Config{Issuer: issuer, Audience: "api"}).Validate()
			if !errors.Is(err, ErrInvalidAuth0Config) {
				t.Fatalf("err = %v, want ErrInvalidAuth0Config", err)
			}
		})
	}
}

func TestPrincipalFromAuth0TokenRequiresSubject(t *testing.T) {
	principal, err := principalFromAuth0Subject("auth0|user-1")
	if err != nil {
		t.Fatal(err)
	}
	if principal.Subject != "auth0|user-1" || principal.Organizations != nil {
		t.Fatalf("principal = %#v, want subject only", principal)
	}
}

func TestPrincipalFromAuth0TokenRejectsMissingSubject(t *testing.T) {
	_, err := principalFromAuth0Subject(" ")
	if !errors.Is(err, ErrInvalidAuth0Principal) {
		t.Fatalf("err = %v, want ErrInvalidAuth0Principal", err)
	}
}

func TestAuth0VerifierRejectsUnavailableVerifier(t *testing.T) {
	var verifier *Auth0Verifier
	_, err := verifier.Verify(context.Background(), "token")
	if !errors.Is(err, ErrAuth0VerifierUnavailable) {
		t.Fatalf("err = %v, want ErrAuth0VerifierUnavailable", err)
	}
}

func TestNewAuth0VerifierRejectsInvalidConfigBeforeDiscovery(t *testing.T) {
	_, err := NewAuth0Verifier(context.Background(), Auth0Config{}, nil)
	if !errors.Is(err, ErrInvalidAuth0Config) {
		t.Fatalf("err = %v, want ErrInvalidAuth0Config", err)
	}
}

func TestAuth0VerifierRejectsNilContext(t *testing.T) {
	verifier := &Auth0Verifier{verifier: &fakeOIDCTokenVerifier{}}
	_, err := verifier.Verify(nil, "token")
	if !errors.Is(err, ErrInvalidAuth0Token) {
		t.Fatalf("err = %v, want ErrInvalidAuth0Token", err)
	}
}

type fakeOIDCTokenVerifier struct {
	token *oidc.IDToken
	err   error
}

func (f *fakeOIDCTokenVerifier) Verify(context.Context, string) (*oidc.IDToken, error) {
	return f.token, f.err
}

func TestAuth0VerifierReturnsSubjectFromVerifiedToken(t *testing.T) {
	verifier := &Auth0Verifier{
		verifier: &fakeOIDCTokenVerifier{
			token: &oidc.IDToken{Subject: "auth0|user-1"},
		},
	}

	principal, err := verifier.Verify(context.Background(), "token")
	if err != nil {
		t.Fatal(err)
	}
	if principal.Subject != "auth0|user-1" || principal.Organizations != nil {
		t.Fatalf("principal = %#v, want subject only", principal)
	}
}

func TestAuth0VerifierHidesVerificationErrors(t *testing.T) {
	verifier := &Auth0Verifier{
		verifier: &fakeOIDCTokenVerifier{err: errors.New("private token details")},
	}

	_, err := verifier.Verify(context.Background(), "token")
	if !errors.Is(err, ErrInvalidAuth0Token) {
		t.Fatalf("err = %v, want ErrInvalidAuth0Token", err)
	}
	if err.Error() == "invalid Auth0 token: token verification failed: private token details" {
		t.Fatal("token verifier details leaked")
	}
}

func TestAuth0VerifierRejectsVerifiedTokenWithoutSubject(t *testing.T) {
	verifier := &Auth0Verifier{
		verifier: &fakeOIDCTokenVerifier{token: &oidc.IDToken{}},
	}

	_, err := verifier.Verify(context.Background(), "token")
	if !errors.Is(err, ErrInvalidAuth0Principal) {
		t.Fatalf("err = %v, want ErrInvalidAuth0Principal", err)
	}
}
