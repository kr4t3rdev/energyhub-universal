package authn

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"energyhub-universal/backend/internal/authz"
)

var (
	ErrInvalidAuth0Config       = errors.New("invalid Auth0 configuration")
	ErrInvalidAuth0Principal    = errors.New("invalid Auth0 principal")
	ErrAuth0VerifierUnavailable = errors.New("Auth0 verifier unavailable")
)

type Auth0Config struct {
	Issuer   string
	Audience string
}

// LoadAuth0Config reads deployment configuration without embedding tenant
// URLs or API identifiers in the repository.
func LoadAuth0Config(getenv func(string) string) (Auth0Config, error) {
	if getenv == nil {
		return Auth0Config{}, ErrInvalidAuth0Config
	}

	config := Auth0Config{
		Issuer:   strings.TrimSpace(getenv("AUTH0_ISSUER")),
		Audience: strings.TrimSpace(getenv("AUTH0_AUDIENCE")),
	}
	if err := config.Validate(); err != nil {
		return Auth0Config{}, err
	}
	return config, nil
}

// Validate requires an HTTPS issuer and rejects values that could alter OIDC
// discovery or token audience matching.
func (c Auth0Config) Validate() error {
	if c.Issuer == "" || c.Audience == "" {
		return ErrInvalidAuth0Config
	}

	issuer, err := url.Parse(c.Issuer)
	if err != nil ||
		issuer.Scheme != "https" ||
		issuer.Host == "" ||
		issuer.User != nil ||
		issuer.RawQuery != "" ||
		issuer.Fragment != "" {
		return fmt.Errorf("%w: issuer must be an HTTPS URL without credentials, query, or fragment", ErrInvalidAuth0Config)
	}
	if strings.ContainsAny(c.Audience, " \t\r\n") {
		return fmt.Errorf("%w: audience must not contain whitespace", ErrInvalidAuth0Config)
	}
	return nil
}

func principalFromAuth0Subject(subject string) (authz.Principal, error) {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return authz.Principal{}, ErrInvalidAuth0Principal
	}
	return authz.Principal{Subject: subject}, nil
}
