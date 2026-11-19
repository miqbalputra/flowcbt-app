// Handler HTTP ujian (S9): CRUD, ruang, token, penugasan & peserta.
// Aksi pengawas (tambah waktu, force submit, dst.) menyusul di S10.
package exam

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/flowcbt/backend/internal/audit"
	"github.com/flowcbt/backend/internal/auth"
	"github.com/flowcbt/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	store *Store
	audit *audit.Writer
	auth  *auth.Service
}

func NewHandler(store *Store, aw *audit.Writer, as *auth.Service) *Handler {
	return &Handler{store: store, audit: aw, auth: as}
}

// Mount dipasang pada grup JWT terproteksi (owner/teacher).
func (h *Handler) Mount(r chi.Router) {
	r.Get("/exams", h.list)
	r.Post("/exams", h.create)
	r.Get("/exams/{id}", h.get)
	r.Put("/exams/{id}", h.update)
	r.Delete("/exams/{id}", h.remove)

	// Ruang & token (FR-4.9).
	r.Get("/exams/{id}/rooms", h.listRooms)
	r.Post("/exams/{id}/rooms", h.addRoom)
	r.Get("/exams/{id}/tokens", h.listTokens)
	r.Post("/exams/{id}/tokens", h.issueToken)
	r.Delete("/exam-tokens/{token}", h.revokeToken)

	// Penugasan & peserta (FR-5.6).
	r.Put("/exams/{id}/assignments", h.setAssignments)
	r.Get("/exams/{id}/assignments", h.listAssignments)
	r.Get("/exams/{id}/participants", h.listParticipants)

	// Siklus status & aksi pengawas (S10, FR-5.7).
	r.Post("/exams/{id}/publish", h.publish)
	r.Post("/exams/{id}/status", h.setStatus)
	r.Post("/exams/{id}/tokens/regenerate", h.regenerateTokens)
	r.Post("/exams/{id}/participants/{studentID}/add-time", h.addTime)
	r.Post("/exams/{id}/participants/{studentID}/reset-attempts", h.resetAttempts)
	r.Post("/exams/{id}/participants/{studentID}/force-submit", h.forceSubmit)
	r.Post("/exams/{id}/participants/{studentID}/kick", h.kickSession)

	// Koreksi & penyelesaian nilai (S14).
	h.MountGrading(r)
}

// MountPublic: rute tanpa JWT — login tamu token (grup publik ber-rate-limit).
func (h *Handler) MountPublic(r chi.Router) {
	r.Post("/exams/guest-auth", h.guestAuth)
}

// MountStudent: rute siswa ber-JWT (grup auth.Middleware + RequireRole("student")).
func (h *Handler) MountStudent(r chi.Router) {
	r.Get("/me/exams", h.myExams)
	r.Post("/exams/{id}/start", h.start)
	r.Get("/attempts/{id}", h.getAttempt)
	r.Post("/attempts/{id}/answers:sync", h.syncAnswers)
	r.Post("/attempts/{id}/submit", h.submit)
	r.Get("/attempts/{id}/result", h.result)
}

// submit: siswa mengirim jawaban → grading → hasil.
func (h *Handler) submit(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	var in submitReq
	_ = json.NewDecoder(r.Body).Decode(&in)
	view, err := h.store.SubmitAttempt(r.Context(), c.TenantID, c.StudentID, chi.URLParam(r, "id"), in.Reason)
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	h.audit.Log(r.Context(), audit.Entry{
		TenantID: c.TenantID, ActorType: "student", ActorID: c.StudentID,
		Action: "attempt.submit", Entity: "attempt", EntityID: view.AttemptID, IP: r.RemoteAddr,
	})
	httpx.WriteJSON(w, http.StatusOK, view)
}

// result: hasil attempt siswa (gating FR-5.4).
func (h *Handler) result(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	res, err := h.store.GetResult(r.Context(), c.TenantID, c.StudentID, chi.URLParam(r, "id"))
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

type guestReq struct {
	Token string `json:"token"`
}

func (h *Handler) guestAuth(w http.ResponseWriter, r *http.Request) {
	var in guestReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Token == "" {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("token wajib diisi", nil))
		return
	}
	tenantID, studentID, examID, err := h.store.GuestAuth(r.Context(), in.Token)
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	tokens, err := h.auth.IssueForStudent(r.Context(), tenantID, studentID)
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"exam_id":       examID,
		"access_token":  tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
		"expires_in":    tokens.ExpiresIn,
	})
}

func (h *Handler) myExams(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	exams, err := h.store.ListStudentExams(r.Context(), c.TenantID, c.StudentID)
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, exams)
}

type startReq struct {
	DeviceHash string `json:"device_hash"`
}

