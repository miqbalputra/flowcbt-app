// Package classroom: CRUD kelas (FR-3.1) dan pemindahan kelas massal (FR-3.5).
// Semua query lewat WithTenantTx agar berjalan di bawah RLS dengan tenant dari JWT.
package classroom

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flowcbt/backend/internal/audit"
	"github.com/flowcbt/backend/internal/auth"
	"github.com/flowcbt/backend/internal/httpx"
	"github.com/flowcbt/backend/internal/platform"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Store struct {
	db *platform.DB
}

func NewStore(db *platform.DB) *Store { return &Store{db: db} }

type Class struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	AcademicYear string  `json:"academic_year"`
	HomeroomID   *string `json:"homeroom_teacher_id,omitempty"`
	StudentCount int     `json:"student_count"`
	Archived     bool    `json:"archived"`
}

func (s *Store) List(ctx context.Context, tenantID string, includeArchived bool) ([]Class, error) {
	var out []Class
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT c.id::text, c.name, c.academic_year, c.homeroom_teacher_id::text,
			       (SELECT count(*) FROM students st WHERE st.class_id = c.id AND st.status = 'active'),
			       (c.archived_at IS NOT NULL)
			FROM classes c
			WHERE ($1 OR c.archived_at IS NULL)
			ORDER BY c.name`, includeArchived)
		if err != nil {
			return err
		}
		defer rows.Close()
		out = []Class{}
		for rows.Next() {
			var c Class
			if err := rows.Scan(&c.ID, &c.Name, &c.AcademicYear, &c.HomeroomID, &c.StudentCount, &c.Archived); err != nil {
				return err
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	return out, err
}

func (s *Store) Create(ctx context.Context, tenantID, name, academicYear, homeroomID string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", httpx.ErrBadRequest("nama kelas wajib diisi", nil)
	}
	if academicYear == "" {
		y := time.Now().Year()
		if time.Now().Month() >= time.July {
			academicYear = strconv.Itoa(y) + "/" + strconv.Itoa(y+1)
		} else {
			academicYear = strconv.Itoa(y-1) + "/" + strconv.Itoa(y)
		}
	}
	var id string
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO classes (name, academic_year, homeroom_teacher_id)
			VALUES ($1, $2, NULLIF($3,'')::uuid) RETURNING id::text`,
			name, academicYear, homeroomID).Scan(&id)
	})
	if err != nil && strings.Contains(err.Error(), "23505") {
		return "", httpx.AppErr(409, "CLASS_EXISTS", "kelas dengan nama & tahun ajaran itu sudah ada", nil)
	}
	return id, err
}

func (s *Store) Rename(ctx context.Context, tenantID, id, name, academicYear string) error {
	if strings.TrimSpace(name) == "" {
		return httpx.ErrBadRequest("nama kelas wajib diisi", nil)
	}
	return s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE classes SET name = $3, academic_year = $4
			WHERE id = $2::uuid AND archived_at IS NULL`, id, name, academicYear)
		return requireOne(tag, err)
	})
}

// Archive: soft-delete (FR-3.1).
func (s *Store) Archive(ctx context.Context, tenantID, id string) error {
	return s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE classes SET archived_at = now() WHERE id = $2::uuid AND archived_at IS NULL`, id)
		return requireOne(tag, err)
	})
}

func requireOne(tag pgconn.CommandTag, err error) error {
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return httpx.ErrNotFound()
	}
	return nil
}

// ---- Handler ----

type Handler struct {
	store *Store
	audit *audit.Writer
}

func NewHandler(store *Store, aw *audit.Writer) *Handler { return &Handler{store: store, audit: aw} }

// Mount dipasang dalam grup terproteksi JWT + peran (dipanggil dari cmd/api).
func (h *Handler) Mount(r chi.Router) {
	r.Get("/classes", h.list)
	r.Post("/classes", h.create)
	r.Put("/classes/{id}", h.rename)
	r.Delete("/classes/{id}", h.archive)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	list, err := h.store.List(r.Context(), c.TenantID, r.URL.Query().Get("archived") == "1")
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

type classReq struct {
	Name         string `json:"name"`
	AcademicYear string `json:"academic_year"`
	HomeroomID   string `json:"homeroom_teacher_id"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	var req classReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("payload tidak valid", nil))
		return
	}
	id, err := h.store.Create(r.Context(), c.TenantID, req.Name, req.AcademicYear, req.HomeroomID)
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	h.audit.Log(r.Context(), audit.Entry{
		TenantID: c.TenantID, ActorType: "user", ActorID: c.UserID,
		Action: "class.create", Entity: "class", EntityID: id,
		Meta: map[string]any{"name": req.Name}, IP: r.RemoteAddr,
	})
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (h *Handler) rename(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	var req classReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("payload tidak valid", nil))
		return
	}
	if err := h.store.Rename(r.Context(), c.TenantID, chi.URLParam(r, "id"), req.Name, req.AcademicYear); err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "kelas diperbarui"})
}

func (h *Handler) archive(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.store.Archive(r.Context(), c.TenantID, id); err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	h.audit.Log(r.Context(), audit.Entry{
		TenantID: c.TenantID, ActorType: "user", ActorID: c.UserID,
		Action: "class.archive", Entity: "class", EntityID: id, IP: r.RemoteAddr,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "kelas diarsipkan"})
}

var _ = errors.Is
