// Sinkronisasi jawaban versi dasar (S12) + resume attempt.
// Event append-only idempoten (client_event_id), proyeksi last-write-wins
// per soal ke attempt_answers; event meuwihi deadline ditandai tidak sah
// (AC-5 dasar; toleransi offline penuh di S15–S18).
package exam

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/flowcbt/backend/internal/httpx"
	"github.com/jackc/pgx/v5"
)

// SyncEvent: satu perubahan dari klien.
type SyncEvent struct {
	ClientEventID string          `json:"client_event_id"`
	Type          string          `json:"type"` // answer | flag | nav
	QuestionID    string          `json:"question_id,omitempty"`
	Answer        json.RawMessage `json:"answer,omitempty"`
	Flagged       *bool           `json:"flagged,omitempty"`
	ElapsedMS     int64           `json:"elapsed_ms,omitempty"`
	ClientTS      string          `json:"client_ts,omitempty"`
}

type SyncRequest struct {
	Seq    int64       `json:"seq"`
	Events []SyncEvent `json:"events"`
}

type SyncResponse struct {
	Accepted         int               `json:"accepted"`
	Rejected         int               `json:"rejected"`
	RejectReasons    map[string]string `json:"reject_reasons,omitempty"` // client_event_id → alasan
	LastSeq          int64             `json:"last_seq"`
	RemainingSeconds int               `json:"remaining_seconds"`
	ServerTime       string            `json:"server_time"`
}

const eventTolerance = 10 * time.Second

var allowedEventTypes = map[string]bool{
	"answer": true, "flag": true, "nav": true,
	"focus_lost": true, "online": true, "offline": true, "sync": true,
}

// SyncAnswers memproses batch event dalam satu transaksi.
// ownerStudentID dari JWT — attempt milik siswa lain ditolak 403.
func (s *Store) SyncAnswers(ctx context.Context, tenantID, ownerStudentID, attemptID string, in SyncRequest) (*SyncResponse, error) {
	if len(in.Events) == 0 {
		return nil, httpx.ErrBadRequest("tidak ada event", nil)
	}
	if in.Seq <= 0 {
		return nil, httpx.ErrBadRequest("seq harus bilangan positif", nil)
	}
	// Batas ukuran batch (proteksi sederhana).
	if len(in.Events) > 500 {
		return nil, httpx.ErrBadRequest("terlalu banyak event dalam satu sync (maks 500)", nil)
	}

	res := &SyncResponse{
		RejectReasons: map[string]string{},
		ServerTime:    time.Now().UTC().Format(time.RFC3339),
	}

	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		// 1. Attempt milik siswa & masih berjalan?
		var studentID, setVersionID string
		var status string
		var deadline time.Time
		err := tx.QueryRow(ctx, `
			SELECT student_id::text, set_version_id::text, status, deadline_at
			FROM attempts WHERE id = $1::uuid AND tenant_id = $2::uuid`, attemptID, tenantID,
		).Scan(&studentID, &setVersionID, &status, &deadline)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if studentID != ownerStudentID {
			return httpx.AppErr(403, "FORBIDDEN", "bukan percobaan Anda", nil)
		}

		var now time.Time
		if err := tx.QueryRow(ctx, `SELECT now()`).Scan(&now); err != nil {
			return err
		}
		if status != "in_progress" {
			return httpx.AppErr(409, "ATTEMPT_CLOSED", "percobaan sudah berakhir", nil)
		}
		afterDeadline := now.After(deadline.Add(eventTolerance))

		// 2. Soal yang sah untuk attempt ini (anti injeksi question_id).
		validQuestions := map[string]bool{}
		questionIDs := []string{}
		for _, ev := range in.Events {
			if ev.QuestionID != "" {
				questionIDs = append(questionIDs, ev.QuestionID)
			}
		}
		if len(questionIDs) > 0 {
			rows, err := tx.Query(ctx, `
				SELECT id::text FROM questions
				WHERE set_version_id = $1::uuid AND id = ANY($2::uuid[])`, setVersionID, questionIDs)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err != nil {
					return err
				}
				validQuestions[id] = true
			}
			if err := rows.Err(); err != nil {
				return err
			}
		}

		// 3. Proses event berurutan.
		for _, ev := range in.Events {
			reject := ""
			switch {
			case !allowedEventTypes[ev.Type]:
				reject = "tipe event tidak dikenal"
			case ev.ClientEventID == "":
				reject = "client_event_id kosong"
			case ev.QuestionID != "" && !validQuestions[ev.QuestionID]:
				reject = "soal bukan bagian ujian ini"
			case afterDeadline:
				reject = "lewat tenggat (AC-5)"
			}
			accepted := reject == ""
			tag, err := tx.Exec(ctx, `
				INSERT INTO attempt_events (tenant_id, attempt_id, client_event_id, type,
				                            payload, client_ts, elapsed_ms, seq, accepted, reject_reason)
				VALUES ($1::uuid, $2::uuid, $3, $4, $5::jsonb,
				        NULLIF($6,'')::timestamptz, $7, $8, $9, NULLIF($10,''))
				ON CONFLICT (attempt_id, client_event_id) DO NOTHING`,
				tenantID, attemptID, ev.ClientEventID, ev.Type,
				payloadOf(ev), ev.ClientTS, ev.ElapsedMS, in.Seq, accepted, reject,
			)
			if err != nil {
				return err
			}
			_ = tag
			if !accepted {
				res.Rejected++
				if reject != "" {
					res.RejectReasons[ev.ClientEventID] = reject
				}
				continue
			}
			if ev.Type == "flag" && ev.QuestionID != "" {
				_, err = tx.Exec(ctx, `
					INSERT INTO attempt_answers (tenant_id, attempt_id, question_id, flagged, seq)
					VALUES ($1::uuid, $2::uuid, $3::uuid, COALESCE($4, true), $5)
					ON CONFLICT (attempt_id, question_id) DO UPDATE
					SET flagged = EXCLUDED.flagged, seq = GREATEST(EXCLUDED.seq, attempt_answers.seq),
					    updated_at = now()`, tenantID, attemptID, ev.QuestionID, ev.Flagged, in.Seq)
				if err != nil {
					return err
				}
				res.Accepted++
				continue
			}
			if ev.Type == "answer" && ev.QuestionID != "" {
				if string(ev.Answer) == "" || string(ev.Answer) == "null" {
					res.Accepted++ // event sah tapi tanpa isi — tidak mengubah jawaban
					continue
				}
				_, err = tx.Exec(ctx, `
					INSERT INTO attempt_answers (tenant_id, attempt_id, question_id, answer_json, seq)
					VALUES ($1::uuid, $2::uuid, $3::uuid, $4::jsonb, $5)
					ON CONFLICT (attempt_id, question_id) DO UPDATE
					SET answer_json = EXCLUDED.answer_json,
					    seq = GREATEST(EXCLUDED.seq, attempt_answers.seq),
					    updated_at = now()`, tenantID, attemptID, ev.QuestionID, string(ev.Answer), in.Seq)
				if err != nil {
					return err
				}
				res.Accepted++
				continue
			}
			res.Accepted++ // nav / lainnya: cukup event log
		}

		// 4. last_seq attempt.
		if _, err := tx.Exec(ctx, `
			UPDATE attempts SET last_seq = GREATEST(last_seq, $2), updated_at = now()
			WHERE id = $1::uuid`, attemptID, in.Seq); err != nil {
			return err
		}
		res.LastSeq = in.Seq

		// 5. Sisa waktu.
		remaining := int(deadline.Sub(now).Seconds())
		if remaining < 0 {
			remaining = 0
		}
		res.RemainingSeconds = remaining
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func payloadOf(ev SyncEvent) string {
	if ev.Type == "answer" && len(ev.Answer) > 0 {
		return string(ev.Answer)
	}
	if ev.Flagged != nil {
		b, _ := json.Marshal(map[string]bool{"flagged": *ev.Flagged})
		return string(b)
	}
	return "{}"
}