// start: siswa mulai/lanjutkan ujian (AC-6 → 409 bila batas tercapai).
func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	var in startReq
	_ = json.NewDecoder(r.Body).Decode(&in)
	if in.DeviceHash == "" {
		in.DeviceHash = "web"
	}
	res, err := h.store.StartAttempt(r.Context(), c.TenantID, c.StudentID, chi.URLParam(r, "id"), in.DeviceHash)
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

// syncAnswers: kirim jawaban batch (S12) — dipanggil berkala klien.
func (h *Handler) syncAnswers(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	var in SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("payload tidak valid", nil))
		return
	}
	res, err := h.store.SyncAnswers(r.Context(), c.TenantID, c.StudentID, chi.URLParam(r, "id"), in)
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

// getAttempt: muat ulang pengerjaan (refresh/sesi baru).
func (h *Handler) getAttempt(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	st, err := h.store.GetAttemptState(r.Context(), c.TenantID, c.StudentID, chi.URLParam(r, "id"))
	if err != nil {
		httpx.ErrorWriter(w, r, notFoundOrErr(err))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, st)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	exams, err := h.store.List(r.Context(), c.TenantID,
		r.URL.Query().Get("q"), r.URL.Query().Get("status"), limit, offset)
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, exams)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	var in ExamInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("payload tidak valid", nil))
		return
	}
	e, err := h.store.Create(r.Context(), c.TenantID, c.UserID, in)
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	h.audit.Log(r.Context(), audit.Entry{
		TenantID: c.TenantID, ActorType: "user", ActorID: c.UserID,
		Action: "exam.create", Entity: "exam", EntityID: e.ID, IP: r.RemoteAddr,
	})
	httpx.WriteJSON(w, http.StatusCreated, e)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	e, err := h.store.Get(r.Context(), c.TenantID, chi.URLParam(r, "id"))
	if err != nil {
		httpx.ErrorWriter(w, r, notFoundOrErr(err))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, e)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	var in ExamUpdate
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("payload tidak valid", nil))
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.store.Update(r.Context(), c.TenantID, id, in); err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	h.audit.Log(r.Context(), audit.Entry{
		TenantID: c.TenantID, ActorType: "user", ActorID: c.UserID,
		Action: "exam.update", Entity: "exam", EntityID: id, IP: r.RemoteAddr,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "ujian diperbarui"})
}

func (h *Handler) remove(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.store.Delete(r.Context(), c.TenantID, id); err != nil {
		httpx.ErrorWriter(w, r, notFoundOrErr(err))
		return
	}
	h.audit.Log(r.Context(), audit.Entry{
		TenantID: c.TenantID, ActorType: "user", ActorID: c.UserID,
		Action: "exam.delete", Entity: "exam", EntityID: id, IP: r.RemoteAddr,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "ujian draft dihapus"})
}

// ---- ruang & token ----

type roomReq struct {
	Name string `json:"name"`
}

func (h *Handler) addRoom(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	var in roomReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("payload tidak valid", nil))
		return
	}
	room, err := h.store.AddRoom(r.Context(), c.TenantID, chi.URLParam(r, "id"), in.Name)
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, room)
}

func (h *Handler) listRooms(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	rooms, err := h.store.ListRooms(r.Context(), c.TenantID, chi.URLParam(r, "id"))
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, rooms)
}

type tokenReq struct {
	RoomID string `json:"room_id"`
}

func (h *Handler) issueToken(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	var in tokenReq
	_ = json.NewDecoder(r.Body).Decode(&in) // body opsional
	var roomID *string
	if in.RoomID != "" {
		roomID = &in.RoomID
	}
	t, err := h.store.IssueToken(r.Context(), c.TenantID, chi.URLParam(r, "id"), roomID)
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	h.audit.Log(r.Context(), audit.Entry{
		TenantID: c.TenantID, ActorType: "user", ActorID: c.UserID,
		Action: "exam.token_issue", Entity: "exam", EntityID: t.ExamID, IP: r.RemoteAddr,
	})
	httpx.WriteJSON(w, http.StatusCreated, t)
}

func (h *Handler) listTokens(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	tokens, err := h.store.ListTokens(r.Context(), c.TenantID, chi.URLParam(r, "id"))
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, tokens)
}

func (h *Handler) revokeToken(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	token := chi.URLParam(r, "token")
	if err := h.store.RevokeToken(r.Context(), c.TenantID, token); err != nil {
		httpx.ErrorWriter(w, r, notFoundOrErr(err))
		return
	}
	h.audit.Log(r.Context(), audit.Entry{
		TenantID: c.TenantID, ActorType: "user", ActorID: c.UserID,
		Action: "exam.token_revoke", Entity: "exam_token", EntityID: token, IP: r.RemoteAddr,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "token dinonaktifkan"})
}

// ---- penugasan & peserta ----

