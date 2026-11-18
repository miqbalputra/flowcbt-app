// Package student: CRUD siswa (FR-3.2), kredensial login otomatis, pindah kelas
// massal (FR-3.5), dan batas tier tenant (FR-3.3b). Query lewat WithTenantTx (RLS).
package student

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"strconv"
	"strings"

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

type Student struct {
	ID          string  `json:"id"`
	StudentCode string  `json:"student_code"`
	NIS         *string `json:"nis,omitempty"`
	Name        string  `json:"name"`
	Gender      string  `json:"gender"`
	ClassID     *string `json:"class_id,omitempty"`
	ClassName   *string `json:"class_name,omitempty"`
	Username    *string `json:"username,omitempty"`
	Status      string  `json:"status"`
}

type Credentials struct {
	Username    string `json:"username"`
	Password    string `json:"password"` // ditampilkan SEKALI setelah pembuatan
	StudentCode string `json:"student_code"`
}

type CreateInput struct {
	Name     string
	Gender   string
	ClassID  string
	NIS      string
	Username string // kosong = otomatis
	Password string // kosong = otomatis
}

// Create membuat siswa aktif + kredensial. Menolak saat batas tier terlampaui (FR-3.3b).
func (s *Store) Create(ctx context.Context, tenantID string, in CreateInput) (*Student, *Credentials, error) {
	if err := validate(in.Name, in.Gender); err != nil {
		return nil, nil, err
	}
	var st *Student
	var cred *Credentials
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		limit, active, err := limits(ctx, tx)
		if err != nil {
			return err
		}
		if active >= limit {
			return httpx.AppErr(403, "QUOTA_EXCEEDED",
				"batas siswa tercapai ("+strconv.Itoa(active)+"/"+strconv.Itoa(limit)+
					"). Perbarui paket untuk menambah siswa.", nil)
		}
		if in.ClassID != "" {
			if err := classExists(ctx, tx, in.ClassID); err != nil {
				return err
			}
		}

		code, err := uniqueCode(ctx, tx)
		if err != nil {
			return err
		}
		username := strings.TrimSpace(in.Username)
		if username == "" {
			username, err = uniqueUsername(ctx, tx, in.Name)
			if err != nil {
				return err
			}
		}
		password := in.Password
		if password == "" {
			password, err = randomToken(8)
			if err != nil {
				return err
			}
		}
		hash, err := auth.HashPassword(password)
		if err != nil {
			return err
		}

		st = &Student{StudentCode: code, Name: in.Name, Gender: in.Gender, Status: "active"}
		var classID, nis any
		if in.ClassID != "" {
			classID = in.ClassID
		}
		if in.NIS != "" {
			nis = in.NIS
		}
		err = tx.QueryRow(ctx, `
			INSERT INTO students (student_code, nis, name, name_normalized, gender, class_id, username, password_hash)
			VALUES ($1, $2, $3, $4, $5, $6::uuid, $7, $8)
			RETURNING id::text, class_id::text, username`,
			code, nis, in.Name, normalizeName(in.Name), in.Gender, classID, username, hash,
		).Scan(&st.ID, &st.ClassID, &st.Username)
		if err != nil {
			return err
		}
		cred = &Credentials{Username: username, Password: password, StudentCode: code}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return st, cred, nil
}

// limits = (batas efektif, jumlah siswa aktif). Batas efektif = min(langganan aktif, 100).
func limits(ctx context.Context, tx pgx.Tx) (limit, active int, err error) {
	var subLimit *int
	err = tx.QueryRow(ctx,
		`SELECT student_limit FROM subscriptions WHERE status = 'active' ORDER BY ends_at DESC LIMIT 1`,
	).Scan(&subLimit)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, err
	}
	limit = 100 // hard cap v1 (keputusan #11)
	if subLimit != nil && *subLimit > 0 && *subLimit < limit {
		limit = *subLimit
	}
	err = tx.QueryRow(ctx,
		`SELECT count(*) FROM students WHERE status = 'active' AND NOT is_guest`).Scan(&active)
	return limit, active, err
}

