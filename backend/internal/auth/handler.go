package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"github.com/flowcbt/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
)

// Handler mengekspos endpoint auth (PRD 9):
//
//	POST /api/v1/auth/register        — FR-1.1
//	POST /api/v1/auth/verify-email    — verifikasi wajib sebelum akses penuh
//	POST /api/v1/auth/login           — FR-1.2 (lockout → 429)
//	POST /api/v1/auth/refresh         — rotasi refresh token (FR-1.6)
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Mount memasang rute auth pada grup /api/v1.
func (h *Handler) Mount(r chi.Router) {
	r.Post("/auth/register", h.register)
	r.Post("/auth/verify-email", h.verifyEmail)
	r.Post("/auth/login", h.login)
	r.Post("/auth/refresh", h.refresh)
}

type registerReq struct {
	Name       string `json:"name"`
	Email      string `json:"email"`
	Phone      string `json:"phone"`
	Password   string `json:"password"`
	TenantName string `json:"school_name"`
	TenantCode string `json:"school_code"`
	Level      string `json:"level"`
}

type registerRes struct {
	TenantID    string `json:"tenant_id"`
	UserID      string `json:"user_id"`
	Message     string `json:"message"`
	VerifyToken string `json:"verify_token,omitempty"` // hanya development (mailer datang di S3)
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("payload tidak valid", nil))
		return
	}
	res, err := h.svc.Register(r.Context(), RegisterInput{
		TenantName: req.TenantName,
		TenantCode: req.TenantCode,
		Level:      req.Level,
		OwnerName:  req.Name,
		Email:      req.Email,
		Phone:      req.Phone,
	}, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, ErrEmailTaken):
			httpx.ErrorWriter(w, r, httpx.AppErr(409, "EMAIL_TAKEN", "email sudah terdaftar", nil))
		case errors.Is(err, ErrTenantCodeUsed):
			httpx.ErrorWriter(w, r, httpx.AppErr(409, "TENANT_CODE_TAKEN", "kode sekolah sudah dipakai", nil))
		default:
			httpx.ErrorWriter(w, r, httpx.ErrBadRequest(err.Error(), nil))
		}
		return
	}
	body := registerRes{
		TenantID: res.TenantID,
		UserID:   res.UserID,
		Message:  "registrasi berhasil. Buka email Anda untuk verifikasi (tautan berlaku 24 jam).",
	}
	if os.Getenv("ENV") != "production" {
		body.VerifyToken = res.VerifyURLRaw
	}
	httpx.WriteJSON(w, http.StatusCreated, body)
}

type verifyEmailReq struct {
	Token string `json:"token"`
}

func (h *Handler) verifyEmail(w http.ResponseWriter, r *http.Request) {
	var req verifyEmailReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("token wajib diisi", nil))
		return
	}
	if err := h.svc.VerifyEmail(r.Context(), req.Token); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.ErrorWriter(w, r, httpx.AppErr(400, "TOKEN_INVALID", "token tidak valid atau kedaluwarsa", nil))
			return
		}
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "email terverifikasi. Silakan login."})
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("payload tidak valid", nil))
		return
	}
	tok, err := h.svc.Login(r.Context(), req.Email, req.Password)
	var locked *LockedError
	switch {
	case errors.As(err, &locked):
		httpx.ErrorWriter(w, r, httpx.AppErr(429, "ACCOUNT_LOCKED", err.Error(), nil))
	case err != nil:
		httpx.ErrorWriter(w, r, httpx.ErrUnauthorized("INVALID_CREDENTIALS", err.Error()))
	default:
		httpx.WriteJSON(w, http.StatusOK, tok)
	}
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("refresh_token wajib diisi", nil))
		return
	}
	tok, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrUnauthorized("TOKEN_INVALID", "refresh token tidak valid"))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, tok)
}
