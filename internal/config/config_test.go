package config_test

import (
	"testing"

	"ssts/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	// Ensure no env vars are set
	t.Setenv("SSTS_LISTEN_ADDR", "")
	t.Setenv("SSTS_MAX_PAYLOAD_BYTES", "")
	t.Setenv("SSTS_TTL_SECONDS", "")
	t.Setenv("SSTS_RATE_LIMIT", "")

	cfg := config.Load()

	if cfg.ListenAddr != "0.0.0.0:8080" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, "0.0.0.0:8080")
	}
	if cfg.MaxPayloadBytes != 1048576 {
		t.Errorf("MaxPayloadBytes = %d, want 1048576", cfg.MaxPayloadBytes)
	}
	if cfg.TTLSeconds != 300 {
		t.Errorf("TTLSeconds = %d, want 300", cfg.TTLSeconds)
	}
	if cfg.RateLimit != 10 {
		t.Errorf("RateLimit = %d, want 10", cfg.RateLimit)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("SSTS_LISTEN_ADDR", "127.0.0.1:9090")
	t.Setenv("SSTS_MAX_PAYLOAD_BYTES", "524288")
	t.Setenv("SSTS_TTL_SECONDS", "60")
	t.Setenv("SSTS_RATE_LIMIT", "5")

	cfg := config.Load()

	if cfg.ListenAddr != "127.0.0.1:9090" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, "127.0.0.1:9090")
	}
	if cfg.MaxPayloadBytes != 524288 {
		t.Errorf("MaxPayloadBytes = %d, want 524288", cfg.MaxPayloadBytes)
	}
	if cfg.TTLSeconds != 60 {
		t.Errorf("TTLSeconds = %d, want 60", cfg.TTLSeconds)
	}
	if cfg.RateLimit != 5 {
		t.Errorf("RateLimit = %d, want 5", cfg.RateLimit)
	}
}

func TestLoad_InvalidIntFallsBackToDefault(t *testing.T) {
	t.Setenv("SSTS_TTL_SECONDS", "not-a-number")
	t.Setenv("SSTS_RATE_LIMIT", "abc")
	t.Setenv("SSTS_MAX_PAYLOAD_BYTES", "xyz")

	cfg := config.Load()

	if cfg.TTLSeconds != 300 {
		t.Errorf("TTLSeconds = %d, want default 300 for invalid input", cfg.TTLSeconds)
	}
	if cfg.RateLimit != 10 {
		t.Errorf("RateLimit = %d, want default 10 for invalid input", cfg.RateLimit)
	}
	if cfg.MaxPayloadBytes != 1048576 {
		t.Errorf("MaxPayloadBytes = %d, want default 1048576 for invalid input", cfg.MaxPayloadBytes)
	}
}
