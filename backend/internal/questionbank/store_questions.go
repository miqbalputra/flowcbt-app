// Soal: baca/tulis/hapus/urut & duplikasi paket — melengkapi store.go
// (publish & versi tetap di store.go). Semua query tunduk WithTenantTx.
package questionbank

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

// WithTenantTxForMedia: unggahan media butuh tx tenant di platform.DB
// (kuota 3.2 diperiksa handler dalam tx yang sama dengan INSERT aset).
func (s *Store) WithTenantTxForMedia(ctx context.Context, tenantID string, fn func(tx pgx.Tx) error) error {
	return s.db.WithTenantTx(ctx, tenantID, fn)
}

// CompensateMedia: simpan file fisik gagal setelah commit DB —
// batalkan baris aset & kembalikan kuota (panggilan best-effort handler).
func (s *Store) CompensateMedia(ctx context.Context, tenantID, assetID string, sizeBytes int64) error {
	return s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`DELETE FROM media_assets WHERE id = $1::uuid`, assetID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			`UPDATE tenants SET storage_used_bytes = GREATEST(0, storage_used_bytes - $2)
			 WHERE id = $1::uuid`, tenantID, sizeBytes)
		return err
	})
}

// draftVersionIDOf: ID versi draft paket; kosong berarti tidak ada.
// (after fallback bila draft_version_id belum tersambung: ambil versi terbaru.)
func draftVersionIDOf(ctx context.Context, tx pgx.Tx, setID string) (string, error) {
	var versionID string
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(
			qs.draft_version_id::text,
			(SELECT v.id::text FROM question_set_versions v
			 WHERE v.question_set_id = qs.id ORDER BY v.version_no DESC LIMIT 1),
			'')
		FROM question_sets qs WHERE qs.id = $1::uuid`, setID).Scan(&versionID)
	return versionID, err
}

// QuestionsOfSet: daftar soal versi draft (editor guru).
func (s *Store) QuestionsOfSet(ctx context.Context, tenantID, setID string) ([]Question, error) {
	var versionID string
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		var err error
		versionID, err = draftVersionIDOf(ctx, tx, setID)
		return err
	})
	if err != nil {
		return nil, err
	}
	if versionID == "" {
		return nil, ErrNotFound
	}
	return s.QuestionsOfVersion(ctx, tenantID, versionID)
}

// QuestionsOfVersion: soal satu versi (apa pun statusnya) urut order_no.
func (s *Store) QuestionsOfVersion(ctx context.Context, tenantID, versionID string) ([]Question, error) {
	out := []Question{}
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, order_no, type, stimulus_id::text, body_html,
			       options_json, answer_key_json, rubric_json, points::float8,
			       scoring_mode, partial_wrong_penalty, shuffle_options,
			       COALESCE(explanation_html, ''), array_to_json(media_ids)::text,
			       created_at::text, updated_at::text
			FROM questions WHERE set_version_id = $1::uuid
			ORDER BY order_no`, versionID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var q Question
			var optionsJSON, answerJSON, rubricJSON, mediaJSON []byte
			if err := rows.Scan(&q.ID, &q.OrderNo, &q.Type, &q.StimulusID, &q.Body,
				&optionsJSON, &answerJSON, &rubricJSON, &q.Points,
				&q.ScoringMode, &q.PartialWrongPenalty, &q.ShuffleOptions,
				&q.ExplanationHTML, &mediaJSON, &q.CreatedAt, &q.UpdatedAt); err != nil {
				return err
			}
			if len(optionsJSON) > 0 {
				_ = json.Unmarshal(optionsJSON, &q.Options)
			}
			if len(answerJSON) > 0 && string(answerJSON) != "null" {
				_ = json.Unmarshal(answerJSON, &q.AnswerKey)
			}
			if len(rubricJSON) > 0 && string(rubricJSON) != "null" {
				_ = json.Unmarshal(rubricJSON, &q.Rubric)
			}
			if len(mediaJSON) > 0 {
				_ = json.Unmarshal(mediaJSON, &q.MediaIDs)
			}
			out = append(out, q)
		}
		return rows.Err()
	})
	return out, err
}

