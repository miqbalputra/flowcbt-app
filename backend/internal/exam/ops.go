// Siklus status ujian & aksi pengawas (S10, FR-5.7):
// publish, jeda/lanjut, tutup, batalkan; tambah waktu, reset percobaan,
// force submit, keluarkan sesi. Semua tervalidasi tenant & tercatat audit
// (audit dilakukan handler).
package exam

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// Publish (S10): draft → published. Syarat: jadwal lengkap, paket soal
// sudah terbit (set_version_id mengunci versi terbit paket).
func (s *Store) Publish(ctx context.Context, tenantID, id, userID string) (*Exam, error) {
	var e *Exam
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		cur, err := s.Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		if cur.Status != "draft" {
			return errors.New("hanya ujian draft yang dapat dipublikasikan")
		}
		if cur.OpenAt == nil || cur.CloseAt == nil || cur.DurationMin == nil {
			return errors.New("jadwal belum lengkap: isi waktu buka, tutup, dan durasi (FR-5.1–5.2)")
		}
		var versionID string
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(published_version_id::text, '') FROM question_sets WHERE id = $1::uuid`,
			cur.QuestionSetID).Scan(&versionID); err != nil {
			return errors.New("paket soal tidak ditemukan")
		}
		if versionID == "" {
			return errors.New("paket soal belum diterbitkan — terbitkan dulu di Bank Soal")
		}
		e, err = scanExam(tx.QueryRow(ctx, `
			UPDATE exams SET status = 'published', set_version_id = $3::uuid,
			                 published_at = now(), updated_at = now()
			WHERE id = $1::uuid AND tenant_id = $2::uuid AND status = 'draft'
			RETURNING `+examCols, id, tenantID, versionID))
		_ = userID // audit karakter oleh pemanggil
		return err
	})
	if err != nil {
		return nil, err
	}
	return e, nil
}

// SetStatus: transisi operasional — jeda/lanjut/tutup/batalkan (FR-5.7).
func (s *Store) SetStatus(ctx context.Context, tenantID, id, action string) (*Exam, error) {
	var from, to string
	switch action {
	case "pause":
		from, to = "published", "paused"
	case "resume":
		from, to = "paused", "published"
	case "close":
		from, to = "published", "closed" // juga paused — ditangani query OR
	case "cancel":
		from, to = "draft", "cancelled" // published/paused — ditangani query OR
	default:
		return nil, errors.New("aksi tidak dikenal")
	}
	statusCond := "status = $3"
	if action == "close" {
		statusCond = "status IN ('published','paused')"
	}
	if action == "cancel" {
		statusCond = "status IN ('draft','published','paused')"
	}
	var e *Exam
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		var err error
		e, err = scanExam(tx.QueryRow(ctx, `
			UPDATE exams SET status = $4, updated_at = now()
			WHERE id = $1::uuid AND tenant_id = $2::uuid AND `+statusCond+`
			RETURNING `+examCols, id, tenantID, from, to))
		return err
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return e, nil
}

// AddTime (FR-5.7): tambah waktu satu siswa — menaikkan extra_time_seconds
// pada peserta dan memajukan deadline attempt yang sedang berjalan.
func (s *Store) AddTime(ctx context.Context, tenantID, examID, studentID string, seconds int) error {
	if seconds <= 0 || seconds > 4*3600 {
		return errors.New("tambahan waktu harus 1 detik s.d. 4 jam")
	}
	return s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE exam_participants SET extra_time_seconds = extra_time_seconds + $3
			WHERE exam_id = $1::uuid AND student_id = $2::uuid`, examID, studentID, seconds)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		_, err = tx.Exec(ctx, `
			UPDATE attempts SET extra_time_seconds = extra_time_seconds + $3,
			                    deadline_at = deadline_at + make_interval(secs => $3),
			                    updated_at = now()
			WHERE exam_id = $1::uuid AND student_id = $2::uuid AND status = 'in_progress'`,
			examID, studentID, seconds)
		return err
	})
}

// ResetAttempts (FR-5.7): hapus seluruh percobaan siswa pada ujian ini —
// attempt_sessions & jawaban ikut terhapus (ON DELETE CASCADE); siswa boleh
// mulai dari awal sesuai batas percobaan.
func (s *Store) ResetAttempts(ctx context.Context, tenantID, examID, studentID string) (int64, error) {
	var n int64
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			DELETE FROM attempts
			WHERE exam_id = $1::uuid AND student_id = $2::uuid`, examID, studentID)
		if err != nil {
			return err
		}
		n = tag.RowsAffected()
		if n == 0 {
			return ErrNotFound
		}
		return nil
	})
	return n, err
}

// ForceSubmit (FR-5.7): paksa selesai attempt berjalan; sesi aktif ditutup.
func (s *Store) ForceSubmit(ctx context.Context, tenantID, examID, studentID string) error {
	return s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE attempts SET status = 'submitted', submit_reason = 'force',
			                   submitted_at = now(), updated_at = now()
			WHERE exam_id = $1::uuid AND student_id = $2::uuid AND status = 'in_progress'`,
			examID, studentID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errors.New("tidak ada percobaan yang sedang berjalan")
		}
		_, err = tx.Exec(ctx, `
			UPDATE attempt_sessions s SET revoked_at = now(), revoked_by = 'teacher'
			FROM attempts a
			WHERE s.attempt_id = a.id AND a.exam_id = $1::uuid AND a.student_id = $2::uuid
			  AND s.revoked_at IS NULL`, examID, studentID)
		return err
	})
}

// KickSession (FR-5.7 "keluarkan sesi aktif"): cabut semua sesi aktif siswa
// agar harus login lagi (single_session_policy menentukan sesi lama/baru).
func (s *Store) KickSession(ctx context.Context, tenantID, examID, studentID string) error {
	return s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE attempt_sessions s SET revoked_at = now(), revoked_by = 'teacher'
			FROM attempts a
			WHERE s.attempt_id = a.id AND a.exam_id = $1::uuid AND a.student_id = $2::uuid
			  AND s.revoked_at IS NULL`, examID, studentID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errors.New("tidak ada sesi aktif untuk siswa ini")
		}
		return nil
	})
}

// RegenerateTokens (S10): matikan semua token aktif (atau satu ruang) lalu
// terbitkan token pengganti. Mengembalikan token baru.
func (s *Store) RegenerateTokens(ctx context.Context, tenantID, examID string, roomID *string) (*Token, error) {
	var t *Token
	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE exam_tokens t SET is_active = false
			FROM exams e
			WHERE t.exam_id = e.id AND e.tenant_id = $2::uuid AND t.exam_id = $1::uuid
			  AND t.is_active
			  AND ($3::uuid IS NULL OR t.room_id = $3::uuid)`, examID, tenantID, roomID); err != nil {
			return err
		}
		var err error
		t, err = issueTokenTx(ctx, tx, tenantID, examID, roomID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return t, nil
}