func (h *Handler) setAssignments(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	var in struct {
		Assignments []Assignment `json:"assignments"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("payload tidak valid", nil))
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.store.SetAssignments(r.Context(), c.TenantID, id, in.Assignments); err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "penugasan & peserta diperbarui"})
}

func (h *Handler) listAssignments(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	list, err := h.store.ListAssignments(r.Context(), c.TenantID, chi.URLParam(r, "id"))
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

// ---- siklus status & aksi pengawas (S10) ----

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	id := chi.URLParam(r, "id")
	e, err := h.store.Publish(r.Context(), c.TenantID, id, c.UserID)
	if err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest(err.Error(), nil))
		return
	}
	h.audit.Log(r.Context(), audit.Entry{
		TenantID: c.TenantID, ActorType: "user", ActorID: c.UserID,
		Action: "exam.publish", Entity: "exam", EntityID: id, IP: r.RemoteAddr,
	})
	httpx.WriteJSON(w, http.StatusOK, e)
}

type statusReq struct {
	Action string `json:"action"` // pause | resume | close | cancel
}

func (h *Handler) setStatus(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	var in statusReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("payload tidak valid", nil))
		return
	}
	id := chi.URLParam(r, "id")
	e, err := h.store.SetStatus(r.Context(), c.TenantID, id, in.Action)
	if err != nil {
		httpx.ErrorWriter(w, r, badReqOrNotFound(err))
		return
	}
	h.audit.Log(r.Context(), audit.Entry{
		TenantID: c.TenantID, ActorType: "user", ActorID: c.UserID,
		Action: "exam." + in.Action, Entity: "exam", EntityID: id, IP: r.RemoteAddr,
	})
	httpx.WriteJSON(w, http.StatusOK, e)
}

func (h *Handler) regenerateTokens(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	var in tokenReq
	_ = json.NewDecoder(r.Body).Decode(&in)
	var roomID *string
	if in.RoomID != "" {
		roomID = &in.RoomID
	}
	id := chi.URLParam(r, "id")
	t, err := h.store.RegenerateTokens(r.Context(), c.TenantID, id, roomID)
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	h.audit.Log(r.Context(), audit.Entry{
		TenantID: c.TenantID, ActorType: "user", ActorID: c.UserID,
		Action: "exam.token_regenerate", Entity: "exam", EntityID: id, IP: r.RemoteAddr,
	})
	httpx.WriteJSON(w, http.StatusCreated, t)
}

// partAction: kerangka aksi pengawas per siswa + audit.
func (h *Handler) partAction(
	w http.ResponseWriter, r *http.Request,
	action string,
	fn func(tenantID, examID, studentID string) error,
) {
	c := auth.FromContext(r.Context())
	examID := chi.URLParam(r, "id")
	studentID := chi.URLParam(r, "studentID")
	if err := fn(c.TenantID, examID, studentID); err != nil {
		httpx.ErrorWriter(w, r, badReqOrNotFound(err))
		return
	}
	h.audit.Log(r.Context(), audit.Entry{
		TenantID: c.TenantID, ActorType: "user", ActorID: c.UserID,
		Action: "exam." + action, Entity: "exam_participant",
		EntityID: examID + "/" + studentID, IP: r.RemoteAddr,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "aksi " + action + " diterapkan"})
}

func (h *Handler) addTime(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Seconds int `json:"seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("payload tidak valid", nil))
		return
	}
	h.partAction(w, r, "add_time", func(tenantID, examID, studentID string) error {
		return h.store.AddTime(r.Context(), tenantID, examID, studentID, in.Seconds)
	})
}

func (h *Handler) resetAttempts(w http.ResponseWriter, r *http.Request) {
	h.partAction(w, r, "reset_attempts", func(tenantID, examID, studentID string) error {
		_, err := h.store.ResetAttempts(r.Context(), tenantID, examID, studentID)
		return err
	})
}

func (h *Handler) forceSubmit(w http.ResponseWriter, r *http.Request) {
	h.partAction(w, r, "force_submit", func(tenantID, examID, studentID string) error {
		return h.store.ForceSubmit(r.Context(), tenantID, examID, studentID)
	})
}

func (h *Handler) kickSession(w http.ResponseWriter, r *http.Request) {
	h.partAction(w, r, "kick", func(tenantID, examID, studentID string) error {
		return h.store.KickSession(r.Context(), tenantID, examID, studentID)
	})
}

// badReqOrNotFound: ErrNotFound → 404; lainnya dilempar apa adanya (pesan ID).
func badReqOrNotFound(err error) error {
	if errors.Is(err, ErrNotFound) {
		return httpx.ErrNotFound()
	}
	return err
}

func (h *Handler) listParticipants(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	ps, err := h.store.ListParticipants(r.Context(), c.TenantID, chi.URLParam(r, "id"))
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, ps)
}

func notFoundOrErr(err error) error {
	if errors.Is(err, ErrNotFound) {
		return httpx.ErrNotFound()
	}
	return err
}
