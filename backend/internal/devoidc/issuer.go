// Package devoidc implements a development-only OIDC issuer.
//
// It exists so the authentication and membership flow can be exercised before
// an Auth0 tenant exists. It is not an Auth0 emulator: it implements only the
// subset the API relies on (discovery, JWKS and a token endpoint) and it is
// never imported by the production API binary.
package devoidc

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

var ErrInvalidConfig = errors.New("invalid development issuer configuration")

const (
	defaultTokenTTL = time.Hour
	signingKeyID    = "dev-signing-key"
	// maxTokenRequestBytes bounds the token request body, so a local client
	// cannot exhaust the memory of the development issuer.
	maxTokenRequestBytes = 16 << 10
)

// User is a development identity. Subjects are synthetic and never come from
// a real identity provider.
type User struct {
	Subject  string
	Username string
	Password string
	Admin    bool
}

// Config describes the development issuer.
type Config struct {
	// Issuer is the advertised issuer URL. It must be a loopback address: the
	// issuer refuses to run anywhere else.
	Issuer       string
	Audience     string
	ClientID     string
	ClientSecret string
	Users        []User
	TTL          time.Duration
}

// Issuer serves the development OIDC endpoints.
type Issuer struct {
	issuerURL    string
	audience     string
	clientID     string
	clientSecret string
	ttl          time.Duration
	adminSubject string
	users        map[string]User
	privateKey   *rsa.PrivateKey
	signer       jose.Signer
}

// New validates the configuration and generates an ephemeral signing key.
func New(config Config) (*Issuer, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("%w: generate signing key: %v", ErrInvalidConfig, err)
	}
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", signingKeyID),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: create signer: %v", ErrInvalidConfig, err)
	}

	users := make(map[string]User, len(config.Users))
	adminSubject := ""
	for _, user := range config.Users {
		users[user.Username] = user
		if user.Admin {
			adminSubject = user.Subject
		}
	}

	ttl := config.TTL
	if ttl == 0 {
		ttl = defaultTokenTTL
	}

	return &Issuer{
		issuerURL:    strings.TrimRight(config.Issuer, "/"),
		audience:     config.Audience,
		clientID:     config.ClientID,
		clientSecret: config.ClientSecret,
		ttl:          ttl,
		adminSubject: adminSubject,
		users:        users,
		privateKey:   key,
		signer:       signer,
	}, nil
}

func validateConfig(config Config) error {
	if err := validateLoopbackURL(config.Issuer); err != nil {
		return err
	}
	if strings.TrimSpace(config.Audience) == "" {
		return fmt.Errorf("%w: audience is required", ErrInvalidConfig)
	}
	if strings.TrimSpace(config.ClientID) == "" || strings.TrimSpace(config.ClientSecret) == "" {
		return fmt.Errorf("%w: client credentials are required", ErrInvalidConfig)
	}
	if len(config.Users) == 0 {
		return fmt.Errorf("%w: at least one user is required", ErrInvalidConfig)
	}

	usernames := make(map[string]struct{}, len(config.Users))
	for _, user := range config.Users {
		if strings.TrimSpace(user.Subject) == "" {
			return fmt.Errorf("%w: user subject is required", ErrInvalidConfig)
		}
		if strings.TrimSpace(user.Username) == "" {
			return fmt.Errorf("%w: user username is required", ErrInvalidConfig)
		}
		if strings.TrimSpace(user.Password) == "" {
			return fmt.Errorf("%w: user password is required", ErrInvalidConfig)
		}
		if _, exists := usernames[user.Username]; exists {
			return fmt.Errorf("%w: duplicate username %q", ErrInvalidConfig, user.Username)
		}
		usernames[user.Username] = struct{}{}
	}
	return nil
}

func validateLoopbackURL(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Errorf("%w: issuer is required", ErrInvalidConfig)
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("%w: issuer must be a plain URL", ErrInvalidConfig)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%w: issuer must use http or https", ErrInvalidConfig)
	}
	// The advertised issuer must be a literal loopback address. A name such
	// as localhost depends on DNS and on the hosts file, so it is not proof
	// that the traffic stays on the machine.
	if !isLoopbackIP(parsed.Hostname()) {
		return fmt.Errorf("%w: issuer must point to a literal loopback address", ErrInvalidConfig)
	}
	return nil
}

// isLoopbackIP reports whether the host is a literal loopback address.
func isLoopbackIP(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// isLoopbackHost reports whether the host names the local machine. It is used
// for the Host header, which the client chooses, so it accepts the localhost
// name for convenience; the peer address is what the security decision uses.
func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	return isLoopbackIP(host)
}

// AdminSubject returns the subject flagged as the development administrator.
func (i *Issuer) AdminSubject() string {
	if i == nil {
		return ""
	}
	return i.adminSubject
}

// IssuerURL returns the advertised issuer URL.
func (i *Issuer) IssuerURL() string {
	if i == nil {
		return ""
	}
	return i.issuerURL
}

// Audience returns the audience accepted by the issuer.
func (i *Issuer) Audience() string {
	if i == nil {
		return ""
	}
	return i.audience
}

