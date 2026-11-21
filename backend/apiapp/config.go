package apiapp

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"energyhub-universal/backend/internal/authn"
)

var (
	ErrIncompleteAuthConfig     = errors.New("authentication configuration is incomplete")
	ErrIncompleteDatabaseConfig = errors.New("database configuration is incomplete")
)

type Config struct {
	Auth              authn.Auth0Config
	DatabaseURL       string
	CORSAllowedOrigin string
}

func (c Config) AuthenticationEnabled() bool {
	return c.Auth.Issuer != "" && c.Auth.Audience != ""
}

// LoadConfig requires authentication and the database to be configured
// together. A partial deployment must not expose database-backed routes.
func LoadConfig(getenv func(string) string) (Config, error) {
	if getenv == nil {
		return Config{}, errors.New("environment reader is required")
	}

	issuer := strings.TrimSpace(getenv("AUTH0_ISSUER"))
	audience := strings.TrimSpace(getenv("AUTH0_AUDIENCE"))
	databaseURL := strings.TrimSpace(getenv("DATABASE_URL"))
	corsAllowedOrigin := strings.TrimSpace(getenv("CORS_ALLOWED_ORIGIN"))
	if corsAllowedOrigin != "" {
		if err := validateCORSOrigin(corsAllowedOrigin); err != nil {
			return Config{}, err
		}
	}

	switch {
	case issuer == "" && audience == "":
		if databaseURL != "" {
			return Config{}, fmt.Errorf("%w: set AUTH0_ISSUER and AUTH0_AUDIENCE to enable the database", ErrIncompleteAuthConfig)
		}
		return Config{CORSAllowedOrigin: corsAllowedOrigin}, nil
	case issuer == "" || audience == "":
		return Config{}, fmt.Errorf("%w: set both AUTH0_ISSUER and AUTH0_AUDIENCE", ErrIncompleteAuthConfig)
	}

	auth := authn.Auth0Config{Issuer: issuer, Audience: audience}
	if err := auth.Validate(); err != nil {
		return Config{}, err
	}
	if databaseURL == "" {
		return Config{}, fmt.Errorf("%w: set DATABASE_URL to read memberships", ErrIncompleteDatabaseConfig)
	}

	return Config{Auth: auth, DatabaseURL: databaseURL, CORSAllowedOrigin: corsAllowedOrigin}, nil
}

func validateCORSOrigin(origin string) error {
	parsed, err := url.Parse(origin)
	if err != nil || parsed == nil || parsed.Opaque != "" || parsed.User != nil || parsed.Hostname() == "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" {
		return errors.New("CORS_ALLOWED_ORIGIN must be an HTTP(S) origin without a path")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return errors.New("CORS_ALLOWED_ORIGIN must be an HTTP(S) origin without a path")
	}
	if parsed.Host == "" || strings.ContainsAny(parsed.Host, " \t\r\n") {
		return errors.New("CORS_ALLOWED_ORIGIN must be an HTTP(S) origin without a path")
	}
	return nil
}
