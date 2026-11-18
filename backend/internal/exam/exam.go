// Package exam: CRUD ujian, ruang, token, penugasan peserta (S9, FR-5.1–5.6).
// Jadwal divalidasi di server (FR-5.1–5.5); status draft tidak punya jadwal —
// validasi penuh berjalan saat publish ujian (S10).
package exam

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"time"

	"github.com/flowcbt/backend/internal/platform"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrNotFound = errors.New("tidak ditemukan")

type Store struct{ db *platform.DB }

func NewStore(db *platform.DB) *Store { return &Store{db: db} }

func requireOne(tag pgconn.CommandTag, err error) error {
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Exam: baris tabel exams (kolom operasional S9; skor/proktor dibaca di modul lain).
type Exam struct {
	ID            string   `json:"id"`
	QuestionSetID string   `json:"question_set_id"`
	SetVersionID  string   `json:"set_version_id,omitempty"`
	Name          string   `json:"name"`
	Status        string   `json:"status"` // draft|published|paused|closed|cancelled
	OpenAt        *string  `json:"open_at,omitempty"`
	CloseAt       *string  `json:"close_at,omitempty"`
	DurationMin   *int     `json:"duration_minutes,omitempty"`
	ClosePolicy   string   `json:"close_policy"`
	LoginMode     string   `json:"login_mode"`
	MaxAttempts   *int     `json:"max_attempts,omitempty"`
	ScorePolicy   string   `json:"score_policy"`
	PassingScore  *float64 `json:"passing_score,omitempty"`
	ShowScoreNow  bool     `json:"show_score_immediately"`
	ShowReview    bool     `json:"show_review"`
	ShowExplain   bool     `json:"show_explanations"`
	ShuffleQ      bool     `json:"shuffle_questions"`
	ShuffleO      bool     `json:"shuffle_options"`
	FreeNav       bool     `json:"free_navigation"`
	RequireAll    bool     `json:"require_all_answered"`
	CreatedBy     string   `json:"created_by"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
}

// Kolom SELECT ringkas yang dipakai berulang.
const examCols = `id::text, question_set_id::text, COALESCE(set_version_id::text,''), name, status,
	open_at::text, close_at::text, duration_minutes, close_policy, login_mode, max_attempts,
	score_policy, passing_score::float8, show_score_immediately, show_review, show_explanations,
	shuffle_questions, shuffle_options, free_navigation, require_all_answered,
	created_by::text, created_at::text, updated_at::text`

func scanExam(row pgx.Row) (*Exam, error) {
	var e Exam
	err := row.Scan(&e.ID, &e.QuestionSetID, &e.SetVersionID, &e.Name, &e.Status,
		&e.OpenAt, &e.CloseAt, &e.DurationMin, &e.ClosePolicy, &e.LoginMode, &e.MaxAttempts,
		&e.ScorePolicy, &e.PassingScore, &e.ShowScoreNow, &e.ShowReview, &e.ShowExplain,
		&e.ShuffleQ, &e.ShuffleO, &e.FreeNav, &e.RequireAll,
		&e.CreatedBy, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// ExamInput: field yang diisi wizard (Info) — sisanya default DB.
type ExamInput struct {
	QuestionSetID string   `json:"question_set_id"`
	Name          string   `json:"name"`
	OpenAt        string   `json:"open_at"` // RFC3339 / datetime-local+07:00
	CloseAt       string   `json:"close_at"`
	DurationMin   int      `json:"duration_minutes"`
	ClosePolicy   string   `json:"close_policy"`
	LoginMode     string   `json:"login_mode"`
	MaxAttempts   *int     `json:"max_attempts"`
	ScorePolicy   string   `json:"score_policy"`
	PassingScore  *float64 `json:"passing_score"`
	ShowScoreNow  *bool    `json:"show_score_immediately"`
	ShowReview    *bool    `json:"show_review"`
	ShowExplain   *bool    `json:"show_explanations"`
	ShuffleQ      *bool    `json:"shuffle_questions"`
	ShuffleO      *bool    `json:"shuffle_options"`
	FreeNav       *bool    `json:"free_navigation"`
	RequireAll    *bool    `json:"require_all_answered"`
}

// ValidateSchedule (FR-5.1–5.5): aturan jadwal & durasi pada level aplikasi
// (di luar CHECK DB agar pesan kesalahan berbahasa Indonesia).
func (in *ExamInput) ValidateSchedule() error {
	if strings.TrimSpace(in.Name) == "" {
		return errors.New("nama ujian wajib diisi")
	}
	if in.QuestionSetID == "" {
		return errors.New("paket soal wajib dipilih")
	}
	if in.DurationMin <= 0 {
		return errors.New("durasi pengerjaan harus lebih dari 0 menit (FR-5.2)")
	}
	if in.MaxAttempts != nil && *in.MaxAttempts <= 0 {
		return errors.New("jumlah percobaan harus > 0 (FR-5.3)")
	}
	if in.PassingScore != nil && (*in.PassingScore < 0 || *in.PassingScore > 100) {
		return errors.New("KKM harus 0–100")
	}
	if in.ClosePolicy != "" && in.ClosePolicy != "hard" && in.ClosePolicy != "grace" {
		return errors.New("kebijakan tutup harus hard|grace")
	}
	if in.LoginMode != "" && in.LoginMode != "account" && in.LoginMode != "guest" && in.LoginMode != "both" {
		return errors.New("mode masuk harus account|guest|both")
	}
	if in.ScorePolicy != "" && in.ScorePolicy != "highest" && in.ScorePolicy != "last" && in.ScorePolicy != "avg" {
		return errors.New("kebijakan nilai harus highest|last|avg (FR-5.3)")
	}
	if in.OpenAt != "" && in.CloseAt != "" {
		open, err1 := time.Parse(time.RFC3339, in.OpenAt)
		close, err2 := time.Parse(time.RFC3339, in.CloseAt)
		if err1 != nil || err2 != nil {
			return errors.New("format waktu tidak valid (RFC3339)")
		}
		if !close.After(open) {
			return errors.New("waktu tutup harus setelah waktu buka (FR-5.1)")
		}
		if close.Sub(open) < time.Duration(in.DurationMin)*time.Minute {
			return errors.New("jarak buka–tutup lebih pendek dari durasi pengerjaan (FR-5.2)")
		}
	} else if in.OpenAt != "" || in.CloseAt != "" {
		return errors.New("waktu buka dan tutup diisi keduanya atau keduanya kosong")
	}
	return nil
}

func optBool(b *bool, def bool) bool {
	if b == nil {
		return def
	}
	return *b
}

// Create: ujian draft dari paket soal (prasyarat publish nanti: paket terbit).
func (s *Store) Create(ctx context.Context, tenantID, userID string, in ExamInput) (*Exam, error) {
	if err := in.ValidateSchedule(); err != nil {
		return nil, err
	}
	var e *Exam
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		// Paket harus ada; alternatif: catat versi terbit bila sudah ada.
		var published string
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(published_version_id::text, '') FROM question_sets WHERE id = $1::uuid`,
			in.QuestionSetID).Scan(&published); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errors.New("paket soal tidak ditemukan")
			}
			return err
		}
		var versionArg any
		if published != "" {
			versionArg = published
		}
		var err error
		e, err = scanExam(tx.QueryRow(ctx, `
			INSERT INTO exams (question_set_id, set_version_id, name, open_at, close_at,
				duration_minutes, close_policy, login_mode, max_attempts, score_policy,
				passing_score, show_score_immediately, show_review, show_explanations,
				shuffle_questions, shuffle_options, free_navigation, require_all_answered, created_by)
			VALUES ($1::uuid, $2::uuid, $3, NULLIF($4,'')::timestamptz, NULLIF($5,'')::timestamptz,
				$6, COALESCE(NULLIF($7,''),'grace'), COALESCE(NULLIF($8,''),'both'), $9,
				COALESCE(NULLIF($10,''),'highest'), $11, $12, $13, $14, $15, $16, $17, $18, $19)
			RETURNING `+examCols,
			in.QuestionSetID, versionArg, in.Name, in.OpenAt, in.CloseAt,
			in.DurationMin, in.ClosePolicy, in.LoginMode, in.MaxAttempts,
			in.ScorePolicy, in.PassingScore,
			optBool(in.ShowScoreNow, true), optBool(in.ShowReview, false), optBool(in.ShowExplain, false),
			optBool(in.ShuffleQ, true), optBool(in.ShuffleO, true), optBool(in.FreeNav, true),
			optBool(in.RequireAll, false), userID,
		))
		return err
	})
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (s *Store) List(ctx context.Context, tenantID, q, status string, limit, offset int) ([]*Exam, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	out := []*Exam{}
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT `+examCols+` FROM exams
			WHERE ($1 = '' OR name ILIKE '%' || $1 || '%') AND ($2 = '' OR status = $2)
			ORDER BY created_at DESC LIMIT $3 OFFSET $4`, q, status, limit, offset)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			e, err := scanExam(rows)
			if err != nil {
				return err
			}
			out = append(out, e)
		}
		return rows.Err()
	})
	return out, err
}

func (s *Store) Get(ctx context.Context, tenantID, id string) (*Exam, error) {
	var e *Exam
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		var err error
		e, err = scanExam(tx.QueryRow(ctx, `SELECT `+examCols+` FROM exams WHERE id = $1::uuid`, id))
		return err
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return e, nil
}

// UpdateSetUpdate: perubahan field bertanda = ubah; nil = biarkan.
type ExamUpdate struct {
	Name         *string
	OpenAt       *string
	CloseAt      *string
	DurationMin  *int
	ClosePolicy  *string
	LoginMode    *string
	MaxAttempts  *int
	ScorePolicy  *string
	PassingScore *float64
	ShowScoreNow *bool
	ShowReview   *bool
	ShowExplain  *bool
	ShuffleQ     *bool
	ShuffleO     *bool
	FreeNav      *bool
	RequireAll   *bool
}

// Update hanya pada ujian draft/published yang belum selesai — jadwal boleh
// digeser selama ujian belum ditutup (FR-5.7 memakai endpoint khusus).
func (s *Store) Update(ctx context.Context, tenantID, id string, in ExamUpdate) error {
	// Validasi atas nilai efektif (perubahan digabung dengan nilai saat ini).
	cur, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if cur.Status == "closed" || cur.Status == "cancelled" {
		return errors.New("ujian sudah ditutup atau dibatalkan")
	}
	eff := ExamInput{
		QuestionSetID: cur.QuestionSetID,
		Name:          firstNonEmpty(in.Name, cur.Name),
		DurationMin:   firstInt(in.DurationMin, cur.DurationMin),
		ClosePolicy:   firstNonEmpty(in.ClosePolicy, cur.ClosePolicy),
		LoginMode:     firstNonEmpty(in.LoginMode, cur.LoginMode),
		ScorePolicy:   firstNonEmpty(in.ScorePolicy, cur.ScorePolicy),
		OpenAt:        firstStr(in.OpenAt, cur.OpenAt),
		CloseAt:       firstStr(in.CloseAt, cur.CloseAt),
		MaxAttempts:   firstPtr(in.MaxAttempts, cur.MaxAttempts),
		PassingScore:  firstPtr(in.PassingScore, cur.PassingScore),
	}
	// Jadwal lengkap → validasi penuh; masih parsial (draft) → validasi ringan.
	if eff.OpenAt != "" && eff.CloseAt != "" {
		if err := eff.ValidateSchedule(); err != nil {
			return err
		}
	} else if strings.TrimSpace(eff.Name) == "" {
		return errors.New("nama ujian wajib diisi")
	}
	return s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE exams SET
				name            = COALESCE($3, name),
				open_at         = COALESCE(NULLIF($4,'')::timestamptz, open_at),
				close_at        = COALESCE(NULLIF($5,'')::timestamptz, close_at),
				duration_minutes = COALESCE($6, duration_minutes),
				close_policy    = COALESCE($7, close_policy),
				login_mode      = COALESCE($8, login_mode),
				max_attempts    = COALESCE($9, max_attempts),
				score_policy    = COALESCE($10, score_policy),
				passing_score   = COALESCE($11, passing_score),
				show_score_immediately = COALESCE($12, show_score_immediately),
				show_review     = COALESCE($13, show_review),
				show_explanations = COALESCE($14, show_explanations),
				shuffle_questions = COALESCE($15, shuffle_questions),
				shuffle_options = COALESCE($16, shuffle_options),
				free_navigation = COALESCE($17, free_navigation),
				require_all_answered = COALESCE($18, require_all_answered),
				updated_at = now()
			WHERE id = $1::uuid AND tenant_id = $2::uuid`,
			id, tenantID, in.Name, strOrEmpty(in.OpenAt), strOrEmpty(in.CloseAt),
			in.DurationMin, in.ClosePolicy, in.LoginMode, in.MaxAttempts,
			in.ScorePolicy, in.PassingScore, in.ShowScoreNow, in.ShowReview,
			in.ShowExplain, in.ShuffleQ, in.ShuffleO, in.FreeNav, in.RequireAll)
		return requireOne(tag, err)
	})
}

