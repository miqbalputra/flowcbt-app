// Handler HTTP questionbank + media (PRD 9 rute "Soal" & "/media/upload").
package questionbank

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/flowcbt/backend/internal/audit"
	"github.com/flowcbt/backend/internal/auth"
	"github.com/flowcbt/backend/internal/httpx"
	"github.com/flowcbt/backend/internal/platform"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type Handler struct {
	store   *Store
	storage platform.Storage
	audit   *audit.Writer
}

func NewHandler(store *Store, storage platform.Storage, aw *audit.Writer) *Handler {
	return &Handler{store: store, storage: storage, audit: aw}
}

// Mount dipasang pada grup JWT terproteksi (owner/teacher).
func (h *Handler) Mount(r chi.Router) {
	r.Get("/question-sets", h.listSets)
	r.Post("/question-sets", h.createSet)
	r.Get("/question-sets/{id}", h.getSet)
	r.Put("/question-sets/{id}", h.updateSet)
	r.Delete("/question-sets/{id}", h.archiveSet)
	r.Get("/question-sets/{id}/questions", h.listQuestions)
	r.Get("/question-sets/{id}/published-questions", h.listPublishedQuestions)
	r.Post("/question-sets/{id}/questions", h.addQuestion)
	r.Put("/questions/{id}", h.updateQuestion)
	r.Delete("/questions/{id}", h.deleteQuestion)
	r.Post("/questions/reorder", h.reorder)
	r.Post("/question-sets/{id}/publish", h.publish)
	r.Post("/question-sets/{id}/unpublish", h.unpublish)
	r.Post("/question-sets/{id}/duplicate", h.duplicate)

	// Import/export template soal (S8, FR-4.9). Kuota = stub (S27).
	r.Get("/question-sets/import/template", h.questionTemplate)
	r.Post("/question-sets/{id}/import/preview", h.questionImportPreview)
	r.Post("/question-sets/{id}/import/commit", h.questionImportCommit)
	r.Get("/question-sets/{id}/export", h.questionExport)

	// Media (gambar/audio soal). FR-4.4; kuota per tenant (PRD 3.2).
	r.Post("/media", h.uploadMedia)
	r.Get("/media/*", h.serveMedia)
}

// ---- paket soal ----

func (h *Handler) listSets(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	sets, err := h.store.ListSets(r.Context(), c.TenantID,
		r.URL.Query().Get("q"), r.URL.Query().Get("status"), limit, offset)
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, sets)
}

func (h *Handler) createSet(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	var in SetInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("payload tidak valid", nil))
		return
	}
	if strings.TrimSpace(in.SubjectName) == "" || strings.TrimSpace(in.Title) == "" {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("mata pelajaran & judul wajib diisi", nil))
		return
	}
	desc, err := Sanitize(in.Description)
	if err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("deskripsi mengandung HTML tidak valid", nil))
		return
	}
	in.Description = desc
	// Kode ujian dibuat otomatis bila kosong (FR-4.1).
	set, err := h.store.CreateSet(r.Context(), c.TenantID, c.UserID, in)
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, set)
}

func (h *Handler) getSet(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	set, err := h.store.GetSet(r.Context(), c.TenantID, chi.URLParam(r, "id"))
	if err != nil {
		httpx.ErrorWriter(w, r, notFoundOrErr(err))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, set)
}

func (h *Handler) updateSet(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	var in SetUpdate
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("payload tidak valid", nil))
		return
	}
	if in.Description != nil {
		if d, err := Sanitize(*in.Description); err == nil {
			in.Description = &d
		}
	}
	if err := h.store.UpdateSet(r.Context(), c.TenantID, chi.URLParam(r, "id"), in); err != nil {
		httpx.ErrorWriter(w, r, notFoundOrErr(err))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "paket diperbarui"})
}

func (h *Handler) archiveSet(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.store.ArchiveSet(r.Context(), c.TenantID, id); err != nil {
		httpx.ErrorWriter(w, r, notFoundOrErr(err))
		return
	}
	h.audit.Log(r.Context(), audit.Entry{
		TenantID: c.TenantID, ActorType: "user", ActorID: c.UserID,
		Action: "question_set.archive", Entity: "question_set", EntityID: id, IP: r.RemoteAddr,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "paket diarsipkan"})
}

// ---- soal ----

type questionReq struct {
	Type                string   `json:"type"`
	Body                string   `json:"body"`
	Options             []Option `json:"options"`
	AnswerKey           any      `json:"answer_key"`
	Rubric              any      `json:"rubric"`
	Points              float64  `json:"points"`
	ScoringMode         string   `json:"scoring_mode"`
	PartialWrongPenalty *bool    `json:"partial_wrong_penalty"`
	ShuffleOptions      *bool    `json:"shuffle_options"`
	ExplanationHTML     string   `json:"explanation_html"`
	StimulusID          string   `json:"stimulus_id"`
}

