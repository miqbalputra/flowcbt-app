package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims JWT akses (singkat). TenantID kosong hanya untuk super_admin.
// Semua handler membaca TenantID dari sini — tidak pernah dari input klien (PRD 9).
type Claims struct {
	TenantID  string `json:"tid,omitempty"`
	UserID    string `json:"uid,omitempty"`
	StudentID string `json:"sid,omitempty"`
	Role      string `json:"role"`
	Subject   string `json:"sub"` // "user:<id>" | "student:<id>"
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret   []byte
	accessT  time.Duration
	refreshT time.Duration
}

func NewJWTManager(secret string, accessTTL, refreshTTL time.Duration) *JWTManager {
	return &JWTManager{secret: []byte(secret), accessT: accessTTL, refreshT: refreshTTL}
}

func (m *JWTManager) AccessTTL() time.Duration  { return m.accessT }
func (m *JWTManager) RefreshTTL() time.Duration { return m.refreshT }

// IssueAccess membuat token akses singkat untuk user tenant atau siswa.
func (m *JWTManager) IssueAccess(c Claims) (string, error) {
	now := time.Now()
	c.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(now.Add(m.accessT)),
		IssuedAt:  jwt.NewNumericDate(now),
		Issuer:    "flowcbt",
	}
	if c.Subject != "" {
		c.RegisteredClaims.Subject = c.Subject
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(m.secret)
}

// ParseAccess memvalidasi signature & kedaluwarsa.
func (m *JWTManager) ParseAccess(token string) (*Claims, error) {
	claims := &Claims{}
	t, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("algoritma tanda tangan tidak didukung")
		}
		return m.secret, nil
	})
	if err != nil || !t.Valid {
		return nil, errors.New("token tidak valid")
	}
	return claims, nil
}

// NewRefreshRaw menghasilkan refresh token polos (dikirim ke klien) yang nantinya
// disimpan sebagai SHA-256 hash di refresh_tokens.token_hash (rotasi, FR-1.6).
func NewRefreshRaw() (raw string, hash string, err error) {
	b := make([]byte, 48)
	if _, err := randRead(b); err != nil {
		return "", "", err
	}
	return hexEncode(b), hashSHA256(b), nil
}
