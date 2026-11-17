package httpx

import (
	"net/http"
	"sync"
	"time"
)

// RateLimit sederhana berbasis memori per kunci (IP atau akun) — PRD 9:
// lebih ketat pada login, token, dan guest login; 429 + Retry-After.
// Pada Fase 5 diganti/lembab dengan adapter Redis agar lintas instance.
type RateLimit struct {
	mu      sync.Mutex
	entries map[string]*bucket
	limit   int
	window  time.Duration
	nowFunc func() time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

// NewRateLimit: limit permintaan per window (mis. 10 per menit) per kunci.
func NewRateLimit(limit int, window time.Duration) *RateLimit {
	rl := &RateLimit{
		entries: make(map[string]*bucket),
		limit:   limit,
		window:  window,
		nowFunc: time.Now,
	}
	go func() {
		// Bersih-bersih entri mati agar map tidak membengkak.
		for range time.Tick(10 * time.Minute) {
			rl.mu.Lock()
			cutoff := rl.nowFunc().Add(-rl.window * 10)
			for k, b := range rl.entries {
				if b.last.Before(cutoff) {
					delete(rl.entries, k)
				}
			}
			rl.mu.Unlock()
		}
	}()
	return rl
}

// Allow mengembalikan true bila kunci masih punya jatah.
func (rl *RateLimit) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := rl.nowFunc()
	fillPerSec := float64(rl.limit) / float64(rl.window) / float64(time.Second)
	b, ok := rl.entries[key]
	if !ok {
		b = &bucket{tokens: float64(rl.limit) - 1, last: now}
		rl.entries[key] = b
		return true
	}
	b.tokens += now.Sub(b.last).Seconds() * fillPerSec
	if b.tokens > float64(rl.limit) {
		b.tokens = float64(rl.limit)
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// Middleware: terapkan rate limit per IP pada handler yang terpasang.
func (rl *RateLimit) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.Allow(clientKey(r)) {
			w.Header().Set("Retry-After", "60")
			ErrorWriter(w, r, AppErr(http.StatusTooManyRequests, "RATE_LIMITED", "terlalu sering. Coba beberapa saat lagi", nil))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientKey(r *http.Request) string {
	// middleware.RealIP sudah menulis X-Real-IP/X-Forwarded-For ke RemoteAddr.
	return r.RemoteAddr
}