// Delete: hanya draft (published/paused memakai arsip status, S10+).
func (s *Store) Delete(ctx context.Context, tenantID, id string) error {
	return s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`DELETE FROM exams WHERE id = $1::uuid AND status = 'draft'`, id)
		return requireOne(tag, err)
	})
}

// firstNonEmpty: pakai nilai baru bila pointer terisi, else nilai saat ini.
func firstNonEmpty(p *string, cur string) string {
	if p != nil {
		return *p
	}
	return cur
}

// firstStr: gabungan dua pointer string → "" bila keduanya nil.
func firstStr(p *string, cur *string) string {
	if p != nil {
		return *p
	}
	if cur != nil {
		return *cur
	}
	return ""
}

// firstInt: gabungan pointer int (nilai saat ini bisa nil).
func firstInt(p *int, cur *int) int {
	if p != nil {
		return *p
	}
	if cur != nil {
		return *cur
	}
	return 0
}

// firstPtr: pointer baru bila terisi, else pointer saat ini.
func firstPtr[T any](p *T, cur *T) *T {
	if p != nil {
		return p
	}
	return cur
}

// strOrEmpty: pointer → "" bila nil (parameter COALESCE di SQL).
func strOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ---- ruang & token ----

type Room struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	ExamID string `json:"exam_id"`
}

