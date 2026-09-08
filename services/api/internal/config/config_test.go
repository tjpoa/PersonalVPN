package config

import "testing"

func TestLoadRequiresDatabaseURL(t *testing.T) {
	_, err := Load(func(string) string { return "" })
	if err == nil || err.Error() != "DATABASE_URL is required" {
		t.Fatalf("Load() error = %v, want DATABASE_URL is required", err)
	}
}

func TestLoadDefaultsAndTrimsValues(t *testing.T) {
	values := map[string]string{"DATABASE_URL": " postgres://localhost/test ", "APP_ENV": " production "}
	config, err := Load(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if config.ListenAddress != ":8080" || config.DatabaseURL != "postgres://localhost/test" || config.Environment != "production" {
		t.Fatalf("Load() = %+v", config)
	}
}

func TestLoadRejectsNewlines(t *testing.T) {
	_, err := Load(func(key string) string {
		if key == "DATABASE_URL" {
			return "postgres://localhost/test\n"
		}
		return ""
	})
	if err == nil || err.Error() != "runtime configuration contains newline" {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadRejectsPartialGatewayTLS(t *testing.T) {
	values := map[string]string{"DATABASE_URL": "postgres://localhost/test", "GATEWAY_TLS_CERT_FILE": "cert.pem"}
	_, err := Load(func(key string) string { return values[key] })
	if err == nil || err.Error() != "gateway TLS requires certificate, key, and CA files" {
		t.Fatalf("Load() error = %v", err)
	}
}
