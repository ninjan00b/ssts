package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ssts/internal/api"
	"ssts/internal/config"
	"ssts/internal/store"
	sststweb "ssts/web"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()
	ttl := time.Duration(cfg.TTLSeconds) * time.Second

	s := store.New(ttl)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.StartCleanup(ctx, 30*time.Second)

	mux := http.NewServeMux()

	mux.Handle("GET /", http.FileServerFS(sststweb.FS))

	rateLimitMW := api.RateLimitMiddleware(cfg.RateLimit)
	maxBodyMW := api.MaxBodySize(cfg.MaxPayloadBytes + 512*1024) // extra headroom for base64 overhead

	mux.Handle("POST /upload", rateLimitMW(maxBodyMW(api.UploadHandler(s, cfg.MaxPayloadBytes))))
	mux.Handle("GET /fetch/{code}", rateLimitMW(api.FetchHandler(s)))

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	slog.Info("server starting",
		"addr", cfg.ListenAddr,
		"ttl_seconds", cfg.TTLSeconds,
		"max_payload_bytes", cfg.MaxPayloadBytes,
		"rate_limit_per_min", cfg.RateLimit,
	)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		slog.Info("shutting down")
		cancel()
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutCancel()
		srv.Shutdown(shutCtx)
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
