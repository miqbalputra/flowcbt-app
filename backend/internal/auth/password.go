// Package auth: registrasi, login, refresh rotasi, verifikasi email, lockout (M1, S2/S3).
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/argon2"
)

// ---- Argon2id (PRD 6: hash password argon2id/bcrypt) ----

type argonParams struct {
	time    uint32
	memory  uint32 // KiB
	threads uint8
	keyLen  uint32
	saltLen uint32
}

var defaultParams = argonParams{time: 3, memory: 64 * 1024, threads: 2, keyLen: 32, saltLen: 16}

// HashPassword menghasilkan string PHC: $argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>
func HashPassword(password string) (string, error) {
	if len(password) < 8 {
		return "", errors.New("password minimal 8 karakter")
	}
	salt := make([]byte, defaultParams.saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, defaultParams.time, defaultParams.memory, defaultParams.threads, defaultParams.keyLen)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		defaultParams.memory, defaultParams.time, defaultParams.threads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// VerifyPassword membandingkan password dengan hash PHC argon2id (constant-time).
func VerifyPassword(password, phc string) bool {
	parts := strings.Split(strings.TrimPrefix(phc, "$"), "$")
	if len(parts) != 5 || parts[0] != "argon2id" {
		return false
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[2], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, t, m, p, uint32(len(want)))
	return timingEqual(got, want)
}

func timingEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

// ---- Password strength sederhana (FR-1.1) ----

func PasswordPolicyError(password string) error {
	var hasLetter, hasDigit bool
	for _, r := range password {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if len(password) < 8 || !hasLetter || !hasDigit {
		return errors.New("password minimal 8 karakter dan memuat huruf serta angka")
	}
	return nil
}

// ---- Token sekali pakai (one_time_tokens.token_hash = SHA-256) ----

// NewOneTimeToken mengembalikan token polos (dikirim via email) dan hash-nya (disimpan).
func NewOneTimeToken() (raw string, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	return raw, hex.EncodeToString(sum[:]), nil
}

func HashOneTimeToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// LockoutRule sesuai FR-1.2: kunci sementara setelah percobaan gagal berulang.
func LockoutDuration(failedCount int) time.Duration {
	// 0-4 gagal: tidak terkunci; >=5: 5 menit; >=10: 15 menit (tidak bertumpuk lebih jauh).
	switch {
	case failedCount < 5:
		return 0
	case failedCount < 10:
		return 5 * time.Minute
	default:
		return 15 * time.Minute
	}
}
