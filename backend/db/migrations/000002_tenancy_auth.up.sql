-- 000002: tenant, user (admin/guru/super admin), token sekali pakai
CREATE TABLE tenants (
  id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code                  text NOT NULL UNIQUE,                 -- kode sekolah (dipakai login siswa untuk menentukan tenant)
  name                  text NOT NULL,
  level                 text NOT NULL CHECK (level IN ('paud_tk','sd_mi','smp_mts','sma_smk_ma','perguruan_tinggi','kursus_bimbel','lainnya')),
  timezone              text NOT NULL DEFAULT 'Asia/Jakarta',
  phone                 text,
  status                text NOT NULL DEFAULT 'active' CHECK (status IN ('active','read_only','suspended','purged')),
  score_decimals        smallint NOT NULL DEFAULT 2 CHECK (score_decimals BETWEEN 0 AND 4),
  storage_quota_bytes   bigint NOT NULL DEFAULT 1073741824,   -- 1 GB (FR 3.2), bisa diubah Super Admin
  storage_used_bytes    bigint NOT NULL DEFAULT 0,
  -- siklus akses & retensi 7 hari (PRD 3.2)
  free_plan_used        boolean NOT NULL DEFAULT false,       -- paket gratis sekali per tenant
  access_ended_at       timestamptz,                          -- waktu hak akses berakhir (trial/bulanan/kredit habis)
  read_only_since       timestamptz,
  purge_after           timestamptz,                          -- = greatest(access_ended_at, close_at ujian terakhir) + 7 hari
  purged_at             timestamptz,
  -- anti-penyalahgunaan paket gratis (tetap disimpan setelah purge, tanpa data siswa)
  name_normalized       text,
  phone_normalized      text,
  owner_email_hash      text,
  created_at            timestamptz NOT NULL DEFAULT now(),
  updated_at            timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX tenants_status_purge_idx ON tenants (status, purge_after);
CREATE INDEX tenants_dup_name_idx  ON tenants (name_normalized);
CREATE INDEX tenants_dup_phone_idx ON tenants (phone_normalized) WHERE phone_normalized IS NOT NULL;

CREATE TABLE users (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id           uuid REFERENCES tenants(id) ON DELETE CASCADE,   -- NULL hanya untuk super_admin
  role                text NOT NULL CHECK (role IN ('super_admin','owner','teacher')),
  name                text NOT NULL,
  email               citext NOT NULL UNIQUE,
  phone               text,
  password_hash       text,                                            -- NULL bila hanya login Google
  google_sub          text UNIQUE,
  email_verified_at   timestamptz,
  status              text NOT NULL DEFAULT 'active' CHECK (status IN ('invited','active','disabled')),
  failed_login_count  integer NOT NULL DEFAULT 0,
  locked_until        timestamptz,
  last_login_at       timestamptz,
  theme_pref          text CHECK (theme_pref IN ('light','dark','system')),
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  CHECK ((role = 'super_admin') = (tenant_id IS NULL))
);
CREATE INDEX users_tenant_idx ON users (tenant_id, role);

-- Token sekali pakai: verifikasi email, reset password, undangan guru
CREATE TABLE one_time_tokens (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   uuid REFERENCES tenants(id) ON DELETE CASCADE,
  user_id     uuid REFERENCES users(id) ON DELETE CASCADE,
  email       citext,
  purpose     text NOT NULL CHECK (purpose IN ('verify_email','reset_password','teacher_invite')),
  token_hash  text NOT NULL UNIQUE,
  payload     jsonb NOT NULL DEFAULT '{}',          -- mis. {"role":"teacher","class_ids":[...]}
  expires_at  timestamptz NOT NULL,
  used_at     timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX one_time_tokens_user_idx ON one_time_tokens (user_id, purpose);
