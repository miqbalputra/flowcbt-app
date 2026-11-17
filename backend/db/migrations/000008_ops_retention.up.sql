-- 000008: audit, notifikasi, job file, pengaturan platform, fungsi retensi
CREATE TABLE audit_logs (
  id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  tenant_id   uuid REFERENCES tenants(id) ON DELETE CASCADE,
  actor_type  text NOT NULL CHECK (actor_type IN ('user','student','system','super_admin')),
  actor_id    uuid,
  action      text NOT NULL,                   -- mis. question.update, attempt.reset, penalty.waive, snapshot.view
  entity      text NOT NULL,
  entity_id   uuid,
  meta        jsonb NOT NULL DEFAULT '{}',
  ip          inet,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_tenant_idx ON audit_logs (tenant_id, created_at DESC);
CREATE INDEX audit_logs_entity_idx ON audit_logs (entity, entity_id);
CREATE INDEX audit_logs_created_idx ON audit_logs (created_at);          -- purge > 12 bulan

CREATE TABLE notifications (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  recipient_type text NOT NULL CHECK (recipient_type IN ('user','student')),
  user_id        uuid REFERENCES users(id) ON DELETE CASCADE,
  student_id     uuid REFERENCES students(id) ON DELETE CASCADE,
  type           text NOT NULL,                 -- exam_opening, essay_pending, cheating_flagged, quota_low, score_released
  title          text NOT NULL,
  body           text,
  data           jsonb NOT NULL DEFAULT '{}',
  read_at        timestamptz,
  created_at     timestamptz NOT NULL DEFAULT now(),
  CHECK ((recipient_type = 'user' AND user_id IS NOT NULL) OR (recipient_type = 'student' AND student_id IS NOT NULL))
);
CREATE INDEX notifications_user_idx ON notifications (user_id, created_at DESC) WHERE user_id IS NOT NULL;
CREATE INDEX notifications_student_idx ON notifications (student_id, created_at DESC) WHERE student_id IS NOT NULL;

-- Dedupe email pengingat (langganan H-7/H-3/H-1, retensi saat berakhir/H+3/H+6)
CREATE TABLE notice_log (
  tenant_id  uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  kind       text NOT NULL,
  ref        text NOT NULL DEFAULT '',
  sent_at    timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, kind, ref)
);

-- Import/export besar yang diproses worker (preview -> commit, export semua data)
CREATE TABLE file_jobs (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  kind         text NOT NULL CHECK (kind IN ('student_import','question_import','tenant_export','report_export','login_cards')),
  status       text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','running','awaiting_commit','done','failed','expired')),
  input_key    text,
  output_key   text,
  summary      jsonb NOT NULL DEFAULT '{}',      -- {valid:495, errors:[{row:12,reason:"..."}]} (AC-10)
  error        text,
  created_by   uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  finished_at  timestamptz,
  expires_at   timestamptz
);
CREATE INDEX file_jobs_tenant_idx ON file_jobs (tenant_id, created_at DESC);

CREATE TABLE platform_settings (
  key         text PRIMARY KEY,
  value       jsonb NOT NULL,
  updated_by  uuid REFERENCES users(id),
  updated_at  timestamptz NOT NULL DEFAULT now()
);

-- ---------- Retensi 7 hari (PRD 3.2) ----------
-- Titik mulai = waktu LEBIH AKHIR antara berakhirnya hak akses dan close_at ujian terakhir.
CREATE OR REPLACE FUNCTION tenant_retention_start(p_tenant uuid) RETURNS timestamptz
LANGUAGE sql STABLE AS $$
  SELECT GREATEST(t.access_ended_at, (SELECT max(e.close_at) FROM exams e WHERE e.tenant_id = t.id AND e.status <> 'draft'))
  FROM tenants t WHERE t.id = p_tenant
$$;

-- Tenant yang siap dihapus: read_only, purge_after lewat, tidak ada pembayaran pending
CREATE OR REPLACE VIEW tenants_due_for_purge AS
SELECT t.id, t.purge_after
FROM tenants t
WHERE t.status = 'read_only' AND t.purge_after <= now()
  AND NOT EXISTS (SELECT 1 FROM payments p WHERE p.tenant_id = t.id AND p.status = 'pending')
  AND NOT EXISTS (SELECT 1 FROM subscriptions s WHERE s.tenant_id = t.id AND s.status = 'active' AND s.ends_at > now());

-- Daftar object key yang HARUS dihapus worker dari storage SEBELUM purge_tenant_data()
CREATE OR REPLACE FUNCTION tenant_object_keys(p_tenant uuid) RETURNS SETOF text LANGUAGE sql STABLE AS $$
  SELECT object_key FROM media_assets WHERE tenant_id = p_tenant
  UNION ALL SELECT object_key FROM proctor_snapshots WHERE tenant_id = p_tenant AND deleted_at IS NULL
  UNION ALL SELECT input_key  FROM file_jobs WHERE tenant_id = p_tenant AND input_key IS NOT NULL
  UNION ALL SELECT output_key FROM file_jobs WHERE tenant_id = p_tenant AND output_key IS NOT NULL
$$;

-- Hapus permanen data tenant; baris tenants + payments/invoices/subscriptions/exam_credits dipertahankan
-- (kebutuhan akuntansi & deteksi paket gratis ganda). Dijalankan oleh role sistem.
CREATE OR REPLACE FUNCTION purge_tenant_data(p_tenant uuid) RETURNS void LANGUAGE plpgsql AS $$
BEGIN
  DELETE FROM attempts          WHERE tenant_id = p_tenant;   -- cascade: answers, events, violations, snapshots, sessions
  DELETE FROM exams             WHERE tenant_id = p_tenant;   -- cascade: tokens, rooms, assignments, participants
  DELETE FROM question_sets     WHERE tenant_id = p_tenant;   -- cascade: versions, stimuli, questions, shares
  DELETE FROM refresh_tokens    WHERE tenant_id = p_tenant;
  DELETE FROM students          WHERE tenant_id = p_tenant;
  DELETE FROM classes           WHERE tenant_id = p_tenant;
  DELETE FROM media_assets      WHERE tenant_id = p_tenant;
  DELETE FROM notifications     WHERE tenant_id = p_tenant;
  DELETE FROM file_jobs         WHERE tenant_id = p_tenant;
  DELETE FROM audit_logs        WHERE tenant_id = p_tenant;
  DELETE FROM one_time_tokens   WHERE tenant_id = p_tenant;
  DELETE FROM users             WHERE tenant_id = p_tenant;
  UPDATE tenants SET status = 'purged', purged_at = now(), storage_used_bytes = 0 WHERE id = p_tenant;
END $$;

-- Pemakaian tenant untuk pengecekan batas tier siswa (FR-3.3b) & Super Admin console
CREATE OR REPLACE VIEW tenant_usage AS
SELECT t.id AS tenant_id,
       (SELECT count(*) FROM students s WHERE s.tenant_id = t.id AND s.status = 'active' AND s.merged_into_id IS NULL) AS active_students,
       t.storage_used_bytes, t.storage_quota_bytes
FROM tenants t;