func classExists(ctx context.Context, tx pgx.Tx, id string) error {
	var ok bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM classes WHERE id = $1::uuid AND archived_at IS NULL)`, id,
	).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return httpx.ErrBadRequest("kelas tidak ditemukan atau diarsipkan", nil)
	}
	return nil
}

// List: filter kelas & pencarian nama; limit/offset sederhana.
func (s *Store) List(ctx context.Context, tenantID, classID, q string, limit, offset int) ([]Student, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var out []Student
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT s.id::text, s.student_code, s.nis, s.name, s.gender,
			       s.class_id::text, c.name, s.username, s.status
			FROM students s LEFT JOIN classes c ON c.id = s.class_id
			WHERE s.status <> 'archived'
			  AND ($1::uuid IS NULL OR s.class_id = $1::uuid)
			  AND ($2 = '' OR s.name ILIKE '%' || $2 || '%')
			ORDER BY s.name
			LIMIT $3 OFFSET $4`, classID, q, limit, offset)
		if err != nil {
			return err
		}
		defer rows.Close()
		out = []Student{}
		for rows.Next() {
			var st Student
			if err := rows.Scan(&st.ID, &st.StudentCode, &st.NIS, &st.Name, &st.Gender,
				&st.ClassID, &st.ClassName, &st.Username, &st.Status); err != nil {
				return err
			}
			out = append(out, st)
		}
		return rows.Err()
	})
	return out, err
}

func (s *Store) Get(ctx context.Context, tenantID, id string) (*Student, error) {
	var st Student
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT s.id::text, s.student_code, s.nis, s.name, s.gender,
			       s.class_id::text, c.name, s.username, s.status
			FROM students s LEFT JOIN classes c ON c.id = s.class_id
			WHERE s.id = $1::uuid`, id,
		).Scan(&st.ID, &st.StudentCode, &st.NIS, &st.Name, &st.Gender,
			&st.ClassID, &st.ClassName, &st.Username, &st.Status)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound()
	}
	if err != nil {
		return nil, err
	}
	return &st, nil
}

type UpdateInput struct {
	Name     *string
	Gender   *string
	ClassID  *string // nil = tetap; string kosong = lepas kelas
	NIS      *string
	Status   *string // active | inactive
	Password *string // reset password (hash baru)
}

func (s *Store) Update(ctx context.Context, tenantID, id string, in UpdateInput) error {
	if in.Name != nil && strings.TrimSpace(*in.Name) == "" {
		return httpx.ErrBadRequest("nama tidak boleh kosong", nil)
	}
	if in.Status != nil && *in.Status != "active" && *in.Status != "inactive" {
		return httpx.ErrBadRequest("status harus active|inactive", nil)
	}
	var hash any
	if in.Password != nil {
		h, err := auth.HashPassword(*in.Password)
		if err != nil {
			return httpx.ErrBadRequest(err.Error(), nil)
		}
		hash = h
	}
	var nameVal, normVal, genderVal, nisVal, statusVal any
	if in.Name != nil {
		nameVal = *in.Name
		normVal = normalizeName(*in.Name)
	}
	if in.Gender != nil {
		if *in.Gender != "L" && *in.Gender != "P" {
			return httpx.ErrBadRequest("jenis kelamin harus L atau P", nil)
		}
		genderVal = *in.Gender
	}
	if in.NIS != nil {
		nisVal = *in.NIS
	}
	if in.Status != nil {
		statusVal = *in.Status
	}

	return s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE students SET
				name            = COALESCE($2,  name),
				name_normalized = COALESCE($3,  name_normalized),
				gender          = COALESCE($4,  gender),
				class_id        = CASE WHEN $5 THEN NULLIF($6,'')::uuid ELSE class_id END,
				nis             = COALESCE($7,  nis),
				status          = COALESCE($8,  status),
				password_hash   = COALESCE($9,  password_hash)
			WHERE id = $1::uuid AND status <> 'archived'`,
			id, nameVal, normVal, genderVal, in.ClassID != nil, derefStr(in.ClassID),
			nisVal, statusVal, hash,
		)
		if err != nil {
			return err
		}
		return requireOne(tag, nil)
	})
}

