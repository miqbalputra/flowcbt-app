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
}

func NewHandler(store *Store, aw *audit.Writer) *Handler {
	return &Handler{store: store, audit: aw}
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
	r.Get("/exams/{id}/participants", h.listParticipants)
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