func (s *Store) AddRoom(ctx context.Context, tenantID, examID, name string) (*Room, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("nama ruang wajib diisi")
	}
	var r Room
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO exam_rooms (exam_id, name) VALUES ($1::uuid, $2)
			ON CONFLICT (exam_id, name) DO UPDATE SET name = EXCLUDED.name
			RETURNING id::text, name, exam_id::text`, examID, name,
		).Scan(&r.ID, &r.Name, &r.ExamID)
	})
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) ListRooms(ctx context.Context, tenantID, examID string) ([]Room, error) {
	out := []Room{}
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT r.id::text, r.name, r.exam_id::text FROM exam_rooms r
			JOIN exams e ON e.id = r.exam_id
			WHERE r.exam_id = $1::uuid AND e.tenant_id = $2::uuid
			ORDER BY r.name`, examID, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var r Room
			if err := rows.Scan(&r.ID, &r.Name, &r.ExamID); err != nil {
				return err
			}
			out = append(out, r)
		}
		return rows.Err()
	})
	return out, err
}

type Token struct {
	Token    string  `json:"token"`
	ExamID   string  `json:"exam_id"`
	RoomID   *string `json:"room_id,omitempty"`
	IsActive bool    `json:"is_active"`
}

// genToken: 8 karakter A-Z0-9 tanpa I/O/0/1 (mudah dibaca/diketik).
func genToken() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "T" + time.Now().UTC().Format("060102150405")
	}
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b)
}

