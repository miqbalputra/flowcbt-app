// Package platform menyediakan infrastruktur lintas modul: config, logger, db, redis,
// storage, mailer, metrics, clock, idgen. Setiap komponen dipisah file sendiri.
package platform

import (
	"context"
	"log/slog"
	"os"
	"time"
)

// Config memuat seluruh konfigurasi runtime dari environment variable.
type Config struct {
	Addr            string // alamat listen API, mis. ":8080"
	DatabaseURL     string // postgres://... (role app yang tunduk RLS)
	DatabaseSysURL  string // postgres://flowcbt_system:... (BYPASSRLS)
	RedisURL        string
	JWTSecret       string
	JWTAccessTTL    time.Duration // token akses singkat
	JWTRefreshTTL   time.Duration
	Env             string // development | staging | production
	LogLevel        slog.Level
	ShutdownTimeout time.Duration
}

// LoadConfig membaca konfigurasi dari environment dengan default yang aman untuk dev.
func LoadConfig() Config {
	cfg := Config{
		Addr:            env("ADDR", ":8080"),
		DatabaseURL:     env("DATABASE_URL", "postgres://flowcbt_app:flowcbt_app@localhost:5432/flowcbt?sslmode=disable"),
		DatabaseSysURL:  env("DATABASE_SYS_URL", ""),
		RedisURL:        env("REDIS_URL", "redis://localhost:6379/0"),
		JWTSecret:       env("JWT_SECRET", "dev-only-secret-change-me"),
		JWTAccessTTL:    envDuration("JWT_ACCESS_TTL", 15*time.Minute),
		JWTRefreshTTL:   envDuration("JWT_REFRESH_TTL", 30*24*time.Hour),
		Env:             env("ENV", "development"),
		ShutdownTimeout: envDuration("SHUTDOWN_TIMEOUT", 30*time.Second),
	}
	switch env("LOG_LEVEL", "info") {
	case "debug":
		cfg.LogLevel = slog.LevelDebug
	case "warn":
		cfg.LogLevel = slog.LevelWarn
	case "error":
		cfg.LogLevel = slog.LevelError
	default:
		cfg.LogLevel = slog.LevelInfo
	}
	return cfg
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

// NewLogger membuat logger terstruktur (JSON saat production, teks saat dev).
func NewLogger(cfg Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	if cfg.Env == "production" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts)).
			With(slog.String("service", "flowcbt-api"))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts)).
		With(slog.String("service", "flowcbt-api"))
}

// Clock dipisah agar logika waktu (deadline attempt, retensi) mudah diuji.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

// RealClock mengembalikan jam sistem UTC.
func RealClock() Clock { return realClock{} }

// Shutdown menunggu ctx selesai atau timeout lalu menutup resource.
type Closer func(context.Context) error

// RunUntilSignal helper sederhana: dibutuhkan cmd untuk menahan proses.
func RunForever(ctx context.Context) (done <-chan struct{}) {
	ch := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch
}
