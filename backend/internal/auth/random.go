package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// jwt.go & store.go memakai helper ini; dipisah agar mudah diuji.
func randRead(b []byte) (int, error) { return rand.Read(b) }

// hexEncode dan hashSHA256 dipakai menyimpan refresh token & one-time token.
func hexEncode(b []byte) string {
	dst := make([]byte, hex.EncodedLen(len(b)))
	hex.Encode(dst, b)
	return string(dst)
}

func hashSHA256(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
