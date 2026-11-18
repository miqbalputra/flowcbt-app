// Submit & hasil (S13): kirim jawaban → grading otomatis → state machine
// submitted/awaiting_manual_grading/graded → GET result. Auto-submit waktu
// habis oleh worker (klaim atomic). Penalti pelanggaran (FR-5.8/8.2)
// menyusul di fase proctoring.
package exam

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/flowcbt/backend/internal/grading"
	"github.com/flowcbt/backend/internal/httpx"
	"github.com/jackc/pgx/v5"
)

type submitReq struct {
	Reason string `json:"submit_reason"` // manual | force (guru memakai force-submit S10)
}

// SubmitAttempt: siswa mengirim jawaban (kunci otoritas: server).
func (s *Store) SubmitAttempt(ctx context.Context, tenantID, studentID, attemptID, reason string) (*ResultView, error) {
	if reason == "" {
		reason = "manual"
	}
	// 1. Klaim atomik: hanya satu pemanggil yang berhasil mengubah status.
	claimed, err := s.claimAndClose(ctx, tenantID, studentID, attemptID, reason, true)
	if err != nil {
		return nil, err
	}
	// 2. Grading atas attempt yang sudah tertutup.
	return s.gradeClosedAttempt(ctx, tenantID, claimed)
}

// claimAndClose: status in_progress → submitted (atau timeout); idempoten 409.
// Mengembalikan data attempt yang dibutuhkan grading.
type closedAttempt struct {
	AttemptID    string
	ExamID       string
	ExamName     string
	SetVersionID string
	Reason       string
}

