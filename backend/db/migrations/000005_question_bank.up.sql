-- 000005: media, paket soal, versi, stimulus, soal
CREATE TABLE media_assets (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  kind         text NOT NULL CHECK (kind IN ('image','audio','import','export')),
  object_key   text NOT NULL UNIQUE,
  mime         text NOT NULL,
  size_bytes   bigint NOT NULL CHECK (size_bytes >= 0),
  created_by   uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX media_assets_tenant_idx ON media_assets (tenant_id, kind);

CREATE TABLE question_sets (
  id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id             uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  subject_name          text NOT NULL,
  title                 text NOT NULL,
  description           text,
  exam_code             text NOT NULL,
  level_tags            text[] NOT NULL DEFAULT '{}',
  status                text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published','archived')),
  draft_version_id      uuid,                      -- FK ditambah di bawah (sirkular)
  published_version_id  uuid,
  shared_with_tenant    boolean NOT NULL DEFAULT false,   -- dibagikan ke semua guru di tenant (matriks akses)
  created_by            uuid NOT NULL REFERENCES users(id),
  archived_at           timestamptz,
  created_at            timestamptz NOT NULL DEFAULT now(),
  updated_at            timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, exam_code)
);
CREATE INDEX question_sets_list_idx ON question_sets (tenant_id, status, updated_at DESC);
CREATE INDEX question_sets_title_trgm ON question_sets USING gin ((title || ' ' || subject_name) gin_trgm_ops);

CREATE TABLE question_set_shares (
  tenant_id        uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  question_set_id  uuid NOT NULL REFERENCES question_sets(id) ON DELETE CASCADE,
  user_id          uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY (question_set_id, user_id)
);

-- Versi: satu 'draft' (dapat diubah) + n 'published' (beku). Publish membekukan draft;
-- mengubah soal setelah publish = clone ke draft baru (FR-4.8).
CREATE TABLE question_set_versions (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id        uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  question_set_id  uuid NOT NULL REFERENCES question_sets(id) ON DELETE CASCADE,
  version_no       integer NOT NULL,
  status           text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published')),
  question_count   integer NOT NULL DEFAULT 0,
  max_points       numeric(8,2) NOT NULL DEFAULT 0,
  published_at     timestamptz,
  published_by     uuid REFERENCES users(id),
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (question_set_id, version_no)
);
CREATE UNIQUE INDEX qsv_one_draft_uq ON question_set_versions (question_set_id) WHERE status = 'draft';

ALTER TABLE question_sets
  ADD CONSTRAINT qs_draft_fk     FOREIGN KEY (draft_version_id)     REFERENCES question_set_versions(id) DEFERRABLE INITIALLY DEFERRED,
  ADD CONSTRAINT qs_published_fk FOREIGN KEY (published_version_id) REFERENCES question_set_versions(id) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE stimuli (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id       uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  set_version_id  uuid NOT NULL REFERENCES question_set_versions(id) ON DELETE CASCADE,
  order_no        integer NOT NULL DEFAULT 0,
  title           text,
  content_html    text NOT NULL,              -- sudah disanitasi (allowlist) di server
  media_ids       uuid[] NOT NULL DEFAULT '{}'
);
CREATE INDEX stimuli_version_idx ON stimuli (set_version_id, order_no);

CREATE TABLE questions (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id           uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  set_version_id      uuid NOT NULL REFERENCES question_set_versions(id) ON DELETE CASCADE,
  order_no            integer NOT NULL,
  type                text NOT NULL CHECK (type IN ('mcq','mcq_complex','matching','short_answer','essay')),
  stimulus_id         uuid REFERENCES stimuli(id) ON DELETE SET NULL,
  body_html           text NOT NULL,
  options_json        jsonb NOT NULL DEFAULT '[]',   -- mcq: [{id,html}] ; matching: {left:[..],right:[..]}
  answer_key_json     jsonb,                         -- TIDAK pernah dikirim ke klien siswa
  rubric_json         jsonb,                         -- uraian
  points              numeric(6,2) NOT NULL CHECK (points >= 0),
  scoring_mode        text NOT NULL DEFAULT 'all_or_nothing' CHECK (scoring_mode IN ('all_or_nothing','partial')),
  partial_wrong_penalty boolean NOT NULL DEFAULT true,   -- FR-8.1b (benar - salah)
  shuffle_options     boolean NOT NULL DEFAULT true,
  explanation_html    text,
  media_ids           uuid[] NOT NULL DEFAULT '{}',
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT questions_order_uq UNIQUE (set_version_id, order_no) DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX questions_stimulus_idx ON questions (stimulus_id) WHERE stimulus_id IS NOT NULL;
