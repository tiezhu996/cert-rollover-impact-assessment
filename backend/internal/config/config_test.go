package config

import "testing"

func TestConfigLoadsPositiveRateLimits(t *testing.T) {
	for _, key := range []string{"LOGIN_LIMIT_PER_MINUTE", "CERT_IMPORT_LIMIT_PER_MINUTE", "SIMULATION_LIMIT_PER_MINUTE"} {
		t.Setenv(key, "")
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LoginLimitPerMinute <= 0 || cfg.CertificateLimitPerMinute <= 0 || cfg.SimulationLimitPerMinute <= 0 {
		t.Fatalf("rate limits must be positive: %+v", cfg)
	}
}

func TestConfigLoadsPositiveJWTExpiry(t *testing.T) {
	t.Setenv("JWT_EXPIRY", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JWTExpiry <= 0 {
		t.Fatalf("JWT expiry must be positive: %v", cfg.JWTExpiry)
	}
}
