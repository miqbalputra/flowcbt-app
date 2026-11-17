-- 000007: pengerjaan (attempt), jawaban, event sinkron, pelanggaran, snapshot kamera
CREATE TABLE attempts (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id           uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  exam_id             uuid NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
  set_version_id      uuid NOT NULL REFERENCES question_set_versions(id),
  student_id          uuid NOT NULL REFERENCES students(id),
  attempt_no          integer NOT NULL CHECK (attempt_no > 0),
  class_snapshot      text,                                  -- nama kelas saat mengerjakan (FR-3.5)
  -- state machine (PRD 18.3); 'not_started' = belum ada baris
  status              text NOT NULL DEFAULT 'in_progress'
                      CHECK (status IN ('in_progress','pending_sync','submitted','awaiting_manual_grading','graded','expired','cancelled')),
  submit_reason       text CHECK (submit_reason IN ('manual','timeout','violation','force','offline_sync')),
  started_at          timestamptz NOT NULL DEFAULT now(),
  deadline_at         timestamptz NOT NULL,                  -- otoritas waktu di server
  submitted_at        timestamptz,
  -- nilai (FR-8.2): final = raw - penalti
  raw_score           numeric(8,2),
  max_score           numeric(8,2),
  penalty_points      numeric(8,2) NOT NULL DEFAULT 0,
  final_score         numeric(8,2),
  score_provisional   boolean NOT NULL DEFAULT false,        -- ada uraian belum dinilai
  passed              boolean,
  -- anti-cheat (FR-5.8C)
  cheating_status     text NOT NULL DEFAULT 'none'
                      CHECK (cheating_status IN ('none','warned','flagged','valid','penalty_cancelled','disqualified')),
  cheating_decided_by uuid REFERENCES users(id),
  cheating_decided_at timestamptz,
  cheating_reason     text,
  violation_count     integer NOT NULL DEFAULT 0,
  -- pengerjaan & pemantauan
  shuffle_seed        bigint NOT NULL,                       -- urutan soal/opsi deterministik per attempt
  extra_time_seconds  integer NOT NULL DEFAULT 0,
  answered_count      integer NOT NULL DEFAULT 0,
  last_seq            bigint NOT NULL DEFAULT 0,
  last_seen_at        timestamptz,
  device_info         jsonb NOT NULL DEFAULT '{}',
  consent_camera_at   timestamptz,
  consent_rules_at    timestamptz,
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  UNIQUE (exam_id, student_id, attempt_no)
);
CREATE INDEX attempts_exam_status_idx ON attempts (exam_id, status);
CREATE INDEX attempts_student_idx ON attempts (student_id, started_at DESC);
CREATE INDEX attempts_tenant_exam_idx ON attempts (tenant_id, exam_id);
CREATE INDEX attempts_grading_idx ON attempts (exam_id) WHERE status = 'awaiting_manual_grading';
CREATE INDEX attempts_flagged_idx ON attempts (exam_id) WHERE cheating_status IN ('flagged','warned');
CREATE INDEX attempts_deadline_idx ON attempts (deadline_at) WHERE status = 'in_progress';   -- worker auto-submit

-- Satu sesi aktif per siswa per ujian (FR-1.6)
CREATE TABLE attempt_sessions (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  attempt_id   uuid NOT NULL REFERENCES attempts(id) ON DELETE CASCADE,
  device_hash  text NOT NULL,
  started_at   timestamptz NOT NULL DEFAULT now(),
  revoked_at   timestamptz,
  revoked_by   text CHECK (revoked_by IN ('new_login','teacher','submit'))
);
CREATE UNIQUE INDEX attempt_sessions_one_active_uq ON attempt_sessions (attempt_id) WHERE revoked_at IS NULL;

