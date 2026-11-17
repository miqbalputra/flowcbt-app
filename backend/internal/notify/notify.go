// Package notify mengirim email via asynq (Redis) dengan fallback kirim langsung
// bila Redis tidak tersedia (dev tanpa infrastruktur). Template Bahasa Indonesia.
package notify

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/flowcbt/backend/internal/platform"
	"github.com/hibiken/asynq"
)

const TypeEmail = "email:send"

type EmailJob struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	HTML    string `json:"html"`
}

// Queue membungkus asynq client + mailer, memilih jalur kirim otomatis.
type Queue struct {
	client *asynq.Client
	mailer *platform.Mailer
}

func NewQueue(redisAddr string, mailer *platform.Mailer) *Queue {
	return &Queue{
		client: asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr}),
		mailer: mailer,
	}
}

// SendEmail: antre lewat asynq; bila Redis gagal dipakai, kirim langsung
// (job email tidak boleh hilang hanya karena infrastruktur dev minimal).
func (q *Queue) SendEmail(ctx context.Context, job EmailJob) error {
	payload, _ := json.Marshal(job)
	if _, err := q.client.EnqueueContext(ctx,
		asynq.NewTask(TypeEmail, payload),
		asynq.MaxRetry(5),
		asynq.Timeout(30*time.Second),
	); err != nil {
		slog.Warn("asynq tidak tersedia, kirim email langsung", slog.String("error", err.Error()))
		return q.mailer.Send(ctx, job.To, job.Subject, job.HTML)
	}
	return nil
}

// HandleEmailTask handler worker (cmd/worker).
func HandleEmailTask(mailer *platform.Mailer) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var job EmailJob
		if err := json.Unmarshal(t.Payload(), &job); err != nil {
			return err
		}
		return mailer.Send(ctx, job.To, job.Subject, job.HTML)
	}
}

// ---- Template email (Bahasa Indonesia, FR-13.1) ----

func layout(title, body, actionURL, actionLabel string) string {
	var actionHTML string
	if actionURL != "" {
		actionHTML = `<p style="margin:24px 0"><a href="` + actionURL +
			`" style="background:#2f6fed;color:#fff;padding:12px 20px;border-radius:8px;text-decoration:none">` +
			actionLabel + `</a></p><p style="font-size:12px;color:#6b7280">Tautan berlaku terbatas. Abaikan email ini bila Anda tidak merasa meminta.` +
			`</p>`
	}
	return `<div style="font-family:system-ui,sans-serif;max-width:520px;margin:0 auto">
<h2>` + title + `</h2><div style="font-size:15px;line-height:1.6">` + body + actionHTML + `</div>
<p style="font-size:12px;color:#6b7280">Email otomatis dari FlowCBT. ` + htmlTimestamp() + `</p></div>`
}

func htmlTimestamp() string { return time.Now().Format("2006-01-02 15:04 MST") }

func VerifyEmailHTML(rawToken string) string {
	url := baseURL() + "/api/v1/auth/verify-email?token=" + rawToken
	// Sampai handler GET tersedia, siswa/guru menempelkan token di UI.
	return layout(
		"Verifikasi email Anda",
		"Selamat datang di FlowCBT! Masukkan kode berikut di halaman verifikasi:<br><b style=\"font-size:18px\">"+rawToken+"</b>",
		url, "Verifikasi email",
	)
}

func TeacherInviteHTML(schoolName, rawToken string) string {
	return layout(
		"Undangan guru — "+schoolName,
		"Anda diundang menjadi guru di <b>"+schoolName+"</b> di FlowCBT. "+
			"Undang akun Anda dengan kode berikut di halaman undangan:<br><b style=\"font-size:18px\">"+rawToken+"</b>",
		baseURL()+"/login", "Masuk FlowCBT",
	)
}

func ResetPasswordHTML(rawToken string) string {
	return layout(
		"Reset password",
		"Kode reset password Anda:<br><b style=\"font-size:18px\">"+rawToken+"</b>"+
			"<br>Kode berlaku 1 jam.",
		"", "",
	)
}

func baseURL() string {
	if b := strings.TrimSpace(env("APP_BASE_URL", "")); b != "" {
		return strings.TrimRight(b, "/")
	}
	return "http://localhost:5173"
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
