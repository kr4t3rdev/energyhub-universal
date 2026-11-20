package apiapp

import (
	"errors"
	"fmt"
	"strings"

	"energyhub-universal/backend/internal/authn"
)

var (
	ErrIncompleteAuthConfig     = errors.New("authentication configuration is incomplete")
	ErrIncompleteDatabaseConfig = errors.New("database configuration is incomplete")
)

type Config struct {
	Auth        authn.Auth0Config
	DatabaseURL string
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

	switch {
	case issuer == "" && audience == "":
		if databaseURL != "" {
			return Config{}, fmt.Errorf("%w: set AUTH0_ISSUER and AUTH0_AUDIENCE to enable the database", ErrIncompleteAuthConfig)
		}
		return Config{}, nil
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

	return Config{Auth: auth, DatabaseURL: databaseURL}, nil
}