// PublicKey exposes the verification key so tests can validate issued tokens.
func (i *Issuer) PublicKey() crypto.PublicKey {
	if i == nil || i.privateKey == nil {
		return nil
	}
	return &i.privateKey.PublicKey
}

// Handler returns the development issuer endpoints.
func (i *Issuer) Handler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		// The Host header is chosen by the client, so the peer address is the
		// check that matters. The listener is bound to loopback as well.
		if !isLoopbackHost(hostOnly(request.Host)) || !isLoopbackPeer(request.RemoteAddr) {
			writeError(writer, http.StatusForbidden, "forbidden")
			return
		}

		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/.well-known/openid-configuration":
			i.writeDiscovery(writer)
		case request.Method == http.MethodGet && request.URL.Path == "/keys":
			i.writeJWKS(writer)
		case request.URL.Path == "/oauth/token":
			if request.Method != http.MethodPost {
				writer.Header().Set("Allow", http.MethodPost)
				writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed")
				return
			}
			i.handleToken(writer, request)
		default:
			writeError(writer, http.StatusNotFound, "not_found")
		}
	})
}

func hostOnly(host string) string {
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		return parsedHost
	}
	return host
}

// isLoopbackPeer reports whether the request came from the local machine.
func isLoopbackPeer(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (i *Issuer) writeDiscovery(writer http.ResponseWriter) {
	writeJSON(writer, http.StatusOK, map[string]any{
		"issuer":                                i.issuerURL,
		"jwks_uri":                              i.issuerURL + "/keys",
		"token_endpoint":                        i.issuerURL + "/oauth/token",
		"response_types_supported":              []string{"token"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{string(jose.RS256)},
		"token_endpoint_auth_methods_supported": []string{"client_secret_post"},
	})
}

func (i *Issuer) writeJWKS(writer http.ResponseWriter) {
	writeJSON(writer, http.StatusOK, jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{{
			Key:       &i.privateKey.PublicKey,
			KeyID:     signingKeyID,
			Algorithm: string(jose.RS256),
			Use:       "sig",
		}},
	})
}

type tokenRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	Audience     string `json:"audience"`
}

func (i *Issuer) handleToken(writer http.ResponseWriter, request *http.Request) {
	body := http.MaxBytesReader(writer, request.Body, maxTokenRequestBytes)
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()

	var payload tokenRequest
	if err := decoder.Decode(&payload); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(writer, http.StatusRequestEntityTooLarge, "request_too_large")
			return
		}
		writeError(writer, http.StatusBadRequest, "invalid_request")
		return
	}

	// The body must hold exactly one JSON value. Without this check a valid
	// object followed by more content would be accepted, and the size limit
	// would not be applied to the trailing part.
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(writer, http.StatusRequestEntityTooLarge, "request_too_large")
			return
		}
		writeError(writer, http.StatusBadRequest, "invalid_request")
		return
	}

	if !matches(payload.ClientID, i.clientID) || !matches(payload.ClientSecret, i.clientSecret) {
		writeError(writer, http.StatusUnauthorized, "invalid_client")
		return
	}
	if strings.TrimSpace(payload.Audience) != i.audience {
		writeError(writer, http.StatusBadRequest, "invalid_target")
		return
	}

	user, ok := i.users[payload.Username]
	if !ok || !matches(payload.Password, user.Password) {
		writeError(writer, http.StatusUnauthorized, "invalid_grant")
		return
	}

	rawToken, err := i.issueToken(request.Context(), user.Subject, i.audience)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "server_error")
		return
	}

	writer.Header().Set("Cache-Control", "no-store")
	writeJSON(writer, http.StatusOK, map[string]any{
		"access_token": rawToken,
		"token_type":   "Bearer",
		"expires_in":   int(i.ttl.Seconds()),
		"scope":        "openid",
	})
}

// IssueToken signs a development access token for the given subject and
// audience. It exists so tests can mint tokens that the token endpoint would
// refuse, such as a token issued for a different audience.
func (i *Issuer) IssueToken(ctx context.Context, subject, audience string) (string, error) {
	return i.issueToken(ctx, subject, audience)
}

// issueToken signs a development access token for the given subject.
func (i *Issuer) issueToken(ctx context.Context, subject, audience string) (string, error) {
	if i == nil || i.signer == nil {
		return "", ErrInvalidConfig
	}
	if ctx == nil {
		return "", ErrInvalidConfig
	}
	subject = strings.TrimSpace(subject)
	if subject == "" || strings.TrimSpace(audience) == "" {
		return "", ErrInvalidConfig
	}

	now := time.Now()
	claims := map[string]any{
		"iss": i.issuerURL,
		"sub": subject,
		"aud": audience,
		"iat": now.Unix(),
		"exp": now.Add(i.ttl).Unix(),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}
	signed, err := i.signer.Sign(payload)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	rawToken, err := signed.CompactSerialize()
	if err != nil {
		return "", fmt.Errorf("serialize token: %w", err)
	}
	return rawToken, nil
}

func matches(provided, expected string) bool {
	providedDigest := sha256.Sum256([]byte(provided))
	expectedDigest := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(providedDigest[:], expectedDigest[:]) == 1
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

func writeError(writer http.ResponseWriter, status int, code string) {
	writeJSON(writer, status, map[string]any{"error": code})
}
