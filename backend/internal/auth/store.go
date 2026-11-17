package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store menulis/membaca baris auth. Semua fungsi menerima *pgxpool.Conn-agnostik
// pool: cukup System (operasi lintas tenant: registrasi, login, refresh).
type Store struct {
	System *pgxpool.Pool
}

type RegisterInput struct {
	TenantName   string
	TenantCode   string
	Level        string
	OwnerName    string
	Email        string
	Phone        string
	PasswordHash string
	EmailHash    string // owner_email_hash untuk anti-penyalahgunaan paket gratis
	Now          time.Time
}

// CreateTenantWithOwner membuat tenant + owner (status invited, verifikasi email
// dulu sebelum akses penuh) + trial otomatis 14 hari dari plan free_trial.
// Menjalankan semuanya dalam SATU transaksi sistem.
func (s *Store) CreateTenantWithOwner(ctx context.Context, in RegisterInput) (tenantID, userID, verifyTokenRaw string, err error) {
	raw, hash, terr := NewOneTimeToken()
	if terr != nil {
		return "", "", "", terr
	}
	tx, err := s.System.Begin(ctx)
	if err != nil {
		return "", "", "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	err = tx.QueryRow(ctx, `
		INSERT INTO tenants (code, name, level, phone, free_plan_used, name_normalized,
		                     phone_normalized, owner_email_hash)
		VALUES ($1, $2, $3, NULLIF($4, ''), true, lower($2),
		        NULLIF(lower(regexp_replace($4, '\s+', '', 'g')), ''),
		        NULLIF($5, ''))
		RETURNING id`,
		in.TenantCode, in.TenantName, in.Level, in.Phone, in.EmailHash,
	).Scan(&tenantID)
	if err != nil {
		return "", "", "", err
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO users (tenant_id, role, name, email, phone, password_hash, status)
		VALUES ($1, 'owner', $2, lower($3), NULLIF($4, ''), $5, 'invited')
		RETURNING id`,
		tenantID, in.OwnerName, in.Email, in.Phone, in.PasswordHash,
	).Scan(&userID)
	if err != nil {
		return "", "", "", err
	}

	// Verifikasi email: token sekali pakai 24 jam.
	if _, err = tx.Exec(ctx, `
		INSERT INTO one_time_tokens (tenant_id, user_id, email, purpose, token_hash, expires_at)
		VALUES ($1, $2, lower($3), 'verify_email', $4, $5)`,
		tenantID, userID, in.Email, hash, in.Now.Add(24*time.Hour),
	); err != nil {
		return "", "", "", err
	}

	// Trial otomatis (keputusan #2): 1–10 siswa, 1x ujian, 14 hari.
	if _, err = tx.Exec(ctx, `
		INSERT INTO subscriptions (tenant_id, plan_id, type, status, student_limit, exam_quota, starts_at, ends_at)
		SELECT $1, p.id, 'trial', 'active', p.student_max, p.exam_quota, $2, $2 + make_interval(days => p.duration_days)
		FROM plans p WHERE p.code = 'free_trial'`,
		tenantID, in.Now,
	); err != nil {
		return "", "", "", err
	}

	if err = tx.Commit(ctx); err != nil {
		return "", "", "", err
	}
	return tenantID, userID, raw, nil
}

type UserRow struct {
	ID               string
	TenantID         *string
	Role             string
	Name             string
	Email            string
	PasswordHash     *string
	Status           string
	EmailVerifiedAt  *time.Time
	FailedLoginCount int
	LockedUntil      *time.Time
}

var ErrNotFound = errors.New("tidak ditemukan")

// UserByID membaca user lintas tenant (role sistem) untuk refresh & undangan.
func (s *Store) UserByID(ctx context.Context, id string) (UserRow, error) {
	row := s.System.QueryRow(ctx, `
		SELECT id, tenant_id, role, name, email::text, password_hash, status,
		       email_verified_at, failed_login_count, locked_until
		FROM users WHERE id = $1::uuid`, id)
	var u UserRow
	if err := row.Scan(&u.ID, &u.TenantID, &u.Role, &u.Name, &u.Email, &u.PasswordHash,
		&u.Status, &u.EmailVerifiedAt, &u.FailedLoginCount, &u.LockedUntil); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return u, ErrNotFound
		}
		return u, err
	}
	return u, nil
}

func (s *Store) UserByEmail(ctx context.Context, email string) (UserRow, error) {
	row := s.System.QueryRow(ctx, `
		SELECT id, tenant_id, role, name, email::text, password_hash, status,
		       email_verified_at, failed_login_count, locked_until
		FROM users WHERE email = lower($1) AND role <> 'super_admin'`, email)
	var u UserRow
	if err := row.Scan(&u.ID, &u.TenantID, &u.Role, &u.Name, &u.Email, &u.PasswordHash,
		&u.Status, &u.EmailVerifiedAt, &u.FailedLoginCount, &u.LockedUntil); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return u, ErrNotFound
		}
		return u, err
	}
	return u, nil
}

