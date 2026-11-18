// Sisi siswa (S11): daftar ujian, mulai ujian, login tamu token.
// Aturan (FR-5.1–5.3, AC-6): jendela waktu & batas percobaan divalidasi
// server; deadline & shuffle_seed ditetapkan server; kunci jawaban tidak
// pernah keluar. Satu sesi aktif per siswa per ujian (FR-1.6).
package exam

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
	"strconv"
	"time"

	"github.com/flowcbt/backend/internal/httpx"
	questionbank "github.com/flowcbt/backend/internal/questionbank"

	"github.com/jackc/pgx/v5"
)

// ---- bentuk data untuk klien siswa (tanpa kunci & rubrik) ----

type ClientOption struct {
	ID   string `json:"id"`
	HTML string `json:"html"`
}

// MatchingChoices: pilihan menjodohkan untuk siswa — hanya teks, tanpa tanda
// pasangan benar (kunci tetap di server).
type MatchingChoices struct {
	Left  []ClientOption `json:"left"`
	Right []ClientOption `json:"right"`
}

type ClientQuestion struct {
	ID             string           `json:"id"`
	OrderNo        int              `json:"order_no"`
	Type           string           `json:"type"`
	StimulusID     *string          `json:"stimulus_id,omitempty"`
	Body           string           `json:"body"`
	Points         float64          `json:"points"`
	Options        []ClientOption   `json:"options,omitempty"`
	Matching       *MatchingChoices `json:"matching,omitempty"`
	ShuffleOptions bool             `json:"shuffle_options"`
	MediaIDs       []string         `json:"media_ids,omitempty"`
}

// clientQuestions membuang kunci & rubrik sebelum data keluar ke siswa.
func clientQuestions(qs []questionbank.Question) []ClientQuestion {
	out := make([]ClientQuestion, 0, len(qs))
	for _, q := range qs {
		opts := make([]ClientOption, 0, len(q.Options))
		for _, o := range q.Options {
			opts = append(opts, ClientOption{ID: o.ID, HTML: o.HTML})
		}
		var match *MatchingChoices
		if mo, ok := q.MatchingOptions.(map[string]any); ok {
			match = &MatchingChoices{
				Left:  choiceList(mo["left"]),
				Right: choiceList(mo["right"]),
			}
		}
		out = append(out, ClientQuestion{
			ID: q.ID, OrderNo: q.OrderNo, Type: q.Type, StimulusID: q.StimulusID,
			Body: q.Body, Points: q.Points, Options: opts, Matching: match,
			ShuffleOptions: q.ShuffleOptions, MediaIDs: q.MediaIDs,
		})
	}
	return out
}

// choiceList: [{id,text}] any → []ClientOption; html disihkan kosong.
func choiceList(v any) []ClientOption {
	items, ok := v.([]any)
	if !ok {
		return []ClientOption{}
	}
	out := make([]ClientOption, 0, len(items))
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		text, _ := m["text"].(string)
		out = append(out, ClientOption{ID: id, HTML: text})
	}
	return out
}

// ---- endpoint data ----

type StudentExam struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Status          string  `json:"status"` // published | paused
	OpenAt          *string `json:"open_at,omitempty"`
	CloseAt         *string `json:"close_at,omitempty"`
	DurationMin     *int    `json:"duration_minutes,omitempty"`
	LoginMode       string  `json:"login_mode"`
	AttemptsDone    int     `json:"attempts_done"`
	MaxAttempts     *int    `json:"max_attempts,omitempty"`
	ExtraTimeSec    int     `json:"extra_time_seconds"`
	ActiveAttemptID string  `json:"active_attempt_id,omitempty"` // lanjutkan, bukan mulai baru
	ShowScoreNow    bool    `json:"show_score_immediately"`
}

