// Koreksi manual & penyelesaian nilai (S14, FR-8): antrean uraian per ujian,
// detail jawaban per attempt dengan rubrik, nilai manual per soal, override
// skor + alasan (diaudit), re-grade, dan agregasi ulang nilai attempt.
package exam

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/flowcbt/backend/internal/grading"
	"github.com/flowcbt/backend/internal/httpx"
	"github.com/flowcbt/backend/internal/platform"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---- antrean koreksi per ujian ----

type GradingQueueItem struct {
	AttemptID   string     `json:"attempt_id"`
	StudentName string     `json:"student_name"`
	AttemptNo   int        `json:"attempt_no"`
	Status      string     `json:"status"`
	Provisional bool       `json:"provisional"`
	FinalScore  *float64   `json:"final_score,omitempty"`
	NeedsGrade  int        `json:"needs_grade"`
	SubmittedAt *time.Time `json:"submitted_at,omitempty"`
}

// GradingQueue: daftar attempt yang sudah dikirim per ujian untuk layar
// koreksi guru; needs_grade = jumlah soal esai yang masih tanpa nilai.
func (s *Store) GradingQueue(ctx context.Context, tenantID, examID string) ([]GradingQueueItem, error) {
	out := []GradingQueueItem{}
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT a.id::text, s.name, a.attempt_no, a.status, a.score_provisional,
			       a.final_score::float8, a.submitted_at,
			       (SELECT count(*) FROM attempt_answers aa JOIN questions q ON q.id = aa.question_id
			        WHERE aa.attempt_id = a.id AND q.type = 'essay' AND aa.score IS NULL)::int
			FROM attempts a JOIN students s ON s.id = a.student_id
			WHERE a.exam_id = $1::uuid AND a.tenant_id = $2::uuid
			  AND a.status IN ('submitted','awaiting_manual_grading','graded')
			ORDER BY a.submitted_at DESC NULLS LAST`, examID, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var it GradingQueueItem
			var submitted *time.Time
			if err := rows.Scan(&it.AttemptID, &it.StudentName, &it.AttemptNo, &it.Status,
				&it.Provisional, &it.FinalScore, &submitted, &it.NeedsGrade); err != nil {
				return err
			}
			it.SubmittedAt = submitted
			out = append(out, it)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ---- detail koreksi per attempt ----

type GradingQuestion struct {
	QuestionID     string   `json:"question_id"`
	OrderNo        int      `json:"order_no"`
	Type           string   `json:"type"`
	Prompt         string   `json:"prompt"`
	Points         float64  `json:"points"`
	Rubric         string   `json:"rubric,omitempty"`
	StudentAnswer  any      `json:"answer"` // anpa selalu json, lihat submit
	AutoScore      *float64 `json:"auto_score,omitempty"`
	Score          *float64 `json:"score,omitempty"`
	GradedBy       *string  `json:"graded_by,omitempty"`
	GraderComment  string   `json:"grader_comment,omitempty"`
	OverrideReason string   `json:"override_reason,omitempty"`
	NeedsGrade     bool     `json:"needs_grade"`
}

type GradingDetail struct {
	AttemptID   string            `json:"attempt_id"`
	ExamID      string            `json:"exam_id"`
	StudentName string            `json:"student_name"`
	Status      string            `json:"status"`
	RawScore    float64           `json:"raw_score"`
	MaxScore    float64           `json:"max_score"`
	FinalScore  float64           `json:"final_score"`
	Provisional bool              `json:"provisional"`
	NeedsGrade  int               `json:"needs_grade"`
	SubmittedAt *time.Time        `json:"submitted_at,omitempty"`
	Questions   []GradingQuestion `json:"questions"`
}

// GradingDetail: header attempt + setiap soal versi yang pernah ditulis
// attempt ini, termasuk jawaban siswa, rubrik, dan skor saat ini.
func (s *Store) GradingDetail(ctx context.Context, tenantID, attemptID string) (*GradingDetail, error) {
	var d GradingDetail
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT a.id::text, a.exam_id::text, s.name, a.status,
			       COALESCE(a.raw_score::float8,0), COALESCE(a.max_score::float8,0),
			       COALESCE(a.final_score::float8,0), a.score_provisional, a.submitted_at
			FROM attempts a JOIN students s ON s.id = a.student_id
			WHERE a.id = $1::uuid AND a.tenant_id = $2::uuid`, attemptID, tenantID,
		).Scan(&d.AttemptID, &d.ExamID, &d.StudentName, &d.Status,
			&d.RawScore, &d.MaxScore, &d.FinalScore, &d.Provisional, &d.SubmittedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
			SELECT q.id::text, q.order_no, q.type, q.body_html, q.points::float8,
			       COALESCE(q.rubric_json::text,''), aa.answer_json,
			       aa.auto_score::float8, aa.score::float8, aa.graded_by,
			       COALESCE(aa.grader_comment,''), COALESCE(aa.override_reason,'')
			FROM attempts a
			JOIN questions q ON q.set_version_id = a.set_version_id
			LEFT JOIN attempt_answers aa
			       ON aa.question_id = q.id AND aa.attempt_id = a.id
			WHERE a.id = $1::uuid AND a.tenant_id = $2::uuid
			ORDER BY q.order_no`, attemptID, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		d.Questions = []GradingQuestion{}
		for rows.Next() {
			var gq GradingQuestion
			var rubric string
			var answerJSON []byte
			if err := rows.Scan(&gq.QuestionID, &gq.OrderNo, &gq.Type, &gq.Prompt, &gq.Points,
				&rubric, &answerJSON, &gq.AutoScore, &gq.Score, &gq.GradedBy,
				&gq.GraderComment, &gq.OverrideReason); err != nil {
				return err
			}
			if len(answerJSON) > 0 {
				var decoded any
				if json.Unmarshal(answerJSON, &decoded) == nil {
					gq.StudentAnswer = decoded
				}
			}
			gq.Rubric = rubric
			gq.NeedsGrade = gq.Type == "essay" && gq.Score == nil
			d.Questions = append(d.Questions, gq)
			if gq.NeedsGrade {
				d.NeedsGrade++
			}
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// ---- nilai manual, override, re-grade (FR-5.7/8.2) ----

type gradeReq struct {
	Score    *float64 `json:"score"`
	Comment  string   `json:"comment"`
	Reason   string   `json:"reason"` // wajib bila override nilai otomatis
	GradedBy string   `json:"-"`      // diisi handler: user id guru
}

// SetAnswerScore: guru memberi nilai satu soal. Bila soal itu sebelumnya
// dinilai otomatis (punya is_correct), operasi dianggap OVERRIDE dan alasan
// wajib diisi; bila esai (belum bernilai) → penilaian manual biasa.
// Keduanya mengubah kolom score dan memicu agregasi ulang nilai attempt.
func (s *Store) SetAnswerScore(ctx context.Context, tenantID, graderID, attemptID, questionID string, in *gradeReq) (bool, error) {
	if in.Score == nil {
		return false, httpx.ErrBadRequest("score wajib diisi", nil)
	}
	var wasOverride bool
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		// Kunci attempt; harus sudah dikirim (bukan in_progress).
		var status string
		err := tx.QueryRow(ctx, `SELECT status FROM attempts
			WHERE id = $1::uuid AND tenant_id = $2::uuid FOR UPDATE`,
			attemptID, tenantID).Scan(&status)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if status == "in_progress" || status == "pending_sync" {
			return httpx.AppErr(409, "ATTEMPT_OPEN", "percobaan masih berjalan", nil)
		}
		// Batas skor: points soal.
		var points float64
		var prevAuto *bool
		err = tx.QueryRow(ctx, `
			SELECT q.points::float8, (aa.graded_by = 'auto' AND aa.is_correct IS NOT NULL)
			FROM questions q
			JOIN attempts a ON a.set_version_id = q.set_version_id AND a.id = $2::uuid
			LEFT JOIN attempt_answers aa ON aa.question_id = q.id AND aa.attempt_id = $2::uuid
			WHERE q.id = $3::uuid AND a.tenant_id = $1::uuid`,
			tenantID, attemptID, questionID).Scan(&points, &prevAuto)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if *in.Score < 0 || *in.Score > points {
			return httpx.AppErr(422, "SCORE_RANGE",
				"skor harus antara 0 dan "+formatPoints(points), nil)
		}
		if prevAuto != nil && *prevAuto {
			wasOverride = true
			if in.Reason == "" {
				return httpx.AppErr(422, "REASON_REQUIRED",
					"override nilai otomatis wajib menyertakan alasan", nil)
			}
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO attempt_answers (tenant_id, attempt_id, question_id, score,
			                             graded_by, grader_id, grader_comment, override_reason)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4::numeric,
			        CASE WHEN $8 THEN 'auto' ELSE 'manual' END, $5::uuid, $6, $7)
			ON CONFLICT (attempt_id, question_id) DO UPDATE SET
			  score = $4::numeric,
			  graded_by = CASE WHEN $8 THEN attempt_answers.graded_by ELSE 'manual' END,
			  grader_id = $5::uuid,
			  grader_comment = $6,
			  override_reason = CASE WHEN $8 THEN $7 ELSE attempt_answers.override_reason END,
			  updated_at = now()`,
			tenantID, attemptID, questionID, *in.Score, graderID, in.Comment, in.Reason, wasOverride,
		); err != nil {
			return err
		}
		return recalcAttemptTx(ctx, tx, attemptID)
	})
	if err != nil {
		return false, err
	}
	return wasOverride, nil
}