// MoveClass: pindah kelas massal + riwayat kelas (FR-3.5; nilai tetap melekat).
func (s *Store) MoveClass(ctx context.Context, tenantID, classID string, studentIDs []string) (int, error) {
	if len(studentIDs) == 0 {
		return 0, httpx.ErrBadRequest("pilih minimal satu siswa", nil)
	}
	var moved int
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		if err := classExists(ctx, tx, classID); err != nil {
			return err
		}
		for _, sid := range studentIDs {
			tag, err := tx.Exec(ctx,
				`UPDATE students SET class_id = $1::uuid WHERE id = $2::uuid AND status = 'active'`,
				classID, sid)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				continue // tidak ada / arsip: dilewati, dijawab lewat jumlah pindah
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO student_class_history (student_id, class_id, from_date)
				VALUES ($1::uuid, $2::uuid, CURRENT_DATE)`, sid, classID); err != nil {
				return err
			}
			moved++
		}
		return nil
	})
	return moved, err
}

// Archive: soft-delete (status 'archived').
func (s *Store) Archive(ctx context.Context, tenantID, id string) error {
	return s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE students SET status = 'archived' WHERE id = $1::uuid AND status <> 'archived'`, id)
		return requireOne(tag, err)
	})
}

// ---- helper murni ----

func validate(name, gender string) error {
	if strings.TrimSpace(name) == "" {
		return httpx.ErrBadRequest("nama siswa wajib diisi", nil)
	}
	if gender != "L" && gender != "P" {
		return httpx.ErrBadRequest("jenis kelamin harus L atau P", nil)
	}
	return nil
}

func normalizeName(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(name), " "))
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
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

func uniqueCode(ctx context.Context, tx pgx.Tx) (string, error) {
	for i := 0; i < 5; i++ {
		n, err := randInt(9_000_000)
		if err != nil {
			return "", err
		}
		code := "S" + strconv.FormatInt(n+1_000_000, 10)
		var taken bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM students WHERE student_code = $1)`, code).Scan(&taken); err != nil {
			return "", err
		}
		if !taken {
			return code, nil
		}
	}
	return "", errors.New("gagal membuat kode siswa unik")
}

func uniqueUsername(ctx context.Context, tx pgx.Tx, name string) (string, error) {
	base := usernameBase(name)
	for i := 0; i < 5; i++ {
		cand := base
		if i > 0 {
			n, err := randInt(1000)
			if err != nil {
				return "", err
			}
			cand = base + strconv.FormatInt(n, 10)
		}
		var taken bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM students WHERE username = $1)`, cand).Scan(&taken); err != nil {
			return "", err
		}
		if !taken {
			return cand, nil
		}
	}
	return "", errors.New("gagal membuat username unik")
}

// usernameBase: nama → huruf kecil tanpa spasi + 3 digit acak.
func usernameBase(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	base := b.String()
	if len(base) > 16 {
		base = base[:16]
	}
	if base == "" {
		base = "siswa"
	}
	n, _ := randInt(1000)
	return base + strconv.FormatInt(n, 10)
}

func randomToken(n int) (string, error) {
	const chars = "abcdefghjkmnpqrstuvwxyz23456789"
	out := make([]byte, n)
	for i := range out {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		out[i] = chars[idx.Int64()]
	}
	return string(out), nil
}

func randInt(maxExclusive int64) (int64, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(maxExclusive))
	if err != nil {
		return 0, err
	}
	return n.Int64(), nil
}

