package config

import "testing"

func TestSwaggerIsDisabledByDefault(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("SERVICE_NAME", "policy-service")
	t.Setenv("SWAGGER_ENABLED", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTP.SwaggerEnabled {
		t.Fatal("swagger must be off unless SWAGGER_ENABLED is true")
	}
}

func TestSwaggerEnabledParsesBool(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("SERVICE_NAME", "policy-service")
	for value, want := range map[string]bool{"true": true, "TRUE": true, "1": true, "false": false, "0": false} {
		t.Setenv("SWAGGER_ENABLED", value)
		cfg, err := Load()
		if err != nil || cfg.HTTP.SwaggerEnabled != want {
			t.Errorf("SWAGGER_ENABLED=%s: enabled = %v, err = %v, want %v", value, cfg != nil && cfg.HTTP.SwaggerEnabled, err, want)
		}
	}
	t.Setenv("SWAGGER_ENABLED", "yes")
	if _, err := Load(); err == nil {
		t.Error("SWAGGER_ENABLED=yes must be rejected")
	}
}

func TestCORSOriginsAreAList(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("SERVICE_NAME", "policy-service")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000, https://cms.example")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.HTTP.CORSOrigins) != 2 || cfg.HTTP.CORSOrigins[1] != "https://cms.example" {
		t.Fatalf("CORSOrigins = %#v", cfg.HTTP.CORSOrigins)
	}
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	if cfg, _ := Load(); len(cfg.HTTP.CORSOrigins) != 0 {
		t.Fatal("CORS must be off by default")
	}
}
