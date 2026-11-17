// Command api menjalankan HTTP API FlowCBT (stateless): auth, tenant, exam,
// attempt, grading, report, billing, import/export, realtime.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/flowcbt/backend/internal/auth"
	"github.com/flowcbt/backend/internal/httpx"
	"github.com/flowcbt/backend/internal/platform"
	"github.com/go-chi/chi/v5"
)

func main() {
	cfg := platform.LoadConfig()
	logger := platform.NewLogger(cfg)

	r := httpx.NewRouter()
	r.Get("/healthz", httpx.Healthz)

	// /internal/active-exams untuk deploy freeze (PRD 7.4): jumlah ujian berjalan.
	r.Get("/internal/active-exams", func(w http.ResponseWriter, _ *http.Request) {
		// Diisi pada Fase 2 (S9) setelah repo attempts tersambung DB.
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"active_exams": 0})
	})

	r.Route("/api/v1", func(api chi.Router) {
		api.Get("/ping", httpx.Healthz)
		mountModules(api, cfg, logger)
	})

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("API berjalan", slog.String("addr", cfg.Addr), slog.String("env", cfg.Env))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("listen gagal", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("sinyal berhenti diterima, graceful shutdown")
	shCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shCtx); err != nil {
		logger.Error("shutdown gagal", slog.String("error", err.Error()))
	}
}

// mountModules memasang modul API per sprint; modul yang butuh DB membuka pool sendiri.
func mountModules(api chi.Router, cfg platform.Config, logger *slog.Logger) {
	ctx := context.Background()
	db, err := platform.NewDB(ctx, cfg.DatabaseURL, cfg.DatabaseSysURL)
	if err != nil {
		// Tanpa DB, endpoint auth tidak tersedia; API tetap hidup (healthz) untuk dev UI.
		logger.Warn("DB tidak tersedia, endpoint auth dilewati", slog.String("error", err.Error()))
		return
	}
	store := &auth.Store{System: db.System}
	svc := auth.NewService(store,
		auth.NewJWTManager(cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL),
		platform.RealClock())
	auth.NewHandler(svc).Mount(api)
	logger.Info("modul auth terpasang")
}