// AddQuestion: soal baru di ujung versi draft.
func (s *Store) AddQuestion(ctx context.Context, tenantID, setID string, q Question) (*Question, error) {
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		draftID, err := ensureDraftVersionTx(ctx, tx, setID)
		if err != nil {
			return err
		}
		_, err = insertQuestionTx(ctx, tx, draftID, q)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &q, nil
}

// insertQuestionTx: INSERT soal pada transaksi berjalan; mengisi ID & timestamp.
func insertQuestionTx(ctx context.Context, tx pgx.Tx, draftID string, q Question) (*Question, error) {
	var next int
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(order_no), 0) + 1 FROM questions WHERE set_version_id = $1::uuid`,
		draftID).Scan(&next); err != nil {
		return nil, err
	}
	if q.ScoringMode == "" {
		q.ScoringMode = "all_or_nothing"
	}
	optionsJSON, err := json.Marshal(q.Options)
	if err != nil {
		return nil, err
	}
	var answerArg, rubricArg any
	if q.AnswerKey != nil {
		answerArg, err = json.Marshal(q.AnswerKey)
		if err != nil {
			return nil, err
		}
	}
	if q.Rubric != nil {
		rubricArg, err = json.Marshal(q.Rubric)
		if err != nil {
			return nil, err
		}
	}
	var stimulusArg any
	if q.StimulusID != nil && *q.StimulusID != "" {
		stimulusArg = *q.StimulusID
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO questions (set_version_id, order_no, type, stimulus_id, body_html,
		                       options_json, answer_key_json, rubric_json, points,
		                       scoring_mode, partial_wrong_penalty, shuffle_options,
		                       explanation_html, media_ids)
		VALUES ($1::uuid, $2, $3, NULLIF($4::text,'')::uuid, $5,
		        $6::jsonb, $7::jsonb, $8::jsonb, $9,
		        $10, $11, $12, NULLIF($13,''), '{}'::uuid[])
		RETURNING id::text, created_at::text, updated_at::text`,
		draftID, next, q.Type, stimulusArg, q.Body,
		string(optionsJSON), answerArg, rubricArg, q.Points,
		q.ScoringMode, q.PartialWrongPenalty, q.ShuffleOptions,
		q.ExplanationHTML,
	).Scan(&q.ID, &q.CreatedAt, &q.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &q, nil
}

// UpdateQuestion: pembaruan penuh dari editor; hanya versi draft (FR-4.8 —
// soal versi terbit dibekukan). Kolom kosong/nol berarti "tidak berubah"
// agar autosave parsial tidak menghapus data.
func (s *Store) UpdateQuestion(ctx context.Context, tenantID, qid string,
	stimulusID *string, body string, options []Option, answerKey any, rubric any,
	points float64, scoringMode string, partialWrongPenalty, shuffleOptions *bool,
	explanationHTML string) error {

	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return err
	}
	if string(optionsJSON) == "null" {
		optionsJSON = []byte("[]")
	}
	var answerArg, rubricArg any
	if answerKey != nil {
		answerArg, err = json.Marshal(answerKey)
		if err != nil {
			return err
		}
	}
	if rubric != nil {
		rubricArg, err = json.Marshal(rubric)
		if err != nil {
			return err
		}
	}
	var stimulusArg any
	if stimulusID != nil && *stimulusID != "" {
		stimulusArg = *stimulusID
	}
	partial := true
	if partialWrongPenalty != nil {
		partial = *partialWrongPenalty
	}
	shuffle := true
	if shuffleOptions != nil {
		shuffle = *shuffleOptions
	}
	return s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE questions SET
				stimulus_id           = COALESCE($2::uuid, stimulus_id),
				body_html             = CASE WHEN $3 <> '' THEN $3 ELSE body_html END,
				options_json          = CASE WHEN $4::text <> 'null' THEN $4::jsonb ELSE options_json END,
				answer_key_json       = COALESCE($5::jsonb, answer_key_json),
				rubric_json           = COALESCE($6::jsonb, rubric_json),
				points                = CASE WHEN $7 > 0 THEN $7 ELSE points END,
				scoring_mode          = CASE WHEN $8 IN ('all_or_nothing','partial') THEN $8 ELSE scoring_mode END,
				partial_wrong_penalty = $9,
				shuffle_options       = $10,
				explanation_html      = CASE WHEN $11 <> '' THEN $11 ELSE explanation_html END,
				updated_at            = now()
			WHERE id = $1::uuid
			  AND set_version_id IN (SELECT id FROM question_set_versions WHERE status = 'draft')`,
			qid, stimulusArg, body, string(optionsJSON), answerArg, rubricArg,
			points, scoringMode, partial, shuffle, explanationHTML)
		return requireOne(tag, err)
	})
}

// DeleteQuestion: hanya pada versi draft.
func (s *Store) DeleteQuestion(ctx context.Context, tenantID, qid string) error {
	return s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			DELETE FROM questions WHERE id = $1::uuid
			  AND set_version_id IN (SELECT id FROM question_set_versions WHERE status = 'draft')`, qid)
		return requireOne(tag, err)
	})
}

