// Command worker menjalankan job queue asynq (Redis): kirim email (S3),
// nanti: import/export besar, auto-submit, re-grade, retensi, rekonsiliasi
// pembayaran (PRD 7.1, 18.7).
package main

import (
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/hibiken/asynq"

	"github.com/flowcbt/backend/internal/notify"
	"github.com/flowcbt/backend/internal/platform"
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

	logger.Info("worker berjalan", "redis", redisAddr(cfg.RedisURL))
	if err := srv.Run(mux); err != nil {
		log.Fatalf("worker berhenti: %v", err)
	}
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