func (s *Store) claimAndClose(ctx context.Context, tenantID, ownerStudentID, attemptID, reason string, checkOwner bool) (*closedAttempt, error) {
	var cl closedAttempt
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			UPDATE attempts a SET status = 'submitted', submit_reason = $3,
			                      submitted_at = now(), updated_at = now()
			FROM exams e
			WHERE a.id = $1::uuid AND a.tenant_id = $2::uuid AND a.exam_id = e.id
			  AND a.status = 'in_progress'
			  AND ($5 = false OR a.student_id = $4::uuid)
			RETURNING a.id::text, a.exam_id::text, e.name, COALESCE(a.set_version_id::text, ''),
			          a.submit_reason`,
			attemptID, tenantID, reason, ownerStudentID, checkOwner,
		).Scan(&cl.AttemptID, &cl.ExamID, &cl.ExamName, &cl.SetVersionID, &cl.Reason)
		if errors.Is(err, pgx.ErrNoRows) {
			// Bukan berjalan: beda sebab → 404 vs 403 vs 409.
			return classifyNotClaimable(ctx, tx, tenantID, ownerStudentID, attemptID, checkOwner)
		}
		if err != nil {
			return err
		}
		// Tutup sesi aktif.
		_, err = tx.Exec(ctx, `
			UPDATE attempt_sessions SET revoked_at = now(), revoked_by = 'submit'
			WHERE attempt_id = $1::uuid AND revoked_at IS NULL`, attemptID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &cl, nil
}

func classifyNotClaimable(ctx context.Context, tx pgx.Tx, tenantID, ownerStudentID, attemptID string, checkOwner bool) error {
	var owner, status string
	err := tx.QueryRow(ctx, `
		SELECT student_id::text, status FROM attempts
		WHERE id = $1::uuid AND tenant_id = $2::uuid`, attemptID, tenantID,
	).Scan(&owner, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if checkOwner && owner != ownerStudentID {
		return httpx.AppErr(403, "FORBIDDEN", "bukan percobaan Anda", nil)
	}
	return httpx.AppErr(409, "ATTEMPT_CLOSED", "percobaan sudah berakhir", nil)
}

// gradeClosedAttempt: nilai seluruh soal tertutup & hitung Skor Akhir (FR-8.2).
func (s *Store) gradeClosedAttempt(ctx context.Context, tenantID string, cl *closedAttempt) (*ResultView, error) {
	var view *ResultView
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		specs, err := loadQuestionSpecsTx(ctx, tx, cl.SetVersionID)
		if err != nil {
			return err
		}
		saved, err := loadSavedAnswersTx(ctx, tx, cl.AttemptID)
		if err != nil {
			return err
		}

		maxScore := 0.0
		earned := 0.0
		hasManual := false
		for _, q := range specs {
			maxScore += q.Points
			ans := saved[q.ID]
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
			if res.NeedsManual {
				hasManual = true
				if _, err := tx.Exec(ctx, `
					INSERT INTO attempt_answers (tenant_id, attempt_id, question_id, answer_json,
					                             flagged, graded_by, seq)
					VALUES ($1::uuid, $2::uuid, $3::uuid, $4::jsonb, $5, 'auto', $6)
					ON CONFLICT (attempt_id, question_id) DO UPDATE
					SET graded_by = 'auto', auto_score = NULL, score = NULL, is_correct = NULL,
					    updated_at = now()`,
					tenantID, cl.AttemptID, q.ID, ans.Answer, ans.Flagged, ans.Seq); err != nil {
					return err
				}
				continue
			}
			earned += res.Score
			var isCorrect any
			if res.IsCorrect != nil {
				isCorrect = *res.IsCorrect
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO attempt_answers (tenant_id, attempt_id, question_id, answer_json,
				                             flagged, is_correct, auto_score, score, graded_by, seq)
				VALUES ($1::uuid, $2::uuid, $3::uuid, $4::jsonb, $5, $6, $7, $7, 'auto', $8)
				ON CONFLICT (attempt_id, question_id) DO UPDATE
				SET is_correct = $6, auto_score = $7, score = $7, graded_by = 'auto',
				    updated_at = now()`,
				tenantID, cl.AttemptID, q.ID, ans.Answer, ans.Flagged, isCorrect, res.Score, ans.Seq); err != nil {
				return err
			}
		}

		provisional := hasManual
		newStatus := "graded"
		earned = round2x(earned)
		maxScore = round2x(maxScore)
		var final any // NULL saat masih menunggu uraian? — tetap diisi earned (Sementara)
		if hasManual {
			newStatus = "awaiting_manual_grading"
			final = earned
		} else {
			final = round2x(safePct(earned, maxScore))
		}
		if _, err := tx.Exec(ctx, `
			UPDATE attempts SET status = $2, raw_score = $3, max_score = $4,
			                    final_score = $5, score_provisional = $6, updated_at = now()
			WHERE id = $1::uuid`,
			cl.AttemptID, newStatus, earned, maxScore, final, provisional,
		); err != nil {
			return err
		}
		var passed *bool
		if !hasManual {
			var ps *float64
			if err := tx.QueryRow(ctx,
				`SELECT passing_score::float8 FROM exams WHERE id = $1::uuid`, cl.ExamID).Scan(&ps); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if ps != nil {
				p := final.(float64) >= *ps
				passed = &p
				if _, err := tx.Exec(ctx, `UPDATE attempts SET passed = $2 WHERE id = $1::uuid`,
					cl.AttemptID, passed); err != nil {
					return err
				}
			}
		}
		view = &ResultView{
			AttemptID: cl.AttemptID, ExamID: cl.ExamID, Status: newStatus,
			Earned: earned, MaxScore: maxScore,
			FinalScore: final.(float64), Provisional: provisional,
			SubmittedReason: cl.Reason, Passed: passed,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return view, nil
}

func safePct(earned, max float64) float64 {
	if max <= 0 {
		return 0
	}
	return earned / max * 100
}

func round2x(v float64) float64 { return math.Round(v*100) / 100 }

// ---- kunci & spesifikasi soal untuk grading ----

