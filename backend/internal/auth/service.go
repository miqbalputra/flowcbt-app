package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/flowcbt/backend/internal/platform"
)

var (
	ErrEmailTaken     = errors.New("email sudah terdaftar")
	ErrTenantCodeUsed = errors.New("kode sekolah sudah dipakai")
	ErrInvalidCreds   = errors.New("email atau password salah")
)

// LockedError: akun sementara dikunci setelah percobaan gagal berulang (FR-1.2).
type LockedError struct{ Until time.Time }

func (e *LockedError) Error() string {
	return "akun terkunci sementara, coba lagi dalam " + e.Until.Sub(time.Now()).Round(time.Second).String()
}

var (
	codeRe   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,39}$`)
	levelSet = map[string]bool{
		"paud_tk": true, "sd_mi": true, "smp_mts": true, "sma_smk_ma": true,
		"perguruan_tinggi": true, "kursus_bimbel": true, "lainnya": true,
	}
)

type Service struct {
	store *Store
	jwt   *JWTManager
	clock platform.Clock
}

func NewService(store *Store, jwt *JWTManager, clock platform.Clock) *Service {
	return &Service{store: store, jwt: jwt, clock: clock}
}

// RegisterValidasi memvalidasi payload registrasi (FR-1.1) tanpa menyentuh DB.
func (s *Service) RegisterValidasi(name, email, password, tenantName, tenantCode, level string) error {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(tenantName) == "" {
		return errors.New("nama dan nama sekolah wajib diisi")
	}
	if !strings.Contains(email, "@") {
		return errors.New("email tidak valid")
	}
	if !codeRe.MatchString(tenantCode) {
		return errors.New("kode sekolah 3-40 karakter: huruf kecil, angka, atau tanda hubung")
	}
	if !levelSet[level] {
		return errors.New("jenjang tidak valid")
	}
	return PasswordPolicyError(password)
}

type RegisterResult struct {
	TenantID     string
	UserID       string
	VerifyURLRaw string // token verifikasi polos; dikirim via email (dev: log)
}

func (s *Service) Register(ctx context.Context, in RegisterInput, password string) (*RegisterResult, error) {
	if err := s.RegisterValidasi(in.OwnerName, in.Email, password, in.TenantName, in.TenantCode, in.Level); err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(in.Email))))
	in.EmailHash = hex.EncodeToString(sum[:])
	in.Now = s.clock.Now()

	res := &RegisterResult{}
	tenantID, userID, raw, err := s.store.CreateTenantWithOwner(ctx, in)
	if err != nil {
		if isUniqueViolation(err) {
			if strings.Contains(err.Error(), "users_email_key") {
				return nil, ErrEmailTaken
			}
			return nil, ErrTenantCodeUsed
		}
		return nil, err
	}
	res.TenantID, res.UserID, res.VerifyURLRaw = tenantID, userID, raw
	return res, nil
}

// Login memverifikasi kredensial, lockout, dan status verifikasi email (FR-1.2).
// Kembalikan access token + refresh token (rotasi keluarga baru).
func (s *Service) Login(ctx context.Context, email, password string) (*Tokens, error) {
	now := s.clock.Now()
	u, err := s.store.UserByEmail(ctx, email)
	if errors.Is(err, ErrNotFound) {
		// Anti-enum: jawaban sama persis untuk email yang tak ada.
		return nil, ErrInvalidCreds
	}
	if err != nil {
		return nil, err
	}
	if u.LockedUntil != nil && u.LockedUntil.After(now) {
		return nil, &LockedError{Until: *u.LockedUntil}
	}
	if u.PasswordHash == nil || !VerifyPassword(password, *u.PasswordHash) {
		if _, ferr := s.store.LoginFailure(ctx, u.ID, now); ferr != nil {
			return nil, ferr
		}
		return nil, ErrInvalidCreds
	}
	if u.Status == "invited" {
		return nil, errors.New("verifikasi email terlebih dahulu (cek kotak masuk)")
	}
	if u.Status == "disabled" {
		return nil, errors.New("akun dinonaktifkan")
	}
	if u.EmailVerifiedAt == nil {
		return nil, errors.New("verifikasi email terlebih dahulu (cek kotak masuk)")
	}
	if terr := s.store.LoginSuccess(ctx, u.ID, now); terr != nil {
		return nil, terr
	}
	return s.issueTokens(ctx, "user", u.ID, deref(u.TenantID), u.Role)
}

type Tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

func (s *Service) issueTokens(ctx context.Context, subjectType, subjectID, tenantID, role string) (*Tokens, error) {
	_, familyHash, err := NewRefreshRaw() // family_id random; raw tidak dikirim ke klien
	if err != nil {
		return nil, err
	}
	claims := Claims{TenantID: tenantID, Role: role, Subject: subjectType + ":" + subjectID}
	if subjectType == "user" {
		claims.UserID = subjectID
	} else {
		claims.StudentID = subjectID
	}
	access, err := s.jwt.IssueAccess(claims)
	if err != nil {
		return nil, err
	}
	refreshRaw, refreshHash, err := NewRefreshRaw()
	if err != nil {
		return nil, err
	}
	if err := s.store.SaveRefreshToken(ctx, subjectType, subjectID, tenantID, familyHash, refreshHash,
		s.clock.Now().Add(s.jwt.RefreshTTL())); err != nil {
		return nil, err
	}
	return &Tokens{AccessToken: access, RefreshToken: refreshRaw, ExpiresIn: int64(s.jwt.AccessTTL().Seconds())}, nil
}

// Refresh memutar refresh token (rotasi). Reuse token yang sudah dicabut
// mencabut seluruh keluarga sesi.
func (s *Service) Refresh(ctx context.Context, rawToken string) (*Tokens, error) {
	hash := hashSHA256([]byte(rawToken))
	row, err := s.store.RefreshByHash(ctx, hash)
	if err != nil {
		return nil, ErrInvalidCreds
	}
	if row.Expired {
		return nil, ErrInvalidCreds
	}
	if row.IsRevoked {
		_ = s.store.RevokeFamily(ctx, row.FamilyID)
		return nil, ErrInvalidCreds
	}
	if row.UserID == nil {
		return nil, ErrInvalidCreds
	}
	u, err := s.store.UserByID(ctx, *row.UserID)
	if err != nil {
		return nil, ErrInvalidCreds
	}
	newRaw, newHash, err := NewRefreshRaw()
	if err != nil {
		return nil, err
	}
	if err := s.store.RotateRefresh(ctx, row.ID, row.FamilyID, "user", u.ID, deref(row.TenantID), newHash,
		s.clock.Now().Add(s.jwt.RefreshTTL())); err != nil {
		return nil, err
	}
	claims := Claims{
		TenantID: deref(row.TenantID), UserID: u.ID, Role: u.Role,
		Subject: "user:" + u.ID,
	}
	access, err := s.jwt.IssueAccess(claims)
	if err != nil {
		return nil, err
	}
	return &Tokens{AccessToken: access, RefreshToken: newRaw, ExpiresIn: int64(s.jwt.AccessTTL().Seconds())}, nil
}

func (s *Service) VerifyEmail(ctx context.Context, rawToken string) error {
	return s.store.VerifyEmail(ctx, rawToken, s.clock.Now())
}

func isUniqueViolation(err error) bool { return err != nil && strings.Contains(err.Error(), "23505") }

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
