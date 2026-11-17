package devoidc

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

func testConfig() Config {
	return Config{
		Issuer:       "http://127.0.0.1:8089",
		Audience:     "https://api.energyhub.local",
		ClientID:     "energyhub-dev",
		ClientSecret: "dev-secret",
		Users: []User{
			{Subject: "dev|admin", Username: "admin", Password: "admin-pass", Admin: true},
			{Subject: "dev|user-1", Username: "user1", Password: "user1-pass"},
		},
	}
}

func newTestIssuer(t *testing.T) (*Issuer, *httptest.Server) {
	t.Helper()
	var handler http.Handler
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		handler.ServeHTTP(writer, request)
	}))
	t.Cleanup(server.Close)

	config := testConfig()
	config.Issuer = server.URL
	issuer, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	handler = issuer.Handler()
	return issuer, server
}

func postToken(t *testing.T, server *httptest.Server, body map[string]string) (int, map[string]any) {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, server.URL+"/oauth/token", strings.NewReader(string(payload)))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	decoded := map[string]any{}
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("decode response %q: %v", string(data), err)
		}
	}
	return response.StatusCode, decoded
}

func TestNewRejectsNonLoopbackIssuer(t *testing.T) {
	config := testConfig()
	config.Issuer = "https://tenant.us.auth0.com"

	if _, err := New(config); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err = %v, want ErrInvalidConfig", err)
	}
}

func TestNewRejectsEmptyIssuer(t *testing.T) {
	config := testConfig()
	config.Issuer = "  "

	if _, err := New(config); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err = %v, want ErrInvalidConfig", err)
	}
}

func TestNewRejectsMissingClientCredentials(t *testing.T) {
	config := testConfig()
	config.ClientSecret = ""

	if _, err := New(config); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err = %v, want ErrInvalidConfig", err)
	}
}

func TestNewRejectsMissingAudience(t *testing.T) {
	config := testConfig()
	config.Audience = ""

	if _, err := New(config); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err = %v, want ErrInvalidConfig", err)
	}
}

func TestNewRejectsUserWithoutSubject(t *testing.T) {
	config := testConfig()
	config.Users = []User{{Username: "admin", Password: "admin-pass"}}

	if _, err := New(config); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err = %v, want ErrInvalidConfig", err)
	}
}

func TestNewRejectsUserWithoutPassword(t *testing.T) {
	config := testConfig()
	config.Users = []User{{Subject: "dev|admin", Username: "admin"}}

	if _, err := New(config); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err = %v, want ErrInvalidConfig", err)
	}
}

func TestNewRejectsDuplicateUsername(t *testing.T) {
	config := testConfig()
	config.Users = append(config.Users, User{Subject: "dev|user-2", Username: "admin", Password: "other"})

	if _, err := New(config); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err = %v, want ErrInvalidConfig", err)
	}
}

func TestDiscoveryDescribesIssuerAndJWKS(t *testing.T) {
	_, server := newTestIssuer(t)

	response, err := http.Get(server.URL + "/.well-known/openid-configuration")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}

	var document struct {
		Issuer   string   `json:"issuer"`
		JWKSURI  string   `json:"jwks_uri"`
		TokenURL string   `json:"token_endpoint"`
		Algs     []string `json:"id_token_signing_alg_values_supported"`
	}
	if err := json.NewDecoder(response.Body).Decode(&document); err != nil {
		t.Fatal(err)
	}
	if document.Issuer != server.URL {
		t.Fatalf("issuer = %q, want %q", document.Issuer, server.URL)
	}
	if document.JWKSURI != server.URL+"/keys" {
		t.Fatalf("jwks_uri = %q", document.JWKSURI)
	}
	if document.TokenURL != server.URL+"/oauth/token" {
		t.Fatalf("token_endpoint = %q", document.TokenURL)
	}
	if len(document.Algs) != 1 || document.Algs[0] != "RS256" {
		t.Fatalf("algs = %#v, want [RS256]", document.Algs)
	}
}

func TestJWKSExposesSigningKey(t *testing.T) {
	_, server := newTestIssuer(t)

	response, err := http.Get(server.URL + "/keys")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}

	var keySet jose.JSONWebKeySet
	if err := json.NewDecoder(response.Body).Decode(&keySet); err != nil {
		t.Fatal(err)
	}
	if len(keySet.Keys) != 1 {
		t.Fatalf("keys = %d, want 1", len(keySet.Keys))
	}
	key := keySet.Keys[0]
	if key.KeyID == "" || key.Algorithm != string(jose.RS256) || key.Use != "sig" {
		t.Fatalf("key = %#v", key)
	}
}

