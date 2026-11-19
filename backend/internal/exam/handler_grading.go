// Koreksi & penyelesaian nilai (S14, FR-5.7/8.2): rute guru untuk antrean
// koreksi, detail jawaban siswa, nilai manual per soal, override nilai
// otomatis dengan alasan, re-grade, dan rilis nilai ujian (scores_released_at).
package exam

import (
	"encoding/json"
	"net/http"

	"github.com/flowcbt/backend/internal/audit"
	"github.com/flowcbt/backend/internal/auth"
	"github.com/flowcbt/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
)

const gradeActionPrefix = "exam."

func (h *Handler) MountGrading(r chi.Router) {
	r.Get("/exams/{id}/grading", h.gradingQueue)
	r.Get("/attempts/{id}/grading", h.gradingDetail)
	r.Post("/attempts/{id}/grading/{questionID}", h.gradeAnswer)
	r.Post("/attempts/{id}/regrade", h.regrade)
	r.Post("/exams/{id}/release-scores", h.releaseScores)
}

func (h *Handler) gradingQueue(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	items, err := h.store.GradingQueue(r.Context(), c.TenantID, chi.URLParam(r, "id"))
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items)
}

func (h *Handler) gradingDetail(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	detail, err := h.store.GradingDetail(r.Context(), c.TenantID, chi.URLParam(r, "id"))
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, detail)
}

// gradeAnswer: POST /attempts/{id}/grading/{questionID}
// Bila soal sudah punya nilai otomatis → override (alasan wajib);
// bila esai kosong → nilai manual biasa. Agregasi ulang otomatis.
func (h *Handler) gradeAnswer(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	attemptID := chi.URLParam(r, "id")
	questionID := chi.URLParam(r, "questionID")
	var in gradeReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Score == nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("score wajib diisi", nil))
		return
	}
	in.GradedBy = c.UserID
	wasOverride, err := h.store.SetAnswerScore(r.Context(), c.TenantID, c.UserID, attemptID, questionID, &in)
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	action := gradeActionPrefix + "grade_manual"
	if wasOverride {
		action = gradeActionPrefix + "score_override"
	}
	h.audit.Log(r.Context(), audit.Entry{
		TenantID: c.TenantID, ActorType: "user", ActorID: c.UserID,
		Action: action, Entity: "attempt_answer",
		EntityID: attemptID + ":" + questionID,
		Meta:     map[string]any{"score": *in.Score, "reason": in.Reason, "override": wasOverride},
		IP:       r.RemoteAddr,
	})
	detail, err := h.store.GradingDetail(r.Context(), c.TenantID, attemptID)
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, detail)
}

func (h *Handler) regrade(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	n, err := h.store.RegradeAttempt(r.Context(), c.TenantID, chi.URLParam(r, "id"))
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	h.audit.Log(r.Context(), audit.Entry{
		TenantID: c.TenantID, ActorType: "user", ActorID: c.UserID,
		Action: gradeActionPrefix + "regrade", Entity: "attempt",
		EntityID: chi.URLParam(r, "id"),
		Meta:     map[string]any{"regraded": n},
		IP:       r.RemoteAddr,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]int{"regraded": n})
}

func (h *Handler) releaseScores(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	if err := h.store.ReleaseScores(r.Context(), c.TenantID, chi.URLParam(r, "id")); err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	h.audit.Log(r.Context(), audit.Entry{
		TenantID: c.TenantID, ActorType: "user", ActorID: c.UserID,
		Action: gradeActionPrefix + "release_scores", Entity: "exam",
		EntityID: chi.URLParam(r, "id"), IP: r.RemoteAddr,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "released"})
}
