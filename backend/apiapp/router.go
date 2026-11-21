package apiapp

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"energyhub-universal/backend/internal/authn"
	"energyhub-universal/backend/internal/platform/httpapi"
	"energyhub-universal/backend/internal/site"
	"energyhub-universal/backend/internal/storage/postgres"
)

const startupPingTimeout = 5 * time.Second

// BuildRouter wires routes and returns cleanup for resources it creates.
func BuildRouter(ctx context.Context, config Config) (http.Handler, func(), error) {
	if !config.AuthenticationEnabled() {
		log.Printf("authentication is not configured: only /health is exposed")
		return httpapi.NewRouter(httpapi.WithCORS(config.CORSAllowedOrigin)), func() {}, nil
	}

	pool, err := pgxpool.New(ctx, config.DatabaseURL)
	if err != nil {
		return nil, func() {}, fmt.Errorf("connect database: %w", postgres.RedactConnectionError(err, config.DatabaseURL))
	}
	cleanup := func() { pool.Close() }

	pingContext, cancel := context.WithTimeout(ctx, startupPingTimeout)
	defer cancel()
	if err := pool.Ping(pingContext); err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("ping database: %w", postgres.RedactConnectionError(err, config.DatabaseURL))
	}

	roleVerifier := postgres.NewRuntimeRoleVerifier(postgres.NewPgxRoleInspector(pool))
	roleContext, cancelRole := context.WithTimeout(ctx, startupPingTimeout)
	defer cancelRole()
	if err := roleVerifier.VerifyRuntimeRole(roleContext); err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("verify database role: %w", err)
	}

	verifier, err := authn.NewAuth0Verifier(ctx, config.Auth, nil)
	if err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("create auth verifier: %w", err)
	}

	reader := postgres.NewMembershipReader(postgres.NewDB(pool))
	siteService := site.NewService(postgres.NewSiteReader(postgres.NewDB(pool)))
	log.Printf("protected routes enabled for issuer %s", config.Auth.Issuer)

	return httpapi.NewRouter(
		httpapi.WithCORS(config.CORSAllowedOrigin),
		httpapi.WithAuthentication(verifier, reader),
		httpapi.WithSites(siteService),
	), cleanup, nil
}
