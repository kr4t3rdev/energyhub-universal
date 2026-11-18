// Command devoidc runs the development-only OIDC issuer.
//
// It exists so the authentication and membership flow can be exercised before
// an Auth0 tenant exists. It listens on loopback only and must never be
// deployed as a real identity provider.
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

	"energyhub-universal/backend/internal/devoidc"
)

const (
	defaultAddr           = "127.0.0.1:8089"
	defaultAudience       = "https://api.energyhub.local"
	defaultClientID       = "energyhub-dev"
	defaultClientSecret   = "dev-secret"
	defaultAdminSubject   = "dev|admin"
	defaultAdminUsername  = "admin"
	defaultAdminPassword  = "admin-dev"
	defaultUserSubject    = "dev|user-1"
	defaultUserUsername   = "user1"
	defaultUserPassword   = "user1-dev"
	defaultReadinessDelay = 2 * time.Second
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

type config struct {
	addr          string
	audience      string
	clientID      string
	clientSecret  string
	adminSubject  string
	adminUsername string
	adminPassword string
	userSubject   string
	userUsername  string
	userPassword  string
}

func loadConfig(getenv func(string) string) (config, error) {
	if getenv == nil {
		return config{}, errors.New("environment reader is required")
	}
	loaded := config{
		addr:          strings.TrimSpace(getenv("DEV_OIDC_ADDR")),
		audience:      strings.TrimSpace(getenv("DEV_OIDC_AUDIENCE")),
		clientID:      strings.TrimSpace(getenv("DEV_OIDC_CLIENT_ID")),
		clientSecret:  strings.TrimSpace(getenv("DEV_OIDC_CLIENT_SECRET")),
		adminSubject:  strings.TrimSpace(getenv("DEV_OIDC_ADMIN_SUBJECT")),
		adminUsername: strings.TrimSpace(getenv("DEV_OIDC_ADMIN_USERNAME")),
		adminPassword: strings.TrimSpace(getenv("DEV_OIDC_ADMIN_PASSWORD")),
		userSubject:   strings.TrimSpace(getenv("DEV_OIDC_USER_SUBJECT")),
		userUsername:  strings.TrimSpace(getenv("DEV_OIDC_USER_USERNAME")),
		userPassword:  strings.TrimSpace(getenv("DEV_OIDC_USER_PASSWORD")),
	}

	if loaded.addr == "" {
		loaded.addr = defaultAddr
	}
	if loaded.audience == "" {
		loaded.audience = defaultAudience
	}
	if loaded.clientID == "" {
		loaded.clientID = defaultClientID
	}
	if loaded.clientSecret == "" {
		loaded.clientSecret = defaultClientSecret
	}
	if loaded.adminSubject == "" {
		loaded.adminSubject = defaultAdminSubject
	}
	if loaded.adminUsername == "" {
		loaded.adminUsername = defaultAdminUsername
	}
	if loaded.adminPassword == "" {
		loaded.adminPassword = defaultAdminPassword
	}
	if loaded.userSubject == "" {
		loaded.userSubject = defaultUserSubject
	}
	if loaded.userUsername == "" {
		loaded.userUsername = defaultUserUsername
	}
	if loaded.userPassword == "" {
		loaded.userPassword = defaultUserPassword
	}

	if err := validateLoopbackAddr(loaded.addr); err != nil {
		return config{}, err
	}
	return loaded, nil
}

func validateLoopbackAddr(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("DEV_OIDC_ADDR must be host:port: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("DEV_OIDC_ADDR must listen on a literal loopback address, got %q", host)
	}
	return nil
}

func run() error {
	loaded, err := loadConfig(os.Getenv)
	if err != nil {
		return err
	}

	issuerURL := "http://" + loaded.addr
	issuer, err := devoidc.New(devoidc.Config{
		Issuer:       issuerURL,
		Audience:     loaded.audience,
		ClientID:     loaded.clientID,
		ClientSecret: loaded.clientSecret,
		Users: []devoidc.User{
			{Subject: loaded.adminSubject, Username: loaded.adminUsername, Password: loaded.adminPassword, Admin: true},
			{Subject: loaded.userSubject, Username: loaded.userUsername, Password: loaded.userPassword},
		},
	})
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              loaded.addr,
		Handler:           issuer.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    8 << 10,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errorChannel := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errorChannel <- err
			return
		}
		errorChannel <- nil
	}()

	printInstructions(loaded, issuerURL)

	select {
	case err := <-errorChannel:
		return err
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), defaultReadinessDelay)
		defer cancel()
		return server.Shutdown(shutdownContext)
	}
}

// printInstructions shows how to use the development issuer without writing
// the secrets to the log: the values live in the environment of the process
// and in the development defaults.
func printInstructions(loaded config, issuerURL string) {
	log.Printf("development OIDC issuer listening on %s (loopback only, never deploy this)", issuerURL)
	log.Printf("AUTH0_ISSUER=%s", issuerURL)
	log.Printf("AUTH0_AUDIENCE=%s", loaded.audience)
	log.Printf("SYSTEM_ADMIN_SUBJECT=%s", loaded.adminSubject)
	log.Printf("client id: %s (secret in DEV_OIDC_CLIENT_SECRET)", loaded.clientID)
	log.Printf("admin user: %s (password in DEV_OIDC_ADMIN_PASSWORD)", loaded.adminUsername)
	log.Printf("regular user: %s (password in DEV_OIDC_USER_PASSWORD)", loaded.userUsername)
	log.Printf("the secrets are not logged: the command below reads them from the environment and falls back to the development defaults")
	log.Printf("request a token with:")
	log.Printf(`  curl -s -X POST %s/oauth/token -H 'Content-Type: application/json' -d "{\"client_id\":\"%s\",\"client_secret\":\"${DEV_OIDC_CLIENT_SECRET:-%s}\",\"username\":\"%s\",\"password\":\"${DEV_OIDC_ADMIN_PASSWORD:-%s}\",\"audience\":\"%s\"}"`,
		issuerURL, loaded.clientID, defaultClientSecret, loaded.adminUsername, defaultAdminPassword, loaded.audience)
}