// ---- Handler HTTP (dipasang di grup JWT terproteksi) ----

type Handler struct {
	store *Store
	audit *audit.Writer
}

func NewHandler(store *Store, aw *audit.Writer) *Handler { return &Handler{store: store, audit: aw} }

func (h *Handler) Mount(r chi.Router) {
	r.Get("/students", h.list)
	r.Post("/students", h.create)
	r.Get("/students/{id}", h.get)
	r.Put("/students/{id}", h.update)
	r.Delete("/students/{id}", h.archive)
	r.Post("/students/move-class", h.moveClass)
	// S5: import/export siswa. Kartu login dicetak dari kredensial hasil
	// import/creation di sisi klien (server hanya menyimpan hash password).
	r.Get("/students/import/template", h.importTemplate)
	r.Post("/students/import/preview", h.importPreview)
	r.Post("/students/import/commit", h.importCommit)
	r.Get("/students/export", h.export)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	qp := r.URL.Query()
	limit, _ := strconv.Atoi(qp.Get("limit"))
	offset, _ := strconv.Atoi(qp.Get("offset"))
	list, err := h.store.List(r.Context(), c.TenantID, qp.Get("class_id"), qp.Get("q"), limit, offset)
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

type createReq struct {
	Name     string `json:"name"`
	Gender   string `json:"gender"`
	ClassID  string `json:"class_id"`
	NIS      string `json:"nis"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	var req createReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("payload tidak valid", nil))
		return
	}
	st, cred, err := h.store.Create(r.Context(), c.TenantID, CreateInput{
		Name: req.Name, Gender: req.Gender, ClassID: req.ClassID,
		NIS: req.NIS, Username: req.Username, Password: req.Password,
	})
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	h.audit.Log(r.Context(), audit.Entry{
		TenantID: c.TenantID, ActorType: "user", ActorID: c.UserID,
		Action: "student.create", Entity: "student", EntityID: st.ID, IP: r.RemoteAddr,
	})
	// Kredensial awal dikembalikan sekali (kartu login siswa datang di S5).
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"student": st, "credentials": cred})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	st, err := h.store.Get(r.Context(), c.TenantID, chi.URLParam(r, "id"))
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, st)
}

type updateReq struct {
	Name     *string `json:"name"`
	Gender   *string `json:"gender"`
	ClassID  *string `json:"class_id"`
	NIS      *string `json:"nis"`
	Status   *string `json:"status"`
	Password *string `json:"password"`
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	var req updateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("payload tidak valid", nil))
		return
	}
	id := chi.URLParam(r, "id")
	in := UpdateInput{Name: req.Name, Gender: req.Gender, ClassID: req.ClassID,
		NIS: req.NIS, Status: req.Status, Password: req.Password}
	if err := h.store.Update(r.Context(), c.TenantID, id, in); err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	h.audit.Log(r.Context(), audit.Entry{
		TenantID: c.TenantID, ActorType: "user", ActorID: c.UserID,
		Action: "student.update", Entity: "student", EntityID: id, IP: r.RemoteAddr,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "siswa diperbarui"})
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
		Action: "student.archive", Entity: "student", EntityID: id, IP: r.RemoteAddr,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "siswa diarsipkan"})
}

type moveClassReq struct {
	StudentIDs []string `json:"student_ids"`
	ClassID    string   `json:"class_id"`
}

func (h *Handler) moveClass(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	var req moveClassReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("payload tidak valid", nil))
		return
	}
	moved, err := h.store.MoveClass(r.Context(), c.TenantID, req.ClassID, req.StudentIDs)
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	h.audit.Log(r.Context(), audit.Entry{
		TenantID: c.TenantID, ActorType: "user", ActorID: c.UserID,
		Action: "student.move_class", Entity: "class", EntityID: req.ClassID,
		Meta: map[string]any{"moved": moved, "students": len(req.StudentIDs)}, IP: r.RemoteAddr,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]int{"moved": moved})
}
