// Cache Redis untuk paket soal versi terbit (S11, PRD 9/FR-4.7).
// Versi terbit BERSIFAT IMMUTABLE (FR-4.8) sehingga cache tanpa invalidasi
// aman: kunci = ID versi, TTL panjang. Redis mati → fallback DB (best effort).
package exam

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type QuestionCache struct {
	rdb *redis.Client
}

// NewQuestionCache: addr kosong → cache dinonaktifkan (dev tanpa Redis).
func NewQuestionCache(addr string) *QuestionCache {
	if addr == "" {
		return &QuestionCache{}
	}
	return &QuestionCache{rdb: redis.NewClient(&redis.Options{Addr: addr})}
}

const questionCacheTTL = 24 * time.Hour

func (c *QuestionCache) Get(ctx context.Context, versionID string) ([]byte, bool) {
	if c.rdb == nil {
		return nil, false
	}
	b, err := c.rdb.Get(ctx, "pkgv:"+versionID).Bytes()
	if err != nil {
		return nil, false
	}
	return b, true
}

func (c *QuestionCache) Set(ctx context.Context, versionID string, data []byte) {
	if c.rdb == nil {
		return
	}
	_ = c.rdb.Set(ctx, "pkgv:"+versionID, data, questionCacheTTL).Err()
}