func loadQuestionSpecsTx(ctx context.Context, tx pgx.Tx, versionID string) ([]grading.QuestionSpec, error) {
	rows, err := tx.Query(ctx, `
		SELECT id::text, type, points::float8, scoring_mode, partial_wrong_penalty,
		       options_json, answer_key_json
		FROM questions WHERE set_version_id = $1::uuid ORDER BY order_no`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []grading.QuestionSpec{}
	for rows.Next() {
		var q grading.QuestionSpec
		var optionsJSON, answerJSON []byte
		var pen bool
		if err := rows.Scan(&q.ID, &q.Type, &q.Points, &q.ScoringMode, &pen,
			&optionsJSON, &answerJSON); err != nil {
			return nil, err
		}
		q.PartialWrongPenalty = pen
		switch q.Type {
		case grading.TypeMatching:
			var key struct {
				Pairs []struct {
					Left  string `json:"left"`
					Right string `json:"right"`
				} `json:"pairs"`
			}
			if len(answerJSON) > 0 && json.Unmarshal(answerJSON, &key) == nil {
				for _, p := range key.Pairs {
					q.MatchPairs = append(q.MatchPairs, [2]string{p.Left, p.Right})
				}
			}
		case grading.TypeMCQ, grading.TypeMCQComplex:
			var opts []struct {
				ID      string `json:"id"`
				Correct bool   `json:"correct"`
			}
			if len(optionsJSON) > 0 && json.Unmarshal(optionsJSON, &opts) == nil {
				for _, o := range opts {
					q.Options = append(q.Options, grading.OptionSpec{ID: o.ID, Correct: o.Correct})
				}
			}
		case grading.TypeShortAnswer:
			var key struct {
				Accepted []string `json:"accepted"`
			}
			if len(answerJSON) > 0 && json.Unmarshal(answerJSON, &key) == nil {
				q.Accepted = key.Accepted
			}
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

type savedAnswerRow struct {
	Answer  json.RawMessage
	Flagged bool
	Seq     int64
}

func loadSavedAnswersTx(ctx context.Context, tx pgx.Tx, attemptID string) (map[string]savedAnswerRow, error) {
	out := map[string]savedAnswerRow{}
	rows, err := tx.Query(ctx, `
		SELECT question_id::text, COALESCE(answer_json, 'null'::jsonb), flagged, seq
		FROM attempt_answers WHERE attempt_id = $1::uuid`, attemptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var qid string
		var a savedAnswerRow
		if err := rows.Scan(&qid, &a.Answer, &a.Flagged, &a.Seq); err != nil {
			return nil, err
		}
		if string(a.Answer) == "null" {
			a.Answer = nil
		}
		out[qid] = a
	}
	return out, rows.Err()
}

// ---- hasil untuk siswa ----

type ResultView struct {
	AttemptID       string  `json:"attempt_id"`
	ExamID          string  `json:"exam_id"`
	Status          string  `json:"status"`
	Earned          float64 `json:"earned"`
	MaxScore        float64 `json:"max_score"`
	FinalScore      float64 `json:"final_score"`
	Passed          *bool   `json:"passed,omitempty"`
	Provisional     bool    `json:"provisional"`
	SubmittedReason string  `json:"submit_reason,omitempty"`
	ServerTime      string  `json:"server_time,omitempty"`
}

type ResultQuestion struct {
	QuestionID string   `json:"question_id"`
	IsCorrect  *bool    `json:"is_correct,omitempty"`
	Score      *float64 `json:"score,omitempty"`
	NeedsGrade bool     `json:"needs_grade"`
}

type StudentResult struct {
	ResultView
	ShowReview bool             `json:"show_review"`
	Questions  []ResultQuestion `json:"questions,omitempty"`
}

// GetResult: hasil attempt untuk siswa; nilai hanya bila pengaturan ujian
// mengizinkan (FR-5.4) ATAU guru sudah merilis (scores_released_at).
func (s *Store) GetResult(ctx context.Context, tenantID, studentID, attemptID string) (*StudentResult, error) {
	var res StudentResult
	var provisional, showNow bool
	var released *time.Time
	var owner string
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT a.id::text, a.exam_id::text, a.status, COALESCE(a.raw_score::float8, 0),
			       COALESCE(a.max_score::float8, 0), COALESCE(a.final_score::float8, 0),
			       a.passed, a.score_provisional,
			       e.show_score_immediately, e.scores_released_at, e.show_review,
			       a.student_id::text
			FROM attempts a JOIN exams e ON e.id = a.exam_id
			WHERE a.id = $1::uuid AND a.tenant_id = $2::uuid`, attemptID, tenantID,
		).Scan(&res.AttemptID, &res.ExamID, &res.Status, &res.Earned, &res.MaxScore,
			&res.FinalScore, &res.Passed, &provisional, &showNow, &released, &res.ShowReview, &owner)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if owner != studentID {
			return httpx.AppErr(403, "FORBIDDEN", "bukan percobaan Anda", nil)
		}
		res.Provisional = provisional
		if res.Status == "in_progress" {
			return httpx.AppErr(409, "NOT_SUBMITTED", "percobaan belum dikirim", nil)
		}
		if !showNow && released == nil {
			return httpx.AppErr(403, "SCORE_HIDDEN", "nilai akan muncul setelah dirilis guru", nil)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	res.ServerTime = time.Now().UTC().Format(time.RFC3339)
	if !res.ShowReview {
		return &res, nil
	}
	err = s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT question_id::text, is_correct, score::float8,
			       (graded_by = 'auto' AND is_correct IS NULL)
			FROM attempt_answers WHERE attempt_id = $1::uuid`, attemptID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var rq ResultQuestion
			if err := rows.Scan(&rq.QuestionID, &rq.IsCorrect, &rq.Score, &rq.NeedsGrade); err != nil {
				return err
			}
			res.Questions = append(res.Questions, rq)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// ---- auto-submit (worker pemindai deadline) ----

// AutoSubmitExpired: klaim dengan satu UPDATE atomik (RETURNING) — aman
// dikonkurensi worker tanpa dead-lock. Mengembalikan yang berhasil dinilai.
func (s *Store) AutoSubmitExpired(ctx context.Context, limit int) (int, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	type claimed struct {
		ID, TenantID string
	}
	rows, err := s.db.System.Query(ctx, `
		UPDATE attempts SET status = 'submitted', submit_reason = 'timeout',
		                    submitted_at = now(), updated_at = now()
		WHERE id IN (
			SELECT id FROM attempts
			WHERE status = 'in_progress' AND deadline_at < now()
			ORDER BY deadline_at LIMIT $1
		)
		RETURNING id::text, tenant_id::text`, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	claimedList := []claimed{}
	for rows.Next() {
		var c claimed
		if err := rows.Scan(&c.ID, &c.TenantID); err != nil {
			return 0, err
		}
		claimedList = append(claimedList, c)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	count := 0
	for _, c := range claimedList {
		if _, err := s.gradeClosedAttemptFromSystem(ctx, c.TenantID, c.ID); err == nil {
			count++
		}
	}
	return count, nil
}

// gradeClosedAttemptFromSystem: grading attempt yang sudah 'submitted' tanpa
// info exam (worker) — ambil exam_id & seed data minimal dari attempts.
func (s *Store) gradeClosedAttemptFromSystem(ctx context.Context, tenantID, attemptID string) (*ResultView, error) {
	var cl closedAttempt
	var setVersionID, examID, reason string
	err := s.db.System.QueryRow(ctx, `
		SELECT set_version_id::text, exam_id::text, COALESCE(submit_reason, 'timeout')
		FROM attempts WHERE id = $1::uuid AND tenant_id = $2::uuid`, attemptID, tenantID,
	).Scan(&setVersionID, &examID, &reason)
	if err != nil {
		return nil, err
	}
	cl = closedAttempt{AttemptID: attemptID, ExamID: examID, SetVersionID: setVersionID, Reason: reason}
	return s.gradeClosedAttempt(ctx, tenantID, &cl)
}
