// Package user: manajemen guru di tenant (undangan, daftar, nonaktif) dan
// alur reset password lintas tenant (M1, FR-1.5, FR-1.2, S3).
package user

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/flowcbt/backend/internal/auth"
)

var errNoRows = pgx.ErrNoRows

type Store struct {
	sys *pgxpool.Pool
}

func NewStore(sys *pgxpool.Pool) *Store { return &Store{sys: sys} }

type TeacherRow struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

func (s *Store) TeachersOfTenant(ctx context.Context, tenantID string) ([]TeacherRow, error) {
	rows, err := s.sys.Query(ctx, `
		SELECT id::text, name, email::text, role, status
		FROM users WHERE tenant_id = $1::uuid AND role IN ('owner','teacher')
		ORDER BY created_at`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []TeacherRow{}
	for rows.Next() {
		var t TeacherRow
		if err := rows.Scan(&t.ID, &t.Name, &t.Email, &t.Role, &t.Status); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

// CreateInvitedGuru: status 'invited' + token undangan sekali pakai (7 hari).
func (s *Store) CreateInvitedGuru(ctx context.Context, tenantID, name, email, phone string, now time.Time) (userID, inviteRawToken string, err error) {
	raw, hash, err := auth.NewOneTimeToken()
	if err != nil {
		return "", "", err
	}
	tx, err := s.sys.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	err = tx.QueryRow(ctx,
		`INSERT INTO users (tenant_id, role, name, email, phone, status) VALUES ($1::uuid,'teacher',$2,lower($3),NULLIF($4,''),'invited') RETURNING id::text`,
		tenantID, name, email, phone).Scan(&userID)
	if err != nil {
		if isUnique(err) {
			return "", "", errors.New("email sudah terdaftar di tenant ini")
		}
		return "", "", err
	}
	if _, err = tx.Exec(ctx,
		`INSERT INTO one_time_tokens (tenant_id, user_id, email, purpose, token_hash, payload, expires_at)
		 VALUES ($1::uuid,$2::uuid,lower($3),'teacher_invite',$4,$5::jsonb,$6)`,
		tenantID, userID, email, hash, `{"must_set_password":true}`, now.Add(7*24*time.Hour)); err != nil {
		return "", "", err
	}
	return userID, raw, tx.Commit(ctx)
}

// SetTeacherStatus: nonaktifkan / pulihkan guru (matriks 2.1; hanya owner).
func (s *Store) SetTeacherStatus(ctx context.Context, tenantID, userID, status string) error {
	tag, err := s.sys.Exec(ctx,
		`UPDATE users SET status = $3 WHERE id = $2::uuid AND tenant_id = $1::uuid AND role IN ('owner','teacher')`,
		tenantID, userID, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ---- Reset password (FR-1.2) ----

// CreateResetToken membuat token reset 1 jam bila email terdaftar.
// Anti-enum: pemanggil menentukan sendiri respons netral bila tidak ada.
func (s *Store) CreateResetToken(ctx context.Context, email string, now time.Time) (raw string, ok bool, err error) {
	raw, hash, err := auth.NewOneTimeToken()
	if err != nil {
		return "", false, err
	}
	tag, err := s.sys.Exec(ctx, `
		INSERT INTO one_time_tokens (user_id, email, purpose, token_hash, expires_at)
		SELECT id, email, 'reset_password', $3, $4 FROM users WHERE email = lower($1) AND role <> 'super_admin'`,
		email, email, hash, now.Add(time.Hour))
	if err != nil {
		return "", false, err
	}
	return raw, tag.RowsAffected() > 0, nil
}

// ConsumeResetToken: token valid → set password baru (hash) + aktifkan user
// bila status invited (akun baru yang kehilangan kesempatan verifikasi).
// Idempoten: token terpakai/kedaluwarsa → ErrNotFound.
func (s *Store) ConsumeResetToken(ctx context.Context, rawToken, passwordHash string, now time.Time) error {
	tx, err := s.sys.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		UPDATE one_time_tokens SET used_at = $1
		WHERE token_hash = $2 AND purpose = 'reset_password'
		  AND used_at IS NULL AND expires_at > $1`,
		now, auth.HashOneTimeToken(rawToken))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrNotFound
	}
	if _, err = tx.Exec(ctx, `
		UPDATE users u SET password_hash = $1, status = 'active'
		FROM one_time_tokens t WHERE t.token_hash = $2 AND t.user_id = u.id`,
		passwordHash, auth.HashOneTimeToken(rawToken)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func isUnique(err error) bool { return err != nil && strings.Contains(err.Error(), "23505") }
