package authn

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"

	"energyhub-universal/backend/internal/authz"
)

var ErrInvalidAuth0Token = errors.New("invalid Auth0 token")

const (
	// defaultHTTPTimeout bounds the discovery request and the JWKS refresh, so
	// a slow issuer cannot block the startup or a request forever.
	defaultHTTPTimeout = 10 * time.Second
	discoveryTimeout   = 10 * time.Second
)

type oidcTokenVerifier interface {
	Verify(ctx context.Context, rawToken string) (*oidc.IDToken, error)
}

type Auth0Verifier struct {
	verifier oidcTokenVerifier
}

// NewAuth0Verifier discovers the issuer metadata and JWKS endpoint. The
// caller owns the lifecycle of the context and HTTP client.
func NewAuth0Verifier(ctx context.Context, config Auth0Config, client *http.Client) (*Auth0Verifier, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if ctx == nil {
		return nil, fmt.Errorf("%w: context is nil", ErrInvalidAuth0Config)
	}
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	} else if client.Timeout == 0 {
		// The key set refresh drops the context deadline, so the client
		// timeout is the only bound left on it. A client without one would
		// let a slow issuer hold the refresh forever.
		cloned := *client
		cloned.Timeout = defaultHTTPTimeout
		client = &cloned
	}

	discoveryContext, cancel := context.WithTimeout(ctx, discoveryTimeout)
	defer cancel()

	provider, err := oidc.NewProvider(oidc.ClientContext(discoveryContext, client), config.Issuer)
	if err != nil {
		return nil, fmt.Errorf("%w: provider discovery failed: %v", ErrInvalidAuth0Config, err)
	}

	return &Auth0Verifier{
		verifier: provider.Verifier(&oidc.Config{
			ClientID: config.Audience,
		}),
	}, nil
}

// Verify validates the token signature and standard OIDC claims through the
// Auth0 discovery configuration. Organization membership is resolved later by
// the server and is intentionally not taken from token claims here.
func (v *Auth0Verifier) Verify(ctx context.Context, token string) (authz.Principal, error) {
	if v == nil || v.verifier == nil {
		return authz.Principal{}, ErrAuth0VerifierUnavailable
	}
	if ctx == nil {
		return authz.Principal{}, ErrInvalidAuth0Token
	}

	idToken, err := v.verifier.Verify(ctx, token)
	if err != nil {
		return authz.Principal{}, fmt.Errorf("%w: token verification failed", ErrInvalidAuth0Token)
	}
	return principalFromAuth0Subject(idToken.Subject)
}
