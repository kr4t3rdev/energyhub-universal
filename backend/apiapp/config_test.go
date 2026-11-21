package apiapp

import "testing"

func TestLoadConfigKeepsExactCORSOrigin(t *testing.T) {
	values := map[string]string{
		"AUTH0_ISSUER":        "https://energyhub.eu.auth0.com/",
		"AUTH0_AUDIENCE":      "https://api.energyhub.example",
		"DATABASE_URL":        "postgres://runtime@127.0.0.1:5432/energyhub",
		"CORS_ALLOWED_ORIGIN": "https://energyhub.kr4t3rdev.xyz",
	}

	config, err := LoadConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if config.CORSAllowedOrigin != values["CORS_ALLOWED_ORIGIN"] {
		t.Fatalf("CORS allowed origin = %q, want %q", config.CORSAllowedOrigin, values["CORS_ALLOWED_ORIGIN"])
	}
}

func TestLoadConfigRejectsInvalidCORSOrigin(t *testing.T) {
	invalidOrigins := []string{
		"*",
		"null",
		"https://energyhub.kr4t3rdev.xyz/path",
		"https://user:pass@energyhub.kr4t3rdev.xyz",
		"ftp://energyhub.kr4t3rdev.xyz",
	}
	for _, origin := range invalidOrigins {
		t.Run(origin, func(t *testing.T) {
			values := map[string]string{"CORS_ALLOWED_ORIGIN": origin}
			if _, err := LoadConfig(func(key string) string { return values[key] }); err == nil {
				t.Fatalf("LoadConfig accepted invalid CORS origin %q", origin)
			}
		})
	}
}