// IssueToken: token GLOBAL unik (tenant ditemukan dari token saat login guest).
func (s *Store) IssueToken(ctx context.Context, tenantID, examID string, roomID *string) (*Token, error) {
	var t Token
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		for attempt := 0; attempt < 5; attempt++ {
			tok := genToken()
			err := tx.QueryRow(ctx, `
				INSERT INTO exam_tokens (token, tenant_id, exam_id, room_id)
				VALUES ($1, $2::uuid, $3::uuid, $4::uuid)
				ON CONFLICT (token) DO NOTHING
				RETURNING token, exam_id::text, room_id::text, is_active`,
				tok, tenantID, examID, roomID,
			).Scan(&t.Token, &t.ExamID, &t.RoomID, &t.IsActive)
			if err == nil {
				return nil
			}
			if errors.Is(err, pgx.ErrNoRows) {
				continue // tabrakan kode: coba lagi
			}
			return err
		}
		return errors.New("gagal membuat token unik, coba lagi")
	})
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) ListTokens(ctx context.Context, tenantID, examID string) ([]Token, error) {
	out := []Token{}
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT t.token, t.exam_id::text, t.room_id::text, t.is_active
			FROM exam_tokens t JOIN exams e ON e.id = t.exam_id
			WHERE t.exam_id = $1::uuid AND e.tenant_id = $2::uuid
			ORDER BY t.created_at DESC`, examID, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var t Token
			if err := rows.Scan(&t.Token, &t.ExamID, &t.RoomID, &t.IsActive); err != nil {
				return err
			}
			out = append(out, t)
		}
		return rows.Err()
	})
	return out, err
}

// RevokeToken: nonaktifkan (riwayat login tetap terlacak).
func (s *Store) RevokeToken(ctx context.Context, tenantID, token string) error {
	return s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE exam_tokens t SET is_active = false
			FROM exams e WHERE t.exam_id = e.id
			  AND t.token = $1 AND e.tenant_id = $2::uuid AND t.is_active`, token, tenantID)
		return requireOne(tag, err)
	})
}