func formatPoints(p float64) string {
	return strconv.FormatFloat(p, 'f', -1, 64)
}

// RegradeAttempt: nilai ulang seluruh soal objektif dengan kunci versi saat
// ini (mis. kunci salah lama sudah dikoreksi guru kelas). Baris yang ditandai
// manual atau pernah di-override TIDAK disentuh. Mengembalikan jumlah soal
// yang dinilai ulang.
func (s *Store) RegradeAttempt(ctx context.Context, tenantID, attemptID string) (int, error) {
	count := 0
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		var status, versionID string
		err := tx.QueryRow(ctx, `SELECT status, set_version_id::text FROM attempts
			WHERE id = $1::uuid AND tenant_id = $2::uuid FOR UPDATE`,
			attemptID, tenantID).Scan(&status, &versionID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if status == "in_progress" || status == "pending_sync" {
			return httpx.AppErr(409, "ATTEMPT_OPEN", "percobaan masih berjalan", nil)
		}
		specs, err := loadQuestionSpecsTx(ctx, tx, versionID)
		if err != nil {
			return err
		}
		saved, err := loadSavedAnswersTx(ctx, tx, attemptID)
		if err != nil {
			return err
		}
		for _, q := range specs {
			if q.Type == grading.TypeEssay {
				continue // penilaian uraian tetap milik guru
			}
			ans, ok := saved[q.ID]
			if !ok {
				continue // tanpa baris jawaban → biarkan (belum dijawab, skor dibiarkan 0 oleh INSERT grading awal)
			}
			// Jangan sentuh baris manual / override.
			var flag string
			if err := tx.QueryRow(ctx, `SELECT COALESCE(graded_by,'') || '|' || COALESCE(override_reason,'')
				FROM attempt_answers WHERE attempt_id = $1::uuid AND question_id = $2::uuid FOR UPDATE`,
				attemptID, q.ID).Scan(&flag); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					continue
				}
				return err
			}
			if flag != "auto|" {
				continue // dinilai manual atau pernah di-override: milik guru
			}
			var res grading.Result
			if ans.Answer == nil {
				res = grading.Score(q, nil)
			} else {
				var decoded any
				if err := json.Unmarshal(ans.Answer, &decoded); err != nil {
					res = grading.Score(q, nil)
				} else {
					res = grading.Score(q, decoded)
				}
			}
			var isCorrect any
			if res.IsCorrect != nil {
				isCorrect = *res.IsCorrect
			}
			if _, err := tx.Exec(ctx, `
				UPDATE attempt_answers SET is_correct = $3, auto_score = $4::numeric,
				       score = $4::numeric, graded_by = 'auto', updated_at = now()
				WHERE attempt_id = $1::uuid AND question_id = $2::uuid`,
				attemptID, q.ID, isCorrect, res.Score); err != nil {
				return err
			}
			count++
		}
		return recalcAttemptTx(ctx, tx, attemptID)
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

