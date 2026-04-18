package config

import (
	"os"
	"strconv"
)

type Config struct {
	ListenAddr      string
	MaxPayloadBytes int64
	TTLSeconds      int
	RateLimit       int
}

func Load() Config {
	return Config{
		ListenAddr:      getEnv("SSTS_LISTEN_ADDR", "0.0.0.0:8080"),
		MaxPayloadBytes: getEnvInt64("SSTS_MAX_PAYLOAD_BYTES", 1048576),
		TTLSeconds:      getEnvInt("SSTS_TTL_SECONDS", 300),
		RateLimit:       getEnvInt("SSTS_RATE_LIMIT", 10),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}
