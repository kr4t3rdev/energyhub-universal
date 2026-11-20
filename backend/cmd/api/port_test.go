package main

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestConfiguredListenAddr(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
		bad  bool
	}{
		{name: "Vercel port overrides local address", env: map[string]string{"PORT": "3000", "LISTEN_ADDR": "0.0.0.0:8080"}, want: ":3000"},
		{name: "local address", env: map[string]string{"LISTEN_ADDR": "127.0.0.1:8080"}, want: "127.0.0.1:8080"},
		{name: "default address", env: map[string]string{}, want: defaultListenAddr},
		{name: "invalid port", env: map[string]string{"PORT": "65536"}, bad: true},
		{name: "zero port", env: map[string]string{"PORT": "0"}, bad: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := configuredListenAddr(func(key string) string { return test.env[key] })
			if test.bad {
				if err == nil {
					t.Fatalf("configuredListenAddr() error = nil, want invalid port error")
				}
				return
			}
			if err != nil {
				t.Fatalf("configuredListenAddr() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("configuredListenAddr() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestVercelHandlerExposesHealthWithoutAuthConfig(t *testing.T) {
	t.Setenv("AUTH0_ISSUER", "")
	t.Setenv("AUTH0_AUDIENCE", "")
	t.Setenv("DATABASE_URL", "")

	vercelHandlerOnce = sync.Once{}
	vercelHandler = nil
	vercelHandlerErr = nil
	t.Cleanup(func() {
		vercelHandlerOnce = sync.Once{}
		vercelHandler = nil
		vercelHandlerErr = nil
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	Handler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("Handler(/health) status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}