// recalcAttemptTx: agregasi ulang nilai attempt dari attempt_answers
// (FR-8.2). Status kembali graded bila tak ada esai yang masih kosong.
func recalcAttemptTx(ctx context.Context, tx pgx.Tx, attemptID string) error {
	var pending, maxSum, earned float64
	err := tx.QueryRow(ctx, `
		WITH specs AS (
		  SELECT q.id, q.points::float8 AS points,
		         (q.type = 'essay') AS is_essay,
		         aa.score::float8 AS score
		  FROM attempts a
		  JOIN questions q ON q.set_version_id = a.set_version_id
		  LEFT JOIN attempt_answers aa
		         ON aa.question_id = q.id AND aa.attempt_id = a.id
		  WHERE a.id = $1::uuid
		)
		SELECT COALESCE(sum(score) FILTER (WHERE score IS NOT NULL), 0),
		       COALESCE(sum(points),0),
		       count(*) FILTER (WHERE is_essay AND score IS NULL)::float8
		FROM specs`, attemptID,
	).Scan(&earned, &maxSum, &pending)
	if err != nil {
		return err
	}
	return updateAttemptAggregatesTx(ctx, tx, attemptID, earned, maxSum, pending > 0)
}

// updateAttemptAggregatesTx: tulis raw/final/status/passed ke attempts.
func updateAttemptAggregatesTx(ctx context.Context, tx pgx.Tx, attemptID string, earned, maxA float64, pending bool) error {
	final := earned
	status := "graded"
	provisional := pending
	if pending {
		status = "awaiting_manual_grading"
	}
	var ps *float64
	var examID string
	if err := tx.QueryRow(ctx, `SELECT exam_id::text FROM attempts WHERE id = $1::uuid`, attemptID).Scan(&examID); err != nil {
		return err
	}
	if true {
		if !pending {
			if err := tx.QueryRow(ctx, `SELECT passing_score::float8 FROM exams WHERE id = $1::uuid`, examID).Scan(&ps); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
	}
	if !pending {
		final = round2x(safePct(earned, maxA))
		if ps != nil {
			p := final >= *ps
			if _, err := tx.Exec(ctx, `UPDATE attempts SET passed = $2 WHERE id = $1::uuid`, attemptID, &p); err != nil {
				return err
			}
		}
	}
	_, err := tx.Exec(ctx, `
		UPDATE attempts SET raw_score = $2::numeric, max_score = $3::numeric,
		                    final_score = $4::numeric, score_provisional = $5,
		                    status = $6, updated_at = now()
		WHERE id = $1::uuid`,
		attemptID, round2(earned), round2(maxA), round2(final), provisional, status)
	return err
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func execTenantTx(ctx context.Context, db *platform.DB, tenantID, q string, args ...any) (pgconn.CommandTag, error) {
	var tag pgconn.CommandTag
	err := db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		t, err := tx.Exec(ctx, q, args...)
		tag = t
		return err
	})
	return tag, err
}

// ReleaseScores: guru merilis nilai (FR-5.4) — siswa melihat hasil pada
// GET result setelah momen ini, terlepas dari show_score_immediately.
func (s *Store) ReleaseScores(ctx context.Context, tenantID, examID string) error {
	tag, err := execTenantTx(ctx, s.db, tenantID, `
		UPDATE exams SET scores_released_at = now(), updated_at = now()
		WHERE id = $1::uuid`, examID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
