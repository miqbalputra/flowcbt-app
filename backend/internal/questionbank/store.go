// Store questionbank: query lewat WithTenantTx (tenant dari JWT) — RLS tetap pengaman.
package questionbank

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/flowcbt/backend/internal/platform"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrNotFound = errors.New("tidak ditemukan")

type Store struct{ db *platform.DB }

func NewStore(db *platform.DB) *Store { return &Store{db: db} }

// requireOne: pastikan operasi tulis benar-benar mengubah satu baris.
func requireOne(tag pgconn.CommandTag, err error) error {
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// cryptoRandID: 26 hex lowercase acak (crypto/rand) — kode ujian & id serupa.
func cryptoRandID() string {
	b := make([]byte, 13)
	if _, err := rand.Read(b); err != nil {
		return strings.ReplaceAll(time.Now().Format("20060102150405.000000000"), ".", "")
	}
	const hexd = "0123456789abcdef"
	out := make([]byte, 26)
	for i, c := range b {
		out[i*2] = hexd[c>>4]
		out[i*2+1] = hexd[c&0x0f]
	}
	return string(out)
}

func (s *Store) CreateSet(ctx context.Context, tenantID, createdBy string, in SetInput) (*QuestionSet, error) {
	var set QuestionSet
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		if in.ExamCode == "" {
			in.ExamCode = GenerateExamCode()
		}
		return tx.QueryRow(ctx, `
			INSERT INTO question_sets (subject_name, title, description, exam_code, level_tags, status, shared_with_tenant, created_by)
			VALUES ($1, $2, $3, $4, $5, 'draft', $6, $7)
			RETURNING id::text, subject_name, title, description, exam_code, level_tags, status, shared_with_tenant,
			          created_by::text, created_at::text, updated_at::text`,
			in.SubjectName, in.Title, in.Description, in.ExamCode, in.LevelTags, in.SharedWithTenant, createdBy,
		).Scan(&set.ID, &set.SubjectName, &set.Title, &set.Description, &set.ExamCode,
			&set.LevelTags, &set.Status, &set.SharedWithTenant, &set.CreatedBy,
			&set.CreatedAt, &set.UpdatedAt)
	})
	if err != nil {
		return nil, err
	}
	// Versi draft pertama dibuat bersama paket.
	if err := s.createInitialDraft(ctx, tenantID, set.ID); err != nil {
		return nil, err
	}
	return &set, nil
}

type SetInput struct {
	SubjectName      string   `json:"subject_name"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	ExamCode         string   `json:"exam_code,omitempty"`
	LevelTags        []string `json:"level_tags"`
	SharedWithTenant bool     `json:"shared_with_tenant"`
}

// GenerateExamCode: unik per tenant; format KODE-XXXXXX (huruf besar tanpa I/O/0/1).
func GenerateExamCode() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	s := make([]byte, 6)
	if _, err := rand.Read(s); err != nil {
		return "KODE-" + time.Now().UTC().Format("060102150405")
	}
	for i := range s {
		s[i] = chars[int(s[i])%len(chars)]
	}
	return "KODE-" + string(s)
}

func (s *Store) ListSets(ctx context.Context, tenantID string, q string, status string, limit, offset int) ([]QuestionSet, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var out []QuestionSet
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, subject_name, title, description, exam_code, level_tags, status,
			       shared_with_tenant, created_by::text, created_at::text, updated_at::text
			FROM question_sets
			WHERE ($1 = '' OR title ILIKE '%' || $1 || '%' OR subject_name ILIKE '%' || $1 || '%')
			  AND ($2 = '' OR status = $2)
			ORDER BY updated_at DESC
			LIMIT $3 OFFSET $4`, q, status, limit, offset)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var set QuestionSet
			if err := rows.Scan(&set.ID, &set.SubjectName, &set.Title, &set.Description,
				&set.ExamCode, &set.LevelTags, &set.Status, &set.SharedWithTenant,
				&set.CreatedBy, &set.CreatedAt, &set.UpdatedAt); err != nil {
				return err
			}
			out = append(out, set)
		}
		return rows.Err()
	})
	return out, err
}

func (s *Store) GetSet(ctx context.Context, tenantID, id string) (*QuestionSet, error) {
	var set QuestionSet
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT id::text, subject_name, title, description, exam_code, level_tags, status,
			       shared_with_tenant, created_by::text, created_at::text, updated_at::text
			FROM question_sets WHERE id = $1::uuid`, id,
		).Scan(&set.ID, &set.SubjectName, &set.Title, &set.Description, &set.ExamCode,
			&set.LevelTags, &set.Status, &set.SharedWithTenant, &set.CreatedBy,
			&set.CreatedAt, &set.UpdatedAt)
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &set, nil
}

type SetUpdate struct {
	SubjectName      *string
	Title            *string
	Description      *string
	LevelTags        *[]string
	SharedWithTenant *bool
}

func (s *Store) UpdateSet(ctx context.Context, tenantID, id string, in SetUpdate) error {
	return s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE question_sets SET
				subject_name       = COALESCE($3, subject_name),
				title              = COALESCE($4, title),
				description        = COALESCE($5, description),
				level_tags         = COALESCE($6, level_tags),
				shared_with_tenant = COALESCE($7, shared_with_tenant),
				updated_at         = now()
			WHERE id = $2::uuid AND status <> 'archived'`,
			id, in.SubjectName, in.Title, in.Description, in.LevelTags, in.SharedWithTenant)
		return requireOne(tag, err)
	})
}