// LoginFailure mencatat kegagalan dan mengembalikan waktu berakhirnya lockout (0 = tidak terkunci).
func (s *Store) LoginFailure(ctx context.Context, userID string, now time.Time) (time.Time, error) {
	var lockedUntil *time.Time
	err := s.System.QueryRow(ctx, `
		UPDATE users SET failed_login_count = failed_login_count + 1,
		                 locked_until = CASE
		                   WHEN failed_login_count + 1 >= 5
		                     THEN $2 + make_interval(secs => $3)
		                   ELSE locked_until END
		WHERE id = $1 RETURNING locked_until`,
		userID, now, int64(LockoutDuration(5).Seconds()),
	).Scan(&lockedUntil)
	if err != nil || lockedUntil == nil {
		return time.Time{}, err
	}
	return *lockedUntil, nil
}

func (s *Store) LoginSuccess(ctx context.Context, userID string, now time.Time) error {
	_, err := s.System.Exec(ctx, `
		UPDATE users SET failed_login_count = 0, locked_until = NULL, last_login_at = $2
		WHERE id = $1`, userID, now)
	return err
}

// VerifyEmail menandai token terpakai dan mengaktifkan user. Idempoten:
// token yang sudah terpakai dibalas tanpa error (tapi user tidak berubah).
func (s *Store) VerifyEmail(ctx context.Context, rawToken string, now time.Time) error {
	tag, err := s.System.Exec(ctx, `
		UPDATE one_time_tokens SET used_at = $1
		WHERE token_hash = $2 AND purpose = 'verify_email'
		  AND used_at IS NULL AND expires_at > $1`,
		now, HashOneTimeToken(rawToken))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_, err = s.System.Exec(ctx, `
		UPDATE users u SET status = 'active', email_verified_at = $1
		FROM one_time_tokens t
		WHERE t.token_hash = $2 AND t.user_id = u.id`,
		now, HashOneTimeToken(rawToken))
	return err
}

// SaveRefreshToken menyimpan hash refresh token (rotasi, FR-1.6).
func (s *Store) SaveRefreshToken(ctx context.Context, subjectType, subjectID, tenantID, familyID, hash string, expiresAt time.Time) error {
	_, err := s.System.Exec(ctx, `
		INSERT INTO refresh_tokens (subject_type, user_id, student_id, tenant_id, family_id, token_hash, expires_at)
		VALUES ($1,
		        CASE WHEN $2 = 'user' THEN $3::uuid END,
		        CASE WHEN $2 = 'student' THEN $3::uuid END,
		        NULLIF($4, '')::uuid, $5::uuid, $6, $7)`,
		subjectType, subjectType, subjectID, tenantID, familyID, hash, expiresAt)
	return err
}

type RefreshRow struct {
	ID        string
	FamilyID  string
	IsRevoked bool
	Expired   bool
	UserID    *string
	TenantID  *string
}

func (s *Store) RefreshByHash(ctx context.Context, hash string) (RefreshRow, error) {
	var r RefreshRow
	err := s.System.QueryRow(ctx, `
		SELECT id::text, family_id::text, revoked_at IS NOT NULL AS is_revoked, expires_at < now() AS expired,
		       user_id::text, tenant_id::text
		FROM refresh_tokens WHERE token_hash = $1`, hash,
	).Scan(&r.ID, &r.FamilyID, &r.IsRevoked, &r.Expired, &r.UserID, &r.TenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}

// RotateRefresh mencabut token lama dan menyimpan penggantinya dalam satu transaksi.
func (s *Store) RotateRefresh(ctx context.Context, oldID, familyID, subjectType, subjectID, tenantID, newHash string, expiresAt time.Time) error {
	tx, err := s.System.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now(), revoked_by = 'rotated' WHERE id = $1::uuid`, oldID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO refresh_tokens (subject_type, user_id, student_id, tenant_id, family_id, token_hash, expires_at)
		VALUES ($1,
		        CASE WHEN $2 = 'user' THEN $3::uuid END,
		        CASE WHEN $2 = 'student' THEN $3::uuid END,
		        NULLIF($4, '')::uuid, $5::uuid, $6, $7)`,
		subjectType, subjectType, subjectID, tenantID, familyID, newHash, expiresAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RevokeFamily dipakai saat reuse terdeteksi: seluruh family dicabut.
func (s *Store) RevokeFamily(ctx context.Context, familyID string) error {
	_, err := s.System.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now(), revoked_by = 'reuse' WHERE family_id = $1::uuid AND revoked_at IS NULL`,
		familyID)
	return err
}