-- Proyeksi jawaban terakhir per soal (hasil reduksi attempt_events, last-write-wins by seq)
CREATE TABLE attempt_answers (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id        uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  attempt_id       uuid NOT NULL REFERENCES attempts(id) ON DELETE CASCADE,
  question_id      uuid NOT NULL REFERENCES questions(id),
  answer_json      jsonb,
  flagged          boolean NOT NULL DEFAULT false,           -- ragu-ragu
  is_correct       boolean,
  auto_score       numeric(6,2),
  score            numeric(6,2),                             -- skor akhir soal (auto atau override/manual)
  graded_by        text CHECK (graded_by IN ('auto','manual')),
  grader_id        uuid REFERENCES users(id),
  grader_comment   text,
  override_reason  text,
  time_spent_ms    integer NOT NULL DEFAULT 0,
  seq              bigint NOT NULL DEFAULT 0,
  updated_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (attempt_id, question_id)
);
CREATE INDEX attempt_answers_question_idx ON attempt_answers (question_id);   -- analisis butir soal

-- Event append-only; idempoten via (attempt_id, client_event_id).
-- CATATAN: PRD menyebut partisi per bulan, tetapi UNIQUE lintas-partisi mewajibkan kolom partisi
-- ikut di indeks unik. Dipilih HASH(attempt_id) agar idempotensi dijamin DB; retensi tetap via cascade attempts.
CREATE TABLE attempt_events (
  id               bigint GENERATED ALWAYS AS IDENTITY,
  tenant_id        uuid NOT NULL,
  attempt_id       uuid NOT NULL REFERENCES attempts(id) ON DELETE CASCADE,
  client_event_id  uuid NOT NULL,
  type             text NOT NULL CHECK (type IN ('answer','flag','nav','submit','focus_lost','online','offline','sync','violation')),
  payload          jsonb NOT NULL DEFAULT '{}',
  client_ts        timestamptz,
  elapsed_ms       bigint,                                   -- jam monotonic klien sejak mulai (FR-7.5)
  server_ts        timestamptz NOT NULL DEFAULT now(),
  seq              bigint NOT NULL,
  accepted         boolean NOT NULL DEFAULT true,            -- false = lewat deadline + toleransi (AC-5)
  reject_reason    text,
  PRIMARY KEY (attempt_id, id),
  UNIQUE (attempt_id, client_event_id)
) PARTITION BY HASH (attempt_id);

DO $$ BEGIN
  FOR i IN 0..15 LOOP
    EXECUTE format('CREATE TABLE attempt_events_p%s PARTITION OF attempt_events FOR VALUES WITH (MODULUS 16, REMAINDER %s)', i, i);
  END LOOP;
END $$;
CREATE INDEX attempt_events_seq_idx ON attempt_events (attempt_id, seq);
CREATE INDEX attempt_events_rejected_idx ON attempt_events (attempt_id) WHERE NOT accepted;

CREATE TABLE attempt_violations (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id        uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  attempt_id       uuid NOT NULL REFERENCES attempts(id) ON DELETE CASCADE,
  client_event_id  uuid NOT NULL,
  type             text NOT NULL CHECK (type IN ('tab_hidden','blur','fullscreen_exit','app_background','camera_off','no_face','multi_face')),
  started_at       timestamptz NOT NULL,
  duration_ms      integer NOT NULL DEFAULT 0,
  counted          boolean NOT NULL DEFAULT false,           -- melewati toleransi
  penalty_applied  numeric(6,2) NOT NULL DEFAULT 0,
  waived_by        uuid REFERENCES users(id),
  waived_reason    text,
  created_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (attempt_id, client_event_id)
);
CREATE INDEX attempt_violations_attempt_idx ON attempt_violations (attempt_id, started_at);

CREATE TABLE proctor_snapshots (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id     uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  attempt_id    uuid NOT NULL REFERENCES attempts(id) ON DELETE CASCADE,
  object_key    text NOT NULL UNIQUE,
  trigger       text NOT NULL DEFAULT 'periodic' CHECK (trigger IN ('periodic','violation')),
  taken_at      timestamptz NOT NULL,
  size_bytes    integer NOT NULL DEFAULT 0,
  flags_json    jsonb NOT NULL DEFAULT '{}',                 -- {no_face:true, multi_face:true, camera_off:true}
  expires_at    timestamptz NOT NULL,                        -- 30 hari setelah ujian selesai / retensi tenant
  deleted_at    timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX proctor_snapshots_attempt_idx ON proctor_snapshots (attempt_id, taken_at);
CREATE INDEX proctor_snapshots_expiry_idx ON proctor_snapshots (expires_at) WHERE deleted_at IS NULL;
