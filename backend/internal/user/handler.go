package user

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/flowcbt/backend/internal/audit"
	"github.com/flowcbt/backend/internal/auth"
	"github.com/flowcbt/backend/internal/httpx"
	"github.com/flowcbt/backend/internal/notify"
	"github.com/flowcbt/backend/internal/platform"
	"github.com/go-chi/chi/v5"
)

// Service menggabungkan store + mailer + audit untuk operasi guru & reset password.
type Service struct {
	store *Store
	queue *notify.Queue
	audit *audit.Writer
	clock platform.Clock
}

func NewService(store *Store, queue *notify.Queue, auditw *audit.Writer, clock platform.Clock) *Service {
	return &Service{store: store, queue: queue, audit: auditw, clock: clock}
}

func (s *Service) TeachersOfTenant(ctx context.Context, tenantID string) ([]TeacherRow, error) {
	if tenantID == "" {
		return nil, httpx.AppErr(400, "NO_TENANT", "akun super_admin tidak terikat tenant", nil)
	}
	return s.store.TeachersOfTenant(ctx, tenantID)
}

func (s *Service) InviteTeacher(ctx context.Context, tenantID, actorID, name, email, phone, ip string) (string, string, error) {
	if strings.TrimSpace(name) == "" || !strings.Contains(email, "@") {
		return "", "", httpx.ErrBadRequest("nama dan email guru wajib diisi", nil)
	}
	userID, raw, err := s.store.CreateInvitedGuru(ctx, tenantID, name, email, phone, s.clock.Now())
	if err != nil {
		return "", "", err
	}
	if err := s.queue.SendEmail(ctx, notify.EmailJob{
		To:      email,
		Subject: "Undangan FlowCBT",
		HTML:    notify.TeacherInviteHTML("sekolah Anda", raw),
	}); err != nil {
		return "", "", err
	}
	s.audit.Log(ctx, audit.Entry{
		TenantID: tenantID, ActorType: "user", ActorID: actorID,
		Action: "teacher.invite", Entity: "user", EntityID: userID,
		Meta: map[string]any{"email": email}, IP: ip,
	})
	return userID, raw, nil
}

func (s *Service) SetTeacherStatus(ctx context.Context, tenantID, actorID, userID, status, ip string) error {
	if status != "active" && status != "disabled" {
		return httpx.ErrBadRequest("status harus active|disabled", nil)
	}
	err := s.store.SetTeacherStatus(ctx, tenantID, userID, status)
	if errors.Is(err, errNoRows) {
		return httpx.ErrNotFound()
	}
	if err != nil {
		return err
	}
	s.audit.Log(ctx, audit.Entry{
		TenantID: tenantID, ActorType: "user", ActorID: actorID,
		Action: "teacher.status." + status, Entity: "user", EntityID: userID, IP: ip,
	})
	return nil
}

// ForgotPassword selalu netral terhadap keberadaan email (anti-enum, FR-1.2).
func (s *Service) ForgotPassword(ctx context.Context, email string) error {
	raw, ok, err := s.store.CreateResetToken(ctx, email, s.clock.Now())
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return s.queue.SendEmail(ctx, notify.EmailJob{
		To:      email,
		Subject: "Reset password FlowCBT",
		HTML:    notify.ResetPasswordHTML(raw),
	})
}

func (s *Service) ResetPassword(ctx context.Context, rawToken, password string) error {
	if err := auth.PasswordPolicyError(password); err != nil {
		return httpx.ErrBadRequest(err.Error(), nil)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return httpx.ErrBadRequest(err.Error(), nil)
	}
	if err := s.store.ConsumeResetToken(ctx, rawToken, hash, s.clock.Now()); err != nil {
		if errors.Is(err, errNoRows) {
			return httpx.AppErr(400, "TOKEN_INVALID", "token reset tidak valid atau kedaluwarsa", nil)
		}
		return err
	}
	s.audit.Log(ctx, audit.Entry{
		ActorType: "system", Action: "password.reset", Entity: "user",
	})
	return nil
}

// ---- Handler HTTP ----

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// MountTenant: rute guru (dipasang di dalam grup yang telah ber-auth + RBAC).
func (h *Handler) MountTenant(r chi.Router) {
	r.Get("/teachers", h.list)
	r.Post("/teachers/invite", h.invite)
	r.Delete("/teachers/{id}", h.deactivate)
}

// MountPublic: reset password tanpa auth (rate limit ketat terpasang di pemanggil).
func (h *Handler) MountPublic(r chi.Router) {
	r.Post("/auth/forgot-password", h.forgotPassword)
	r.Post("/auth/reset-password", h.resetPassword)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	claims := auth.FromContext(r.Context())
	list, err := h.svc.TeachersOfTenant(r.Context(), claims.TenantID)
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

type inviteReq struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

func (h *Handler) invite(w http.ResponseWriter, r *http.Request) {
	var req inviteReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("payload tidak valid", nil))
		return
	}
	claims := auth.FromContext(r.Context())
	userID, raw, err := h.svc.InviteTeacher(r.Context(), claims.TenantID, claims.UserID,
		req.Name, req.Email, req.Phone, r.RemoteAddr)
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	res := map[string]string{"id": userID, "message": "undangan terkirim. Guru mengaktifkan akun lewat email."}
	if platform.NonProduction() {
		res["invite_token"] = raw // dev: mailer mungkin belum tersedia
	}
	httpx.WriteJSON(w, http.StatusCreated, res)
}

func (h *Handler) deactivate(w http.ResponseWriter, r *http.Request) {
	claims := auth.FromContext(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.svc.SetTeacherStatus(r.Context(), claims.TenantID, claims.UserID, id, "disabled", r.RemoteAddr); err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "guru dinonaktifkan"})
}

type emailReq struct {
	Email string `json:"email"`
}

func (h *Handler) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var req emailReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !strings.Contains(req.Email, "@") {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("email tidak valid", nil))
		return
	}
	_ = h.svc.ForgotPassword(r.Context(), req.Email) // selalu 202 — anti-enum
	httpx.WriteJSON(w, http.StatusAccepted, map[string]string{
		"message": "bila email terdaftar, kode reset telah dikirim",
	})
}

type resetReq struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (h *Handler) resetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("token & password wajib", nil))
		return
	}
	if err := h.svc.ResetPassword(r.Context(), req.Token, req.Password); err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "password diperbarui. Silakan login."})
}