// ListStudentExams: ujian yang mengikutsertakan siswa ini (FR-5.6),
// hanya yang sedang/akan berjalan.
func (s *Store) ListStudentExams(ctx context.Context, tenantID, studentID string) ([]StudentExam, error) {
	out := []StudentExam{}
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT e.id::text, e.name, e.status, e.open_at::text, e.close_at::text,
			       e.duration_minutes, e.login_mode, e.max_attempts, p.extra_time_seconds,
			       e.show_score_immediately,
			       COALESCE(cnt.n, 0),
			       COALESCE(act.id::text, '')
			FROM exam_participants p
			JOIN exams e ON e.id = p.exam_id
			LEFT JOIN LATERAL (
				SELECT count(*) AS n FROM attempts a
				WHERE a.exam_id = p.exam_id AND a.student_id = p.student_id
			) cnt ON true
			LEFT JOIN LATERAL (
				SELECT a.id FROM attempts a
				WHERE a.exam_id = p.exam_id AND a.student_id = p.student_id
				  AND a.status = 'in_progress'
				ORDER BY a.attempt_no DESC LIMIT 1
			) act ON true
			WHERE p.student_id = $1::uuid AND p.tenant_id = $2::uuid
			  AND e.status IN ('published', 'paused')
			ORDER BY e.open_at NULLS LAST`, studentID, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var x StudentExam
			if err := rows.Scan(&x.ID, &x.Name, &x.Status, &x.OpenAt, &x.CloseAt,
				&x.DurationMin, &x.LoginMode, &x.MaxAttempts, &x.ExtraTimeSec,
				&x.ShowScoreNow, &x.AttemptsDone, &x.ActiveAttemptID); err != nil {
				return err
			}
			out = append(out, x)
		}
		return rows.Err()
	})
	return out, err
}

// StartAttempt: mulai (atau lanjutkan) pengerjaan. Semua aturan dicek dalam
// satu transaksi; waktu menurut now() database.
func (s *Store) StartAttempt(ctx context.Context, tenantID, studentID, examID, deviceHash string) (*StartResult, error) {
	var res *StartResult
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		// 1. Ujian harus sedang tayang (published).
		var e Exam
		if err := tx.QueryRow(ctx, `
			SELECT `+examCols+` FROM exams
			WHERE id = $1::uuid AND tenant_id = $2::uuid`, examID, tenantID,
		).Scan(&e.ID, &e.QuestionSetID, &e.SetVersionID, &e.Name, &e.Status,
			&e.OpenAt, &e.CloseAt, &e.DurationMin, &e.ClosePolicy, &e.LoginMode, &e.MaxAttempts,
			&e.ScorePolicy, &e.PassingScore, &e.ShowScoreNow, &e.ShowReview, &e.ShowExplain,
			&e.ShuffleQ, &e.ShuffleO, &e.FreeNav, &e.RequireAll,
			&e.CreatedBy, &e.CreatedAt, &e.UpdatedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if e.Status == "paused" {
			return conflict("EXAM_PAUSED", "ujian sedang dijeda guru — tunggu sebentar")
		}
		if e.Status != "published" {
			return conflict("EXAM_NOT_OPEN", "ujian tidak sedang dibuka")
		}

		// 2. Peserta terdaftar (FR-5.6).
		var extraTime int
		var attemptsOverride *int
		err := tx.QueryRow(ctx, `
			SELECT extra_time_seconds, attempts_override FROM exam_participants
			WHERE exam_id = $1::uuid AND student_id = $2::uuid`, examID, studentID,
		).Scan(&extraTime, &attemptsOverride)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}

		// 3. Jendela waktu (FR-5.1) — now() server, bukan klien.
		var now time.Time
		if err := tx.QueryRow(ctx, `SELECT now()`).Scan(&now); err != nil {
			return err
		}
		if e.OpenAt != nil {
			open, err := parseTime(*e.OpenAt)
			if err != nil {
				return err
			}
			if now.Before(open) {
				return conflict("EXAM_NOT_OPEN", "ujian belum dibuka — mulai "+
					open.Format("02/01 15:04"))
			}
		}
		var closeAt *time.Time
		if e.CloseAt != nil {
			c, err := parseTime(*e.CloseAt)
			if err != nil {
				return err
			}
			closeAt = &c
			if now.After(c) {
				return conflict("EXAM_ENDED", "masa ujian sudah berakhir")
			}
		}

		// 4. Batas percobaan (FR-5.3, AC-6 → 409).
		var done int
		if err := tx.QueryRow(ctx, `
			SELECT count(*) FROM attempts WHERE exam_id = $1::uuid AND student_id = $2::uuid`,
			examID, studentID).Scan(&done); err != nil {
			return err
		}
		maxAttempts := e.MaxAttempts
		if attemptsOverride != nil {
			maxAttempts = attemptsOverride
		}
		if maxAttempts != nil && done >= *maxAttempts {
			return conflict("ATTEMPT_LIMIT", "batas percobaan tercapai ("+
				strconv.Itoa(done)+"/"+strconv.Itoa(*maxAttempts)+")")
		}

		// 5. Lanjutkan attempt berjalan bila ada (satu sesi aktif, FR-1.6).
		var existingID string
		var existingDeadline time.Time
		err = tx.QueryRow(ctx, `
			SELECT id::text, deadline_at FROM attempts
			WHERE exam_id = $1::uuid AND student_id = $2::uuid AND status = 'in_progress'
			ORDER BY attempt_no DESC LIMIT 1`, examID, studentID,
		).Scan(&existingID, &existingDeadline)
		if err == nil {
			var err2 error
			res, err2 = s.resultFor(ctx, tenantID, existingID, e, existingDeadline, 0)
			return err2
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		// Sesi aktif lain pada attempt lama → kebijakan single_session.
		var activeSessions int
		if err := tx.QueryRow(ctx, `
			SELECT count(*) FROM attempt_sessions s
			JOIN attempts a ON a.id = s.attempt_id
			WHERE a.exam_id = $1::uuid AND a.student_id = $2::uuid AND s.revoked_at IS NULL`,
			examID, studentID).Scan(&activeSessions); err != nil {
			return err
		}
		if activeSessions > 0 {
			var policy string
			if err := tx.QueryRow(ctx, `SELECT single_session_policy FROM exams WHERE id = $1::uuid`,
				examID).Scan(&policy); err != nil {
				return err
			}
			if policy == "reject_new" {
				return conflict("SESSION_ACTIVE", "masih ada sesi aktif di perangkat lain")
			}
			if _, err := tx.Exec(ctx, `
				UPDATE attempt_sessions s SET revoked_at = now(), revoked_by = 'new_login'
				FROM attempts a
				WHERE s.attempt_id = a.id AND a.exam_id = $1::uuid AND a.student_id = $2::uuid
				  AND s.revoked_at IS NULL`, examID, studentID); err != nil {
				return err
			}
		}

		// 6. Deadline (FR-5.2): hard → dipotong close_at; grace → durasi habis.
		if e.DurationMin == nil {
			return conflict("EXAM_MISCONFIGURED", "durasi ujian belum diatur")
		}
		deadline := now.Add(time.Duration(*e.DurationMin)*time.Minute + time.Duration(extraTime)*time.Second)
		if e.ClosePolicy == "hard" && closeAt != nil && deadline.After(*closeAt) {
			deadline = *closeAt
		}
		seed, err := randSeed()
		if err != nil {
			return err
		}
		var attemptID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO attempts (tenant_id, exam_id, set_version_id, student_id, attempt_no,
			                      deadline_at, shuffle_seed, extra_time_seconds)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6, $7, $8)
			RETURNING id::text`,
			tenantID, examID, e.SetVersionID, studentID, done+1, deadline, seed, extraTime,
		).Scan(&attemptID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO attempt_sessions (tenant_id, attempt_id, device_hash)
			VALUES ($1::uuid, $2::uuid, $3)`, tenantID, attemptID, deviceHash); err != nil {
			return err
		}
		res, err = s.resultFor(ctx, tenantID, attemptID, e, deadline, *e.DurationMin*60)
		return err
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// resultFor: susun hasil start (soal versi ujian, via cache bila tersedia).
func (s *Store) resultFor(ctx context.Context, tenantID, attemptID string, e Exam,
	deadline time.Time, durationSeconds int) (*StartResult, error) {

	res := &StartResult{
		AttemptID:       attemptID,
		ExamID:          e.ID,
		ExamName:        e.Name,
		ExamStatus:      e.Status,
		DeadlineAt:      deadline.UTC().Format(time.RFC3339),
		DurationSeconds: durationSeconds,
		ServerTime:      time.Now().UTC().Format(time.RFC3339),
		ShowScoreNow:    e.ShowScoreNow,
		ShuffleQuestion: e.ShuffleQ,
		FreeNavigation:  e.FreeNav,
		RequireAll:      e.RequireAll,
	}
	qs, err := s.questionsForVersion(ctx, tenantID, e.SetVersionID)
	if err != nil {
		return nil, err
	}
	res.Questions = qs
	return res, nil
}

// questionsForVersion: soal klien tanpa kunci — cache Redis per versi.
// Versi terbit immutable (FR-4.8) sehingga cache tanpa invalidasi aman.
func (s *Store) questionsForVersion(ctx context.Context, tenantID, versionID string) ([]ClientQuestion, error) {
	if versionID == "" {
		return nil, errors.New("paket soal ujian belum memiliki versi")
	}
	if s.cache != nil {
		if b, ok := s.cache.Get(ctx, versionID); ok {
			var out []ClientQuestion
			if json.Unmarshal(b, &out) == nil && len(out) > 0 {
				return out, nil
			}
		}
	}
	qs, err := s.qb.QuestionsOfVersion(ctx, tenantID, versionID)
	if err != nil {
		return nil, err
	}
	out := clientQuestions(qs)
	if len(out) == 0 {
		return out, errors.New("versi ujian tidak memiliki soal")
	}
	if s.cache != nil {
		if b, err := json.Marshal(out); err == nil {
			s.cache.Set(ctx, versionID, b)
		}
	}
	return out, nil
}

func conflict(code, msg string) error {
	return httpx.AppErr(409, code, msg, nil)
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

func randSeed() (int64, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return 0, err
	}
	return n.Int64(), nil
}

type StartResult struct {
	AttemptID       string           `json:"attempt_id"`
	ExamID          string           `json:"exam_id"`
	ExamName        string           `json:"exam_name"`
	ExamStatus      string           `json:"exam_status"`
	DeadlineAt      string           `json:"deadline_at"`
	DurationSeconds int              `json:"duration_seconds"`
	ServerTime      string           `json:"server_time"`
	ShowScoreNow    bool             `json:"show_score_immediately"`
	ShuffleQuestion bool             `json:"shuffle_questions"`
	FreeNavigation  bool             `json:"free_navigation"`
	RequireAll      bool             `json:"require_all_answered"`
	Questions       []ClientQuestion `json:"questions"`
}

// GuestAuth: login tanpa akun (FR-1.3) — token ujian mencukupi. Siswa tamu
// dibuat sekali per token (is_guest) supaya attempt & peserta tetap tervalidasi
// RLS setelah login; data dapat digabung ke akun siswa asli (merged_into_id).
// Token bersifat global → tenant dicari lewat pool sistem (pola login).
func (s *Store) GuestAuth(ctx context.Context, token string) (tenantID, studentID, examID string, err error) {
	// 1. Token aktif → tenant & ujian (token global, butuh BYPASSRLS).
	err = s.db.System.QueryRow(ctx, `
		SELECT t.tenant_id::text, t.exam_id::text
		FROM exam_tokens t
		JOIN exams e ON e.id = t.exam_id
		WHERE t.token = $1 AND t.is_active AND e.status = 'published'`, token,
	).Scan(&tenantID, &examID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", "", httpx.AppErr(401, "TOKEN_INVALID", "token tidak dikenal atau sudah dicabut", nil)
		}
		return "", "", "", err
	}

	// 2. Siswa tamu per token (buat bila belum ada) + peserta.
	err = s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT id::text FROM students
			WHERE tenant_id = $1::uuid AND username = $2 AND is_guest`,
			tenantID, "tamu-"+token,
		).Scan(&studentID)
		if err == nil {
			// Sudah ada — lanjut.
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		} else {
			code, cerr := guestCode(ctx, tx, tenantID)
			if cerr != nil {
				return cerr
			}
			if err := tx.QueryRow(ctx, `
				INSERT INTO students (tenant_id, student_code, name, name_normalized, gender,
				                      username, status, is_guest)
				VALUES ($1::uuid, $2, $3, $3, 'L', $4, 'active', true)
				RETURNING id::text`,
				tenantID, code, "Tamu "+token, "tamu-"+token,
			).Scan(&studentID); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO exam_participants (tenant_id, exam_id, student_id, source)
			VALUES ($1::uuid, $2::uuid, $3::uuid, 'guest')
			ON CONFLICT (exam_id, student_id) DO NOTHING`, tenantID, examID, studentID)
		return err
	})
	if err != nil {
		return "", "", "", err
	}
	return tenantID, studentID, examID, nil
}

// guestCode: kode siswa unik untuk tamu (T-<acak>).
func guestCode(ctx context.Context, tx pgx.Tx, tenantID string) (string, error) {
	for i := 0; i < 5; i++ {
		b := make([]byte, 6)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		const hexd = "0123456789abcdef"
		code := "T-"
		for _, c := range b {
			code += string(hexd[c&0x0f])
		}
		var exists bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM students WHERE tenant_id = $1::uuid AND student_code = $2)`,
			tenantID, code).Scan(&exists); err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}
	return "", errors.New("gagal membuat kode siswa tamu")
}