// GetAttempt (resume): soal + jawaban tersimpan. Kunci tidak pernah keluar.
type AttemptState struct {
	AttemptID       string                 `json:"attempt_id"`
	ExamID          string                 `json:"exam_id"`
	ExamName        string                 `json:"exam_name"`
	Status          string                 `json:"status"`
	DeadlineAt      string                 `json:"deadline_at"`
	ServerTime      string                 `json:"server_time"`
	ShowScoreNow    bool                   `json:"show_score_immediately"`
	ShuffleQuestion bool                   `json:"shuffle_questions"`
	FreeNavigation  bool                   `json:"free_navigation"`
	RequireAll      bool                   `json:"require_all_answered"`
	Questions       []ClientQuestion       `json:"questions"`
	Answers         map[string]SavedAnswer `json:"answers"`
}

type SavedAnswer struct {
	Answer  json.RawMessage `json:"answer,omitempty"`
	Flagged bool            `json:"flagged"`
}

// GetAttemptState: muat ulang pengerjaan (refresh browser / ganti perangkat
// setelah sesi lama dicabut dan sesi baru dibuat lewat start).
// ownerStudentID dari JWT — attempt milik siswa lain ditolak 403.
func (s *Store) GetAttemptState(ctx context.Context, tenantID, ownerStudentID, attemptID string) (*AttemptState, error) {
	var st AttemptState
	var setVersionID string
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT a.id::text, a.exam_id::text, e.name, a.status, a.deadline_at,
			       e.show_score_immediately, e.shuffle_questions, e.free_navigation,
			       e.require_all_answered, COALESCE(a.set_version_id::text, '')
			FROM attempts a JOIN exams e ON e.id = a.exam_id
			WHERE a.id = $1::uuid AND a.tenant_id = $2::uuid`, attemptID, tenantID,
		).Scan(&st.AttemptID, &st.ExamID, &st.ExamName, &st.Status, &st.DeadlineAt,
			&st.ShowScoreNow, &st.ShuffleQuestion, &st.FreeNavigation, &st.RequireAll,
			&setVersionID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if st.AttemptID != "" {
			var owner string
			if err := tx.QueryRow(ctx,
				`SELECT student_id::text FROM attempts WHERE id = $1::uuid`, attemptID,
			).Scan(&owner); err == nil && owner != ownerStudentID {
				return httpx.AppErr(403, "FORBIDDEN", "bukan percobaan Anda", nil)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	qs, err := s.questionsForVersion(ctx, tenantID, setVersionID)
	if err != nil {
		return nil, err
	}
	st.Questions = qs
	st.ServerTime = time.Now().UTC().Format(time.RFC3339)
	st.Answers = map[string]SavedAnswer{}
	err = s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT question_id::text, COALESCE(answer_json::text, ''), flagged
			FROM attempt_answers WHERE attempt_id = $1::uuid`, attemptID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var qid string
			var a SavedAnswer
			if err := rows.Scan(&qid, &a.Answer, &a.Flagged); err != nil {
				return err
			}
			st.Answers[qid] = a
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return &st, nil
}
