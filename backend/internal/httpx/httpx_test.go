package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flowcbt/backend/internal/httpx"
)

func TestHealthz(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	httpx.Healthz(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200", w.Code)
	}
	if got := w.Body.String(); got[:len(got)-1] != `{"status":"ok"}` {
		t.Fatalf("body = %q", got)
	}
}

func TestErrorWriterStableCodes(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		want    string // substring yang harus muncul
	}{
		{"quota", httpx.AppErr(400, "QUOTA_EXCEEDED", "kuota habis", nil), 400, "QUOTA_EXCEEDED"},
		{"not found", httpx.ErrNotFound(), 404, "NOT_FOUND"},
		{"fallback", contextCanceledErr{}, 500, "INTERNAL"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			httpx.ErrorWriter(w, httptest.NewRequest("GET", "/x", nil), c.err)
			if w.Code != c.status {
				t.Fatalf("status = %d, ingin %d", w.Code, c.status)
			}
			if !contains(w.Body.String(), c.want) {
				t.Fatalf("body = %q, ingin mengandung %q", w.Body.String(), c.want)
			}
		})
	}
}

type contextCanceledErr struct{}

func (contextCanceledErr) Error() string { return "context canceled" }

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
