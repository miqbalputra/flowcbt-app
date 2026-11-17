// Package httpx berisi router, middleware, dan konvensi HTTP API
// (format error {code,message,details}, request id, recover, pagination).
package httpx

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Error adalah bentuk error standar API (PRD Bagian 9): {code, message, details}.
// Kode stabil seperti QUOTA_EXCEEDED, ATTEMPT_EXPIRED, TOKEN_INVALID.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// HTTPError menggabungkan Error dengan status HTTP.
type HTTPError struct {
	Status int
	Err    Error
}

func (e *HTTPError) Error() string { return e.Err.Code + ": " + e.Err.Message }

// AppErr membuat HTTPError baru.
func AppErr(status int, code, message string, details any) *HTTPError {
	return &HTTPError{Status: status, Err: Error{Code: code, Message: message, Details: details}}
}

// ErrNotFound untuk resource tidak ditemukan tanpa membocorkan keberadaan
// (dipakai pula pelanggaran isolasi tenant → 404, AC-9).
func ErrNotFound() *HTTPError {
	return AppErr(http.StatusNotFound, "NOT_FOUND", "resource tidak ditemukan", nil)
}

// ErrBadRequest untuk payload tidak valid.
func ErrBadRequest(message string, details any) *HTTPError {
	return AppErr(http.StatusBadRequest, "BAD_REQUEST", message, details)
}

// ErrUnauthorized untuk kredensial hilang/tidak valid.
func ErrUnauthorized(code, message string) *HTTPError {
	return AppErr(http.StatusUnauthorized, code, message, nil)
}

// WriteJSON menulis response JSON.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// ErrorWriter menangkap error dari handler lalu memformat sesuai standar.
func ErrorWriter(w http.ResponseWriter, r *http.Request, err error) {
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		httpErr = AppErr(http.StatusInternalServerError, "INTERNAL", "terjadi kesalahan internal", nil)
	}
	WriteJSON(w, httpErr.Status, httpErr.Err)
}

// NewRouter memasang middleware dasar: request id, real IP, recover, logger ringkas.
func NewRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	return r
}

// Healthz endpoint liveness untuk Coolify.
func Healthz(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
