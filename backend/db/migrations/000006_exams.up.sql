-- 000006: ujian, token, ruang, penugasan, peserta
CREATE TABLE exams (
  id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id               uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  question_set_id         uuid NOT NULL REFERENCES question_sets(id),
  set_version_id          uuid REFERENCES question_set_versions(id),  -- terisi saat publish
  name                    text NOT NULL,
  status                  text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published','paused','closed','cancelled')),
  -- jadwal & aturan (FR-5.1 .. 5.5)
  open_at                 timestamptz,
  close_at                timestamptz,
  duration_minutes        integer CHECK (duration_minutes > 0),
  close_policy            text NOT NULL DEFAULT 'grace' CHECK (close_policy IN ('hard','grace')),
  login_mode              text NOT NULL DEFAULT 'both' CHECK (login_mode IN ('account','guest','both')),
  max_attempts            integer CHECK (max_attempts IS NULL OR max_attempts > 0),   -- NULL = tak terbatas
  score_policy            text NOT NULL DEFAULT 'highest' CHECK (score_policy IN ('highest','last','avg')),
  passing_score           numeric(6,2),
  show_score_immediately  boolean NOT NULL DEFAULT true,
  show_review             boolean NOT NULL DEFAULT false,
  show_explanations       boolean NOT NULL DEFAULT false,
  scores_released_at      timestamptz,
  shuffle_questions       boolean NOT NULL DEFAULT true,
  shuffle_options         boolean NOT NULL DEFAULT true,
  free_navigation         boolean NOT NULL DEFAULT true,
  require_all_answered    boolean NOT NULL DEFAULT false,
  single_session_policy   text NOT NULL DEFAULT 'replace_old' CHECK (single_session_policy IN ('reject_new','replace_old')),
  -- anti-cheat (FR-5.8A) — nilai default = tabel default PRD
  anticheat_enabled       boolean NOT NULL DEFAULT true,
  leave_tolerance_seconds integer NOT NULL DEFAULT 3 CHECK (leave_tolerance_seconds >= 0),
  penalty_mode            text NOT NULL DEFAULT 'points' CHECK (penalty_mode IN ('none','points','percent')),
  penalty_value           numeric(6,2) NOT NULL DEFAULT 5,
  penalty_max             numeric(6,2) NOT NULL DEFAULT 30,
  flag_threshold          integer DEFAULT 3 CHECK (flag_threshold IS NULL OR flag_threshold > 0),
  auto_submit_threshold   integer CHECK (auto_submit_threshold IS NULL OR auto_submit_threshold > 0),
  -- kamera (FR-5.8B)
  camera_enabled          boolean NOT NULL DEFAULT false,
  camera_deny_policy      text NOT NULL DEFAULT 'block' CHECK (camera_deny_policy IN ('block','allow_flag')),
  snapshot_interval_min_s integer NOT NULL DEFAULT 30,
  snapshot_interval_max_s integer NOT NULL DEFAULT 90,
  face_flag_penalty       boolean NOT NULL DEFAULT false,
  -- sumber kuota saat publish (FR 3.2)
  quota_source            text CHECK (quota_source IN ('trial','monthly','credit')),
  quota_ref_id            uuid,                         -- subscriptions.id atau exam_credits.id
  quota_refunded_at       timestamptz,
  published_at            timestamptz,
  created_by              uuid NOT NULL REFERENCES users(id),
  created_at              timestamptz NOT NULL DEFAULT now(),
  updated_at              timestamptz NOT NULL DEFAULT now(),
  CHECK (close_at IS NULL OR open_at IS NULL OR close_at > open_at),
  CHECK (snapshot_interval_max_s >= snapshot_interval_min_s),
  CHECK (status = 'draft' OR (open_at IS NOT NULL AND close_at IS NOT NULL AND duration_minutes IS NOT NULL AND set_version_id IS NOT NULL))
);
CREATE INDEX exams_tenant_status_idx ON exams (tenant_id, status, open_at);
CREATE INDEX exams_window_idx ON exams (tenant_id, close_at);       -- retensi: close_at ujian terakhir
CREATE INDEX exams_set_idx ON exams (question_set_id);

-- Ruang/sesi opsional dengan token sendiri (FR-4.9)
CREATE TABLE exam_rooms (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id  uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  exam_id    uuid NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
  name       text NOT NULL,
  UNIQUE (exam_id, name)
);

-- Token unik GLOBAL: login tanpa akun cukup kode ujian + token (tenant ditemukan dari token)
CREATE TABLE exam_tokens (
  token       text PRIMARY KEY CHECK (token ~ '^[A-Z0-9]{6,12}$'),
  tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  exam_id     uuid NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
  room_id     uuid REFERENCES exam_rooms(id) ON DELETE CASCADE,
  is_active   boolean NOT NULL DEFAULT true,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX exam_tokens_exam_idx ON exam_tokens (exam_id) WHERE is_active;

-- Penugasan: kelas / siswa (kombinasi diperbolehkan) — FR-4.2
CREATE TABLE exam_assignments (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  exam_id      uuid NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
  target_type  text NOT NULL CHECK (target_type IN ('class','student')),
  class_id     uuid REFERENCES classes(id) ON DELETE CASCADE,
  student_id   uuid REFERENCES students(id) ON DELETE CASCADE,
  CHECK ((target_type = 'class' AND class_id IS NOT NULL AND student_id IS NULL)
      OR (target_type = 'student' AND student_id IS NOT NULL AND class_id IS NULL))
);
CREATE UNIQUE INDEX exam_assign_class_uq ON exam_assignments (exam_id, class_id) WHERE class_id IS NOT NULL;
CREATE UNIQUE INDEX exam_assign_student_uq ON exam_assignments (exam_id, student_id) WHERE student_id IS NOT NULL;

-- Peserta hasil resolusi penugasan (kelas -> siswa) + penyesuaian per siswa
CREATE TABLE exam_participants (
  tenant_id            uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  exam_id              uuid NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
  student_id           uuid NOT NULL REFERENCES students(id) ON DELETE CASCADE,
  room_id              uuid REFERENCES exam_rooms(id) ON DELETE SET NULL,
  source               text NOT NULL DEFAULT 'assignment' CHECK (source IN ('assignment','manual','guest')),
  extra_time_seconds   integer NOT NULL DEFAULT 0,
  attempts_override    integer CHECK (attempts_override IS NULL OR attempts_override > 0),
  PRIMARY KEY (exam_id, student_id)
);
CREATE INDEX exam_participants_student_idx ON exam_participants (student_id);
