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
	"strings"
	"syscall"
	"time"

	"github.com/flowcbt/backend/internal/audit"
	"github.com/flowcbt/backend/internal/auth"
	"github.com/flowcbt/backend/internal/classroom"
	"github.com/flowcbt/backend/internal/httpx"
	"github.com/flowcbt/backend/internal/notify"
	"github.com/flowcbt/backend/internal/platform"
	"github.com/flowcbt/backend/internal/questionbank"
	"github.com/flowcbt/backend/internal/student"
	"github.com/flowcbt/backend/internal/user"
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

// mountModules memasang seluruh modul. Autentikasi ke DB/Redis gagal → modul
// terkait dilewati dengan peringatan; healthz tetap hijau (dev tanpa infra).
func mountModules(api chi.Router, cfg platform.Config, logger *slog.Logger) {
	ctx := context.Background()
	db, err := platform.NewDB(ctx, cfg.DatabaseURL, cfg.DatabaseSysURL)
	if err != nil {
		logger.Warn("DB tidak tersedia, endpoint tenant dilewati", slog.String("error", err.Error()))
		return
	}

	clock := platform.RealClock()
	jwtm := auth.NewJWTManager(cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)
	queue := notify.NewQueue(redisAddr(cfg.RedisURL), platform.NewMailerFromEnv())
	auditw := audit.NewWriter(db.System)

	// Publik + rate limit ketat: login/register/reset (PRD 9, anti brute force).
	strict := httpx.NewRateLimit(10, time.Minute)
	api.Group(func(pub chi.Router) {
		pub.Use(strict.Middleware)
		auth.NewHandler(auth.NewService(&auth.Store{System: db.System}, jwtm, clock)).Mount(pub)
		user.NewHandler(user.NewService(user.NewStore(db.System), queue, auditw, clock)).MountPublic(pub)
	})

	// Terproteksi JWT: modul tenant. RBAC di satu tempat: owner/guru boleh
	// kelas & siswa (matriks 2.1); detil per-kelas guru disempurnakan di S5+.
	api.Group(func(prot chi.Router) {
		prot.Use(auth.Middleware(jwtm))
		prot.Use(auth.RequireRole("owner", "teacher"))
		classroomHandler := classroom.NewHandler(classroom.NewStore(db), auditw)
		classroomHandler.Mount(prot)
		student.NewHandler(student.NewStore(db), auditw).Mount(prot)
		questionbank.NewHandler(questionbank.NewStore(db), platform.NewStorage(), auditw).Mount(prot)
		user.NewHandler(user.NewService(user.NewStore(db.System), queue, auditw, clock)).MountTenant(prot)
	})

	logger.Info("modul terpasang", slog.String("modules", "auth,user,classroom,student,questionbank"))
}

// redisAddr mengubah redis://host:port/db menjadi host:port.
func redisAddr(u string) string {
	h := strings.TrimPrefix(u, "redis://")
	if i := strings.Index(h, "/"); i >= 0 {
		h = h[:i]
	}
	if h == "" {
		h = "localhost:6379"
	}
	return h
}