// ---- penugasan & peserta (FR-5.6 / FR-4.2) ----

type Assignment struct {
	TargetType string `json:"target_type"` // class | student
	ClassID    string `json:"class_id,omitempty"`
	StudentID  string `json:"student_id,omitempty"`
}

// SetAssignments mengganti seluruh penugasan lalu resolving ulang peserta:
// kelas → seluruh siswa aktif di kelas itu; siswa tunggal → baris sendiri.
// Peserta manual (source='manual') tidak tersentuh.
func (s *Store) SetAssignments(ctx context.Context, tenantID, examID string, list []Assignment) error {
	return s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT 1 FROM exams WHERE id = $1::uuid AND tenant_id = $2::uuid FOR UPDATE`,
			examID, tenantID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM exam_assignments WHERE exam_id = $1::uuid`, examID); err != nil {
			return err
		}
		for _, a := range list {
			switch a.TargetType {
			case "class":
				if a.ClassID == "" {
					return errors.New("target kelas butuh class_id")
				}
				if _, err := tx.Exec(ctx, `
					INSERT INTO exam_assignments (tenant_id, exam_id, target_type, class_id)
					VALUES ($1::uuid, $2::uuid, 'class', $3::uuid)`, tenantID, examID, a.ClassID); err != nil {
					return err
				}
			case "student":
				if a.StudentID == "" {
					return errors.New("target siswa butuh student_id")
				}
				if _, err := tx.Exec(ctx, `
					INSERT INTO exam_assignments (tenant_id, exam_id, target_type, student_id)
					VALUES ($1::uuid, $2::uuid, 'student', $3::uuid)`, tenantID, examID, a.StudentID); err != nil {
					return err
				}
			default:
				return errors.New("target_type harus class|student")
			}
		}
		// Resolving: hapus peserta lama bersumber assignment lalu isi ulang.
		if _, err := tx.Exec(ctx, `
			DELETE FROM exam_participants WHERE exam_id = $1::uuid AND source = 'assignment'`, examID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO exam_participants (tenant_id, exam_id, student_id, source)
			SELECT $1::uuid, $2::uuid, s.id, 'assignment'
			FROM students s
			JOIN exam_assignments a ON a.exam_id = $2::uuid
			  AND ((a.target_type = 'class' AND a.class_id = s.class_id)
			    OR (a.target_type = 'student' AND a.student_id = s.id))
			WHERE s.status = 'active'
			ON CONFLICT (exam_id, student_id) DO NOTHING`, tenantID, examID)
		return err
	})
}

type Participant struct {
	StudentID    string  `json:"student_id"`
	StudentCode  string  `json:"student_code"`
	Name         string  `json:"name"`
	ClassName    *string `json:"class_name,omitempty"`
	Source       string  `json:"source"`
	ExtraTimeSec int     `json:"extra_time_seconds"`
}

func (s *Store) ListParticipants(ctx context.Context, tenantID, examID string) ([]Participant, error) {
	out := []Participant{}
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT p.student_id::text, s.student_code, s.name, c.name, p.source, p.extra_time_seconds
			FROM exam_participants p
			JOIN students s ON s.id = p.student_id
			LEFT JOIN classes c ON c.id = s.class_id
			WHERE p.exam_id = $1::uuid AND p.tenant_id = $2::uuid
			ORDER BY s.name`, examID, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var p Participant
			if err := rows.Scan(&p.StudentID, &p.StudentCode, &p.Name, &p.ClassName, &p.Source, &p.ExtraTimeSec); err != nil {
				return err
			}
			out = append(out, p)
		}
		return rows.Err()
	})
	return out, err
}
