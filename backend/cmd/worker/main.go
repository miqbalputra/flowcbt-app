// Command worker menjalankan job queue asynq (Redis): kirim email (S3),
// nanti: import/export besar, auto-submit, re-grade, retensi, rekonsiliasi
// pembayaran (PRD 7.1, 18.7).
package main

import (
	"context"
	"log"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"github.com/flowcbt/backend/internal/exam"
	"github.com/flowcbt/backend/internal/notify"
	"github.com/flowcbt/backend/internal/platform"
	"github.com/flowcbt/backend/internal/questionbank"
)

func main() {
	cfg := platform.LoadConfig()
	logger := platform.NewLogger(cfg)
	mailer := platform.NewMailerFromEnv()

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr(cfg.RedisURL)},
		asynq.Config{Concurrency: 10},
	)
	mux := asynq.NewServeMux()
	mux.HandleFunc(notify.TypeEmail, notify.HandleEmailTask(mailer))

	// Pemindai auto-submit (S13): attempt lewat deadline ditutup & dinilai.
	// Ticker sederhana; klaim atomic UPDATE..RETURNING aman multi-instance.
	if db, err := platform.NewDB(context.Background(), cfg.DatabaseURL, cfg.DatabaseSysURL); err == nil {
		defer db.Close()
		go autoSubmitLoop(context.Background(), db, logger)
	} else {
		logger.Warn("DB tidak tersedia, auto-submit dinonaktifkan", slog.String("error", err.Error()))
	}

	logger.Info("worker berjalan", "redis", redisAddr(cfg.RedisURL))
	if err := srv.Run(mux); err != nil {
		log.Fatalf("worker berhenti: %v", err)
	}
}

// autoSubmitLoop: scan tiap 30 detik (PRD 18.7); satu anggota attempt
// dinilai via mesin grading sebelum status graded/awaiting_manual_grading.
func autoSubmitLoop(ctx context.Context, db *platform.DB, logger *slog.Logger) {
	store := exam.NewStore(db, questionbank.NewStore(db), nil)
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			n, err := store.AutoSubmitExpired(ctx, 25)
			if err != nil {
				if !isCtxDone(ctx, err) {
					logger.Warn("auto-submit gagal", slog.String("error", err.Error()))
				}
				continue
			}
			if n > 0 {
				logger.Info("auto-submit", slog.Int("attempts", n))
			}
		}
	}
}

func isCtxDone(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	return ctx.Err() != nil || strings.Contains(err.Error(), context.Canceled.Error())
}

// redisAddr mengubah redis://host:port/db menjadi host:port untuk asynq.
func redisAddr(redisURL string) string {
	u := strings.TrimPrefix(redisURL, "redis://")
	host := u
	if i := strings.Index(u, "/"); i >= 0 {
		host = strings.Split(u[:i], "@")[0] // buang remote path & credentials
	}
	if host == "" {
		host = "localhost:6379"
	}
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = net.JoinHostPort(host, strconv.Itoa(6379))
	}
	return host
}