// ArchiveSet: soft delete; publish terkunci ikut tanpa menghapus versi.
func (s *Store) ArchiveSet(ctx context.Context, tenantID, id string) error {
	return s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE question_sets SET status = 'archived', archived_at = now() WHERE id = $1::uuid AND status <> 'archived'`, id)
		return requireOne(tag, err)
	})
}

// ---- Versi & publish (FR-4.7/4.8) ----

// ensureDraftVersionTx: pastikan ada persis satu versi draft per paket dan
// question_sets.draft_version_id selalu menunjuk ke sana, bekerja pada
// transaksi yang SUDAH berjalan (dipakai PublishSet). Mengembalikan ID draft.
func ensureDraftVersionTx(ctx context.Context, tx pgx.Tx, setID string) (string, error) {
	var draftID string
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(draft_version_id::text, '') FROM question_sets WHERE id = $1::uuid`, setID).Scan(&draftID); err != nil {
		return "", err
	}
	if draftID == "" {
		// Versi draft sudah ada tapi belum tersambung — sambungkan dulu.
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(id::text, '') FROM question_set_versions
			WHERE question_set_id = $1::uuid AND status = 'draft' LIMIT 1`, setID).Scan(&draftID); err != nil {
			return "", err
		}
	}
	if draftID != "" {
		if _, err := tx.Exec(ctx, `
			UPDATE question_sets SET draft_version_id = $2::uuid
			WHERE id = $1::uuid AND (draft_version_id IS NULL OR draft_version_id <> $2::uuid)`,
			setID, draftID); err != nil {
			return "", err
		}
		return draftID, nil
	}
	var versionNo int
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(version_no), 0) + 1 FROM question_set_versions WHERE question_set_id = $1::uuid`, setID).Scan(&versionNo); err != nil {
		return "", err
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO question_set_versions (question_set_id, version_no, status)
		VALUES ($1::uuid, $2, 'draft') RETURNING id::text`, setID, versionNo).Scan(&draftID); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE question_sets SET draft_version_id = $2::uuid WHERE id = $1::uuid`, setID, draftID); err != nil {
		return "", err
	}
	return draftID, nil
}

// PublishSet (FR-4.7): validasi kelengkapan → buat versi published (beku) →
// clone draft baru (keputusan #4, lampiran 18.6). Mengembalikan versi terbit.
func (s *Store) PublishSet(ctx context.Context, tenantID, setID, userID string) (*QuestionSetVersion, error) {
	var published *QuestionSetVersion
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		// 1. Pastikan ada versi draft sumber dengan soal di dalamnya.
		draftID, err := ensureDraftVersionTx(ctx, tx, setID)
		if err != nil {
			return err
		}

		// 2. Validasi tiap soal punya kunci & poin (FR-4.7).
		if problems := s.validateQuestions(ctx, tx, draftID); len(problems) > 0 {
			return errors.New(strings.Join(problems, "; "))
		}

		// 3. Nomor versi berikutnya.
		var versionNo int
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(MAX(version_no), 0) + 1 FROM question_set_versions WHERE question_set_id = $1::uuid`, setID).Scan(&versionNo); err != nil {
			return err
		}

		// 4. Bekukan: buat baris versi terbit (id dari DB), lalu salin stimulus & soal.
		nowStr := time.Now().UTC().Format(time.RFC3339)
		byStr := userID
		var newVersionID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO question_set_versions (question_set_id, version_no, status, published_at, published_by, question_count, max_points)
			VALUES ($1::uuid, $2, 'published', $3, $4, 0, 0)
			RETURNING id::text`, setID, versionNo, nowStr, byStr,
		).Scan(&newVersionID); err != nil {
			return err
		}
		published = &QuestionSetVersion{
			ID:            newVersionID,
			QuestionSetID: setID,
			VersionNo:     versionNo,
			Status:        "published",
		}
		published.PublishedAt = &nowStr
		published.PublishedBy = &byStr
		if err := s.cloneStimuliForVersion(ctx, tx, draftID, newVersionID); err != nil {
			return err
		}
		if err := s.cloneQuestionsForVersion(ctx, tx, draftID, newVersionID); err != nil {
			return err
		}

		// 5. Kunci set: published_version_id diisi; draft asli tetap jadi
		// salinan kerja — perubahan setelah publish tidak menyentuh versi
		// terbit (keputusan #4, lampiran 18.6).
		if _, err := tx.Exec(ctx, `
			UPDATE question_sets
			SET published_version_id = $3, status = 'published', updated_at = now()
			WHERE id = $1::uuid AND tenant_id = $2::uuid`, setID, tenantID, newVersionID); err != nil {
			return err
		}
		if _, err := ensureDraftVersionTx(ctx, tx, setID); err != nil {
			return err
		}

		// 6. Hitung ulang jumlah & total poin versi terbit.
		return s.recountVersion(ctx, tx, published)
	})
	if err != nil {
		return nil, err
	}
	return published, nil
}