func TestTokenEndpointIssuesSignedToken(t *testing.T) {
	issuer, server := newTestIssuer(t)

	status, body := postToken(t, server, map[string]string{
		"client_id":     "energyhub-dev",
		"client_secret": "dev-secret",
		"username":      "user1",
		"password":      "user1-pass",
		"audience":      "https://api.energyhub.local",
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, body = %#v", status, body)
	}
	rawToken, _ := body["access_token"].(string)
	if rawToken == "" {
		t.Fatalf("body = %#v, want access_token", body)
	}
	if body["token_type"] != "Bearer" {
		t.Fatalf("token_type = %v, want Bearer", body["token_type"])
	}
	if expiresIn, _ := body["expires_in"].(float64); expiresIn <= 0 {
		t.Fatalf("expires_in = %v, want positive", body["expires_in"])
	}

	parsed, err := jose.ParseSigned(rawToken, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		t.Fatal(err)
	}
	publicKey, ok := issuer.PublicKey().(*rsa.PublicKey)
	if !ok {
		t.Fatal("issuer public key is not RSA")
	}
	payloadBytes, err := parsed.Verify(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	claims := map[string]any{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		t.Fatal(err)
	}
	if claims["iss"] != server.URL {
		t.Fatalf("iss = %v, want %q", claims["iss"], server.URL)
	}
	if claims["sub"] != "dev|user-1" {
		t.Fatalf("sub = %v, want dev|user-1", claims["sub"])
	}
	if claims["aud"] != "https://api.energyhub.local" {
		t.Fatalf("aud = %v", claims["aud"])
	}
	expiry, _ := claims["exp"].(float64)
	if expiry <= float64(time.Now().Unix()) {
		t.Fatalf("exp = %v, want future timestamp", claims["exp"])
	}
}

func TestTokenEndpointRejectsInvalidClientSecret(t *testing.T) {
	_, server := newTestIssuer(t)

	status, body := postToken(t, server, map[string]string{
		"client_id":     "energyhub-dev",
		"client_secret": "wrong",
		"username":      "user1",
		"password":      "user1-pass",
		"audience":      "https://api.energyhub.local",
	})
	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", status)
	}
	if body["error"] != "invalid_client" {
		t.Fatalf("error = %v, want invalid_client", body["error"])
	}
	if _, ok := body["access_token"]; ok {
		t.Fatal("response contains a token")
	}
}

func TestTokenEndpointRejectsWrongPassword(t *testing.T) {
	_, server := newTestIssuer(t)

	status, body := postToken(t, server, map[string]string{
		"client_id":     "energyhub-dev",
		"client_secret": "dev-secret",
		"username":      "user1",
		"password":      "wrong",
		"audience":      "https://api.energyhub.local",
	})
	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", status)
	}
	if body["error"] != "invalid_grant" {
		t.Fatalf("error = %v, want invalid_grant", body["error"])
	}
}

func TestTokenEndpointRejectsUnknownUser(t *testing.T) {
	_, server := newTestIssuer(t)

	status, body := postToken(t, server, map[string]string{
		"client_id":     "energyhub-dev",
		"client_secret": "dev-secret",
		"username":      "nobody",
		"password":      "user1-pass",
		"audience":      "https://api.energyhub.local",
	})
	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", status)
	}
	if body["error"] != "invalid_grant" {
		t.Fatalf("error = %v, want invalid_grant", body["error"])
	}
}

func TestTokenEndpointRejectsUnexpectedAudience(t *testing.T) {
	_, server := newTestIssuer(t)

	status, body := postToken(t, server, map[string]string{
		"client_id":     "energyhub-dev",
		"client_secret": "dev-secret",
		"username":      "user1",
		"password":      "user1-pass",
		"audience":      "https://other.example",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}
	if body["error"] != "invalid_target" {
		t.Fatalf("error = %v, want invalid_target", body["error"])
	}
}

func TestTokenEndpointRejectsMalformedBody(t *testing.T) {
	_, server := newTestIssuer(t)

	response, err := http.Post(server.URL+"/oauth/token", "application/json", strings.NewReader("not-json"))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.StatusCode)
	}
}

func TestTokenEndpointRejectsGet(t *testing.T) {
	_, server := newTestIssuer(t)

	response, err := http.Get(server.URL + "/oauth/token")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", response.StatusCode)
	}
}

func TestUnknownPathReturnsNotFound(t *testing.T) {
	_, server := newTestIssuer(t)

	response, err := http.Get(server.URL + "/admin")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.StatusCode)
	}
}

func TestAdminSubjectIsExposed(t *testing.T) {
	issuer, err := New(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	if issuer.AdminSubject() != "dev|admin" {
		t.Fatalf("admin subject = %q, want dev|admin", issuer.AdminSubject())
	}
}

func TestHandlerRejectsNonLoopbackHost(t *testing.T) {
	_, server := newTestIssuer(t)

	request, err := http.NewRequest(http.MethodGet, server.URL+"/.well-known/openid-configuration", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Host = "energyhub.example.com"
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.StatusCode)
	}
}

func TestIssuerSurvivesConcurrentTokenRequests(t *testing.T) {
	_, server := newTestIssuer(t)

	type result struct {
		status int
		body   map[string]any
	}
	results := make(chan result, 4)
	for range 4 {
		go func() {
			status, body := postToken(t, server, map[string]string{
				"client_id":     "energyhub-dev",
				"client_secret": "dev-secret",
				"username":      "user1",
				"password":      "user1-pass",
				"audience":      "https://api.energyhub.local",
			})
			results <- result{status: status, body: body}
		}()
	}
	for range 4 {
		got := <-results
		if got.status != http.StatusOK {
			t.Fatalf("status = %d, body = %#v", got.status, got.body)
		}
	}
}

func TestIssuerContextIsRequiredForSigning(t *testing.T) {
	issuer, err := New(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := issuer.issueToken(context.TODO(), "dev|user-1", "https://api.energyhub.local"); err != nil {
		t.Fatal(err)
	}
}
