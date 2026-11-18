package platform

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Storage: penyimpanan objek (media soal, gambar/audio stimulus, snapshot kamera).
// v1: LocalFS. Antarmuka siap S3/MinIO saat STORAGE_BACKEND=s3 dikonfigurasi.
type Storage interface {
	Save(ctx context.Context, objectKey, mime string, r io.Reader) (size int64, err error)
	Get(ctx context.Context, objectKey string) (io.ReadCloser, error)
	Delete(ctx context.Context, objectKey string) error
}

// NewStorage menentukan backend dari env.
func NewStorage() Storage {
	if v := os.Getenv("STORAGE_BACKEND"); v != "" {
		switch strings.ToLower(v) {
		case "s3", "minio":
			return &S3Storage{Bucket: envOr("STORAGE_S3_BUCKET", "flowcbt")}
		}
	}
	return &LocalStorage{Dir: envOr("MEDIA_DIR", "storage/media")}
}

// envOr terpisah dari platform.env (config API) agar tidak bentrok nama.
func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// envOr terpisah dari platform.env (config API) agar tidak bentrok nama.
// (Definisi dipindah ke bawah agar file terbaca urut.)

// ---- Local storage (dev & single-VPS) ----

type LocalStorage struct{ Dir string }

func (s *LocalStorage) path(objectKey string) string {
	return filepath.Join(s.Dir, filepath.FromSlash(objectKey))
}

func (s *LocalStorage) Save(_ context.Context, objectKey, _ string, r io.Reader) (int64, error) {
	objectKey = filepath.Clean(objectKey)
	// Cegah path traversal
	if strings.HasPrefix(objectKey, "..") || filepath.IsAbs(objectKey) {
		return 0, fmt.Errorf("object key tidak valid")
	}
	p := s.path(objectKey)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return 0, err
	}
	f, err := os.CreateTemp(filepath.Dir(p), ".upload-*")
	if err != nil {
		return 0, err
	}
	defer func() {
		f.Close()
		if err != nil {
			os.Remove(f.Name())
		}
	}()
	n, err := io.Copy(f, r)
	if err != nil {
		return 0, err
	}
	if err = f.Close(); err != nil {
		return 0, err
	}
	err = os.Rename(f.Name(), p)
	if err != nil {
		return 0, err
	}
	return int64(n), nil
}

func (s *LocalStorage) Get(_ context.Context, objectKey string) (io.ReadCloser, error) {
	objectKey = filepath.Clean(objectKey)
	if strings.HasPrefix(objectKey, "..") || filepath.IsAbs(objectKey) {
		return nil, fmt.Errorf("object key tidak valid")
	}
	return os.Open(s.path(objectKey))
}

func (s *LocalStorage) Delete(_ context.Context, objectKey string) error {
	objectKey = filepath.Clean(objectKey)
	if strings.HasPrefix(objectKey, "..") || filepath.IsAbs(objectKey) {
		return fmt.Errorf("object key tidak valid")
	}
	err := os.Remove(s.path(objectKey))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// S3Storage stub: aktif hanya ketika konfigur lengkap; endpoint & kredensial
// dibaca saat deploy Coolify (PRD 7.4). v1 mengembalikan error jelas.
type S3Storage struct {
	Bucket string
	once   sync.Once
}

func (s *S3Storage) initErr() error {
	s.once.Do(func() {})
	if s.Bucket == "" {
		return fmt.Errorf("STORAGE_S3_BUCKET belum dikonfigurasi")
	}
	return fmt.Errorf("backend S3 belum diimplementasi v1 — gunakan STORAGE_BACKEND=default (LocalFS) atau konfigurasikan MinIO")
}

func (s *S3Storage) Save(ctx context.Context, key, mime string, r io.Reader) (int64, error) {
	return 0, s.initErr()
}
func (s *S3Storage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	return nil, s.initErr()
}
func (s *S3Storage) Delete(ctx context.Context, key string) error { return s.initErr() }

// Walk membuat object key unik per tenant: tenant/<id>/<uuid>.<ext>
func ObjectKey(tenantID, ext string) string {
	ext = strings.TrimPrefix(strings.ToLower(ext), ".")
	if ext == "" || len(ext) > 5 || strings.ContainsAny(ext, "/\\") {
		ext = "bin"
	}
	return "tenant/" + tenantID + "/" + idgen() + "." + ext
}

// idgen: 26 hex lowercase acak.
func idgen() string {
	b := make([]byte, 16)
	if _, err := cryptoRandRead(b); err != nil {
		return strings.ReplaceAll(fmt.Sprintf("%x", nowUnixNano()), "\n", "")
	}
	const hexd = "0123456789abcdef"
	out := make([]byte, 32)
	for i, c := range b {
		out[i*2] = hexd[c>>4]
		out[i*2+1] = hexd[c&0x0f]
	}
	return string(out)
}

// ---- snapshot kamera: penanda tersimpan di flags_json; file via Storage ----

// RegisterSnapshotDir dipakai worker proctor (S22) — disiapkan agar worker
// tidak menembus modul lain (depguard).
var cameraSnapshotDirs sync.Map

func RegisterSnapshotDir(tenantID, dir string) { cameraSnapshotDirs.Store(tenantID, dir) }

// ---- util tipe kecil ----

// cryptoRandRead: 16 byte acak daripada dependensi baru.
func cryptoRandRead(b []byte) (int, error) { return rand.Read(b) }

func nowUnixNano() int64 { return time.Now().UnixNano() }