func (h *Handler) listQuestions(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	id := chi.URLParam(r, "id")
	qs, err := h.store.QuestionsOfSet(r.Context(), c.TenantID, id)
	if err != nil {
		httpx.ErrorWriter(w, r, notFoundOrErr(err))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, qs)
}

// listPublishedQuestions: soal versi terbit — kuncijawaban tetap disensor.
func (h *Handler) listPublishedQuestions(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	qs, err := h.store.PublishedQuestionsOfSet(r.Context(), c.TenantID, chi.URLParam(r, "id"))
	if err != nil {
		httpx.ErrorWriter(w, r, notFoundOrErr(err))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, qs)
}

func (h *Handler) addQuestion(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	setID := chi.URLParam(r, "id")
	var req questionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("payload tidak valid", nil))
		return
	}
	if !IsSupportedType(req.Type) {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("tipe soal tidak dikenal", nil))
		return
	}
	if req.ScoringMode == "" {
		req.ScoringMode = "all_or_nothing"
	}
	body, err := Sanitize(req.Body)
	if err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("soal mengandung HTML tidak valid", nil))
		return
	}
	expl, err := Sanitize(req.ExplanationHTML)
	if err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("penjelasan mengandung HTML tidak valid", nil))
		return
	}
	q, err := h.store.AddQuestion(r.Context(), c.TenantID, setID, Question{
		Type: req.Type, Body: body, Options: req.Options, AnswerKey: req.AnswerKey,
		Rubric: req.Rubric, Points: req.Points, ScoringMode: req.ScoringMode,
		PartialWrongPenalty: req.PartialWrongPenalty == nil || *req.PartialWrongPenalty,
		ShuffleOptions:      req.ShuffleOptions == nil || *req.ShuffleOptions,
		ExplanationHTML:     expl, StimulusID: strPtr(req.StimulusID),
	})
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	h.audit.Log(r.Context(), audit.Entry{
		TenantID: c.TenantID, ActorType: "user", ActorID: c.UserID,
		Action: "question.create", Entity: "question", EntityID: q.ID, IP: r.RemoteAddr,
	})
	httpx.WriteJSON(w, http.StatusCreated, q)
}

func (h *Handler) updateQuestion(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	var req questionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("payload tidak valid", nil))
		return
	}
	if req.Body != "" {
		if b, err := Sanitize(req.Body); err == nil {
			req.Body = b
		}
	}
	if req.ExplanationHTML != "" {
		if e, err := Sanitize(req.ExplanationHTML); err == nil {
			req.ExplanationHTML = e
		}
	}
	// Audit perubahan kunci (S8): bandingkan kunci lama sebelum menulis.
	oldKey, hadKey, err := h.store.AnswerKeyOf(r.Context(), c.TenantID, chi.URLParam(r, "id"))
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	err = h.store.UpdateQuestion(r.Context(), c.TenantID, chi.URLParam(r, "id"),
		strPtr(req.StimulusID), req.Body, req.Options, req.AnswerKey, req.Rubric,
		req.Points, req.ScoringMode, req.PartialWrongPenalty, req.ShuffleOptions,
		req.ExplanationHTML)
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	if keyChanged(oldKey, hadKey, req.AnswerKey) {
		h.audit.Log(r.Context(), audit.Entry{
			TenantID: c.TenantID, ActorType: "user", ActorID: c.UserID,
			Action: "question.answer_key_change", Entity: "question",
			EntityID: chi.URLParam(r, "id"), IP: r.RemoteAddr,
		})
	}
	h.audit.Log(r.Context(), audit.Entry{
		TenantID: c.TenantID, ActorType: "user", ActorID: c.UserID,
		Action: "question.update", Entity: "question", EntityID: chi.URLParam(r, "id"), IP: r.RemoteAddr,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "soal diperbarui"})
}

func (h *Handler) deleteQuestion(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteQuestion(r.Context(), c.TenantID, id); err != nil {
		httpx.ErrorWriter(w, r, notFoundOrErr(err))
		return
	}
	h.audit.Log(r.Context(), audit.Entry{
		TenantID: c.TenantID, ActorType: "user", ActorID: c.UserID,
		Action: "question.delete", Entity: "question", EntityID: id, IP: r.RemoteAddr,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "soal dihapus"})
}

type reorderReq struct {
	SetID string   `json:"set_id"`
	Order []string `json:"order"`
}

func (h *Handler) reorder(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	var req reorderReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Order) == 0 {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("order wajib diisi", nil))
		return
	}
	if err := h.store.Reorder(r.Context(), c.TenantID, req.SetID, req.Order); err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "urutan disimpan"})
}

