// Command api serves the EnergyHub Universal backend.
//
// The health endpoint is always available. Protected routes are only
// registered when authentication and the database are fully configured, so a
// half-configured deployment exposes no protected surface.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"energyhub-universal/backend/internal/authn"
	"energyhub-universal/backend/internal/platform/httpapi"
	"energyhub-universal/backend/internal/site"
	"energyhub-universal/backend/internal/storage/postgres"
)

var (
	ErrIncompleteAuthConfig     = errors.New("authentication configuration is incomplete")
	ErrIncompleteDatabaseConfig = errors.New("database configuration is incomplete")
)

const (
	defaultListenAddr  = ":8080"
	startupPingTimeout = 5 * time.Second
	shutdownTimeout    = 5 * time.Second
	readTimeout        = 15 * time.Second
	writeTimeout       = 30 * time.Second
	idleTimeout        = 60 * time.Second
	maxHeaderBytes     = 8 << 10
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

type apiConfig struct {
	auth        authn.Auth0Config
	databaseURL string
}

func (c apiConfig) authenticationEnabled() bool {
	return c.auth.Issuer != "" && c.auth.Audience != ""
}

// loadAPIConfig reads the deployment configuration. Authentication and the
// database are configured together: one without the other is a mistake, not a
// degraded mode.
func loadAPIConfig(getenv func(string) string) (apiConfig, error) {
	if getenv == nil {
		return apiConfig{}, errors.New("environment reader is required")
	}

	issuer := strings.TrimSpace(getenv("AUTH0_ISSUER"))
	audience := strings.TrimSpace(getenv("AUTH0_AUDIENCE"))
	databaseURL := strings.TrimSpace(getenv("DATABASE_URL"))

	switch {
	case issuer == "" && audience == "":
		if databaseURL != "" {
			return apiConfig{}, fmt.Errorf("%w: set AUTH0_ISSUER and AUTH0_AUDIENCE to enable the database", ErrIncompleteAuthConfig)
		}
		return apiConfig{}, nil
	case issuer == "" || audience == "":
		return apiConfig{}, fmt.Errorf("%w: set both AUTH0_ISSUER and AUTH0_AUDIENCE", ErrIncompleteAuthConfig)
	}

	auth := authn.Auth0Config{Issuer: issuer, Audience: audience}
	if err := auth.Validate(); err != nil {
		return apiConfig{}, err
	}
	if databaseURL == "" {
		return apiConfig{}, fmt.Errorf("%w: set DATABASE_URL to read memberships", ErrIncompleteDatabaseConfig)
	}

	return apiConfig{auth: auth, databaseURL: databaseURL}, nil
}

func run() error {
	addr := strings.TrimSpace(os.Getenv("LISTEN_ADDR"))
	if addr == "" {
		addr = defaultListenAddr
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	config, err := loadAPIConfig(os.Getenv)
	if err != nil {
		return err
	}
	if config.authenticationEnabled() && config.auth.UsesLoopbackHTTPIssuer() && !isLoopbackListenAddr(addr) {
		return fmt.Errorf("the development issuer over plain HTTP requires a loopback listen address, got %q", addr)
	}

	router, cleanup, err := buildRouter(ctx, config)
	if err != nil {
		return err
	}
	defer cleanup()

	server := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}

	errorChannel := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errorChannel <- err
			return
		}
		errorChannel <- nil
	}()

	log.Printf("api listening on %s", addr)

	select {
	case err := <-errorChannel:
		return err
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return server.Shutdown(shutdownContext)
	}
}

// isLoopbackListenAddr reports whether the server binds a loopback address.
// An empty host means every interface, which is not loopback.
func isLoopbackListenAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "" {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// buildRouter wires the router and returns the cleanup that releases the
// resources it owns. The database pool stays open for the lifetime of the
// server, so it is not closed here.
func buildRouter(ctx context.Context, config apiConfig) (http.Handler, func(), error) {
	if !config.authenticationEnabled() {
		log.Printf("authentication is not configured: only /health is exposed")
		return httpapi.NewRouter(), func() {}, nil
	}

	pool, err := pgxpool.New(ctx, config.databaseURL)
	if err != nil {
		return nil, func() {}, fmt.Errorf("connect database: %w", postgres.RedactConnectionError(err, config.databaseURL))
	}
	cleanup := func() { pool.Close() }

	pingContext, cancel := context.WithTimeout(ctx, startupPingTimeout)
	defer cancel()
	if err := pool.Ping(pingContext); err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("ping database: %w", postgres.RedactConnectionError(err, config.databaseURL))
	}

	// The API must connect with the read only runtime role. A credential that
	// is a superuser, that bypasses row level security or that can write the
	// membership tables would read or change every tenant.
	//
	// The inspection has its own deadline: a database that does not answer
	// must stop the startup instead of leaving the process waiting forever.
	roleVerifier := postgres.NewRuntimeRoleVerifier(postgres.NewPgxRoleInspector(pool))
	roleContext, cancelRole := context.WithTimeout(ctx, startupPingTimeout)
	defer cancelRole()
	if err := roleVerifier.VerifyRuntimeRole(roleContext); err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("verify database role: %w", err)
	}

	verifier, err := authn.NewAuth0Verifier(ctx, config.auth, nil)
	if err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("create auth verifier: %w", err)
	}

	reader := postgres.NewMembershipReader(postgres.NewDB(pool))
	siteService := site.NewService(postgres.NewSiteReader(postgres.NewDB(pool)))
	log.Printf("protected routes enabled for issuer %s", config.auth.Issuer)

	return httpapi.NewRouter(
		httpapi.WithAuthentication(verifier, reader),
		httpapi.WithSites(siteService),
	), cleanup, nil
}
