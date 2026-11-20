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
	"strconv"
	"strings"
	"syscall"
	"time"

	"energyhub-universal/backend/apiapp"
	"energyhub-universal/backend/internal/authn"
)

var (
	ErrIncompleteAuthConfig     = apiapp.ErrIncompleteAuthConfig
	ErrIncompleteDatabaseConfig = apiapp.ErrIncompleteDatabaseConfig
)

const (
	defaultListenAddr = ":8080"
	shutdownTimeout   = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second
	maxHeaderBytes    = 8 << 10
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
	config, err := apiapp.LoadConfig(getenv)
	if err != nil {
		return apiConfig{}, err
	}
	return apiConfig{auth: config.Auth, databaseURL: config.DatabaseURL}, nil
}

func run() error {
	addr, err := configuredListenAddr(os.Getenv)
	if err != nil {
		return err
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

func configuredListenAddr(getenv func(string) string) (string, error) {
	if port := strings.TrimSpace(getenv("PORT")); port != "" {
		parsed, err := strconv.ParseUint(port, 10, 16)
		if err != nil || parsed == 0 {
			return "", errors.New("PORT must be a valid TCP port")
		}
		return ":" + strconv.FormatUint(parsed, 10), nil
	}

	if addr := strings.TrimSpace(getenv("LISTEN_ADDR")); addr != "" {
		return addr, nil
	}
	return defaultListenAddr, nil
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
	return apiapp.BuildRouter(ctx, apiapp.Config{Auth: config.auth, DatabaseURL: config.databaseURL})
}
