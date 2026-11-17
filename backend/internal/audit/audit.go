// Package audit menulis baris append-only ke audit_logs (PRD 12).
// Aksi sensitif yang WAJIB diaudit: perubahan soal/kunci, reset attempt,
// pembatalan penalti, akses snapshot, undangan/nonaktif guru, reset password.
package audit

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Writer struct {
	sys *pgxpool.Pool
}

func NewWriter(sys *pgxpool.Pool) *Writer { return &Writer{sys: sys} }

type Entry struct {
	TenantID  string
	ActorType string // user | student | system | super_admin
	ActorID   string
	Action    string // mis. teacher.invite, password.reset, attempt.reset
	Entity    string
	EntityID  string
	Meta      map[string]any
	IP        string
}

// Log best-effort: kegagalan audit tidak menggagalkan aksi utama, tapi dicatat log.
func (w *Writer) Log(ctx context.Context, e Entry) {
	meta, _ := json.Marshal(e.Meta)
	if e.Meta == nil {
		meta = []byte("{}")
	}
	_, err := w.sys.Exec(ctx, `
		INSERT INTO audit_logs (tenant_id, actor_type, actor_id, action, entity, entity_id, meta, ip)
		VALUES (NULLIF($1,'')::uuid, $2, NULLIF($3,'')::uuid, $4, $5,
		        NULLIF($6,'')::uuid, $7::jsonb, NULLIF($8,'')::inet)`,
		e.TenantID, e.ActorType, e.ActorID, e.Action, e.Entity, e.EntityID, string(meta), e.IP)
	if err != nil {
		slog.Error("audit write gagal", slog.String("action", e.Action), slog.String("error", err.Error()))
	}
}