func (s *Store) validateQuestions(ctx context.Context, tx pgx.Tx, versionID string) []string {
	rows, err := tx.Query(ctx, `
		SELECT type, body_html IS NULL OR btrim(body_html) = '', points IS NULL, options_json
		FROM questions WHERE set_version_id = $1::uuid`, versionID)
	if err != nil {
		return []string{"gagal membaca soal: " + err.Error()}
	}
	defer rows.Close()
	problems := []string{}
	i := 0
	for rows.Next() {
		i++
		var typ string
		var bodyEmpty, pointsNull bool
		var optionsJSON []byte
		if err := rows.Scan(&typ, &bodyEmpty, &pointsNull, &optionsJSON); err != nil {
			problems = append(problems, "soal ke-"+strconv.Itoa(i)+": tidak terbaca")
			continue
		}
		switch {
		case (typ == TypeMCQ || typ == TypeMCQComplex) && !hasCorrectOption(optionsJSON):
			problems = append(problems, "soal ke-"+strconv.Itoa(i)+": kunci jawaban pilihan belum diatur")
		case bodyEmpty:
			problems = append(problems, "soal ke-"+strconv.Itoa(i)+": pertanyaan kosong")
		case pointsNull:
			problems = append(problems, "soal ke-"+strconv.Itoa(i)+": poin belum diatur")
		}
	}
	if i == 0 {
		problems = append(problems, "paket belum memiliki soal")
	}
	return problems
}

// hasCorrectOption: kunci MCQ sah bila minimal satu opsi bertanda benar.
func hasCorrectOption(optionsJSON []byte) bool {
	var opts []struct {
		Correct bool `json:"correct"`
	}
	if json.Unmarshal(optionsJSON, &opts) != nil {
		return false
	}
	for _, o := range opts {
		if o.Correct {
			return true
		}
	}
	return false
}

// cloneQuestionsForVersion: salin seluruh soal dari satu versi ke versi lain.
func (s *Store) cloneQuestionsForVersion(ctx context.Context, tx pgx.Tx, fromVersionID, toVersionID string) error {
	tag, err := tx.Exec(ctx, `
		INSERT INTO questions (set_version_id, order_no, type, stimulus_id, body_html, options_json,
		                       answer_key_json, rubric_json, points, scoring_mode, partial_wrong_penalty,
		                       shuffle_options, explanation_html, media_ids, created_at, updated_at)
		SELECT $2::uuid, order_no, type, stimulus_id, body_html, options_json,
		       answer_key_json, rubric_json, points, scoring_mode, partial_wrong_penalty,
		       shuffle_options, explanation_html, media_ids, now(), now()
		FROM questions WHERE set_version_id = $1::uuid
		ORDER BY order_no`, fromVersionID, toVersionID)
	if err != nil {
		return err
	}
	_ = tag
	return nil
}

// cloneStimuliForVersion: salin stimulus antar versi.
func (s *Store) cloneStimuliForVersion(ctx context.Context, tx pgx.Tx, fromVersionID, toVersionID string) error {
	tag, err := tx.Exec(ctx, `
		INSERT INTO stimuli (set_version_id, order_no, title, content_html, media_ids)
		SELECT $2::uuid, order_no, title, content_html, media_ids
		FROM stimuli WHERE set_version_id = $1::uuid
		ORDER BY order_no`, fromVersionID, toVersionID)
	if err != nil {
		return err
	}
	_ = tag
	return nil
}

// recountVersion: hitung ulang question_count & max_points (poin maksimum).
func (s *Store) recountVersion(ctx context.Context, tx pgx.Tx, v *QuestionSetVersion) error {
	return tx.QueryRow(ctx, `
		SELECT count(*)::int, COALESCE(SUM(points), 0)::float8
		FROM questions WHERE set_version_id = $1::uuid`, v.ID,
	).Scan(&v.QuestionCount, &v.MaxPoints)
}

// createInitialDraft: draft pertama saat paket dibuat (tx baru).
func (s *Store) createInitialDraft(ctx context.Context, tenantID, setID string) error {
	return s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		_, err := ensureDraftVersionTx(ctx, tx, setID)
		return err
	})
}