// Reorder: simpan urutan baru pada versi draft (order_no mengikuti indeks).
func (s *Store) Reorder(ctx context.Context, tenantID, setID string, order []string) error {
	return s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		draftID, err := ensureDraftVersionTx(ctx, tx, setID)
		if err != nil {
			return err
		}
		for i, questionID := range order {
			if _, err := tx.Exec(ctx,
				`UPDATE questions SET order_no = $3
				 WHERE id = $2::uuid AND set_version_id = $1::uuid`,
				draftID, questionID, i+1); err != nil {
				return err
			}
		}
		return nil
	})
}

// DuplicateSet: salin paket beserta soal versi sumber (draft bila ada,
// selain itu versi terbit) ke paket draft baru milik pengerjakan saat ini.
func (s *Store) DuplicateSet(ctx context.Context, tenantID, setID, userID string) (string, error) {
	var newID string
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		var subjectName, title, description, srcDraft, srcPublished string
		var levelTags []string
		var shared bool
		if err := tx.QueryRow(ctx, `
			SELECT subject_name, title, COALESCE(description, ''), level_tags, shared_with_tenant,
			       COALESCE(draft_version_id::text, ''), COALESCE(published_version_id::text, '')
			FROM question_sets WHERE id = $1::uuid`, setID,
		).Scan(&subjectName, &title, &description, &levelTags, &shared, &srcDraft, &srcPublished); err != nil {
			return err
		}
		code := GenerateExamCode()
		if err := tx.QueryRow(ctx, `
			INSERT INTO question_sets (subject_name, title, description, exam_code, level_tags,
			                           status, shared_with_tenant, created_by)
			VALUES ($1, $2, $3, $4, $5, 'draft', $6, $7)
			RETURNING id::text`,
			subjectName, title+" (salinan)", description, code, levelTags, shared, userID,
		).Scan(&newID); err != nil {
			return err
		}
		newDraft, err := ensureDraftVersionTx(ctx, tx, newID)
		if err != nil {
			return err
		}
		src := srcDraft
		if src == "" {
			src = srcPublished
		}
		if src == "" {
			return nil // paket sumber masih kosong — salinan kosong pun sah
		}
		if err := s.cloneStimuliForVersion(ctx, tx, src, newDraft); err != nil {
			return err
		}
		return s.cloneQuestionsForVersion(ctx, tx, src, newDraft)
	})
	return newID, err
}

// UnpublishSet (S8): kembalikan status terbit → draft. Ditolak bila ujian
// masih memakai versi terbit (integritas attempt, FR-5.x). Versi terbit
// tetap disimpan sebagai riwayat.
func (s *Store) UnpublishSet(ctx context.Context, tenantID, setID string) error {
	return s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE question_sets SET status = 'draft', updated_at = now()
			WHERE id = $1::uuid AND status = 'published' AND published_version_id IS NOT NULL
			  AND NOT EXISTS (SELECT 1 FROM exams e
			                  WHERE e.set_version_id = question_sets.published_version_id)`, setID)
		return requireOne(tag, err)
	})
}

// AnswerKeyOf: kunci jawaban soal (untuk audit perubahan kunci, S8).
func (s *Store) AnswerKeyOf(ctx context.Context, tenantID, qid string) (json.RawMessage, bool, error) {
	var raw []byte
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT answer_key_json FROM questions WHERE id = $1::uuid`, qid,
		).Scan(&raw)
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, ErrNotFound
		}
		return nil, false, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, false, nil
	}
	return json.RawMessage(raw), true, nil
}