// ---- publish & duplicate ----

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	id := chi.URLParam(r, "id")
	v, err := h.store.PublishSet(r.Context(), c.TenantID, id, c.UserID)
	if err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest(err.Error(), nil))
		return
	}
	h.audit.Log(r.Context(), audit.Entry{
		TenantID: c.TenantID, ActorType: "user", ActorID: c.UserID,
		Action: "question_set.publish", Entity: "question_set", EntityID: id,
		Meta: map[string]any{"version": v.VersionNo}, IP: r.RemoteAddr,
	})
	httpx.WriteJSON(w, http.StatusCreated, v)
}

func (h *Handler) duplicate(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	id := chi.URLParam(r, "id")
	newID, err := h.store.DuplicateSet(r.Context(), c.TenantID, id, c.UserID)
	if err != nil {
		httpx.ErrorWriter(w, r, notFoundOrErr(err))
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"id": newID})
}

// ---- media ----

var maxMediaBytes int64 = 5 << 20 // 5 MB per file (gambar/audio kecil)

func (h *Handler) uploadMedia(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	if err := r.ParseMultipartForm(maxMediaBytes + 1<<20); err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("unggahan tidak valid", nil))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("field file wajib diisi", nil))
		return
	}
	defer file.Close()
	kind := r.FormValue("kind")
	if kind != "image" && kind != "audio" {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("kind harus image|audio", nil))
		return
	}
	if header.Size > maxMediaBytes {
		httpx.ErrorWriter(w, r, httpx.AppErr(413, "PAYLOAD_TOO_LARGE", "ukuran file melebihi 5 MB", nil))
		return
	}

	ext := ""
	if i := strings.LastIndex(header.Filename, "."); i >= 0 {
		ext = header.Filename[i+1:]
	}
	key := platform.ObjectKey(c.TenantID, ext)
	var id string
	err = h.store.WithTenantTxForMedia(r.Context(), c.TenantID, func(tx pgx.Tx) error {
		// Kuota penyimpanan (PRD 3.2: default 1 GB, diubah Super Admin).
		var used, quota int64
		if err := tx.QueryRow(r.Context(),
			`SELECT storage_used_bytes, storage_quota_bytes FROM tenants WHERE id = $1::uuid`,
			c.TenantID).Scan(&used, &quota); err != nil {
			return err
		}
		if used+header.Size > quota {
			return httpx.AppErr(403, "QUOTA_EXCEEDED",
				"kuota penyimpanan media tercapai", nil)
		}
		if _, err := tx.Exec(r.Context(),
			`UPDATE tenants SET storage_used_bytes = storage_used_bytes + $2 WHERE id = $1::uuid`,
			c.TenantID, header.Size); err != nil {
			return err
		}
		return tx.QueryRow(r.Context(), `
			INSERT INTO media_assets (kind, object_key, mime, size_bytes, created_by)
			VALUES ($1, $2, $3, $4, $5::uuid) RETURNING id::text`,
			kind, key, header.Header.Get("Content-Type"), header.Size, c.UserID).Scan(&id)
	})
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	// Setelah DB commit, simpan file fisik. Gagal simpan → koreksi kuota.
	if _, err = h.storage.Save(r.Context(), key, header.Header.Get("Content-Type"), file); err != nil {
		_ = h.store.CompensateMedia(r.Context(), c.TenantID, id, header.Size)
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"id": id, "object_key": key, "url": "/media/" + key,
		"mime": header.Header.Get("Content-Type"), "size_bytes": header.Size,
	})
}

// serveMedia: baca objek dari storage (editor & pratinjau guru; klien siswa
// memuat media saat pra-muat ujian, S15).
func (h *Handler) serveMedia(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(chi.URLParam(r, "*"), "/")
	if !strings.HasPrefix(key, "tenant/") || strings.Contains(key, "..") {
		httpx.ErrorWriter(w, r, httpx.ErrNotFound())
		return
	}
	rc, err := h.storage.Get(r.Context(), key)
	if err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrNotFound())
		return
	}
	defer rc.Close()
	if _, err = io.Copy(w, rc); err != nil {
		return
	}
}

func notFoundOrErr(err error) error {
	if errors.Is(err, ErrNotFound) {
		return httpx.ErrNotFound()
	}
	return err
}

// keyChanged menyandingkan kunci lama & baru secara kanonik (JSON ringkas).
func keyChanged(old json.RawMessage, hadKey bool, newKey any) bool {
	oldStr := "null"
	if hadKey {
		var buf bytes.Buffer
		if err := json.Compact(&buf, old); err == nil {
			oldStr = buf.String()
		}
	}
	newStr := "null"
	if newKey != nil {
		if b, err := json.Marshal(newKey); err == nil {
			newStr = string(b)
		}
	}
	return oldStr != newStr
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// randomHex helper bila perlu token kecil tanpa dep.
func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
