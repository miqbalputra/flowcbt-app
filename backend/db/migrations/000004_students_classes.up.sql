-- 000004: kelas, siswa (termasuk peserta tamu), riwayat kelas, sesi login
CREATE TABLE classes (
  id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id            uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name                 text NOT NULL,
  academic_year        text NOT NULL,
  homeroom_teacher_id  uuid REFERENCES users(id) ON DELETE SET NULL,
  archived_at          timestamptz,
  created_at           timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, name, academic_year)
);

-- Pembatasan akses guru pada kelas/mapel tertentu (FR-1.5)
CREATE TABLE teacher_class_assignments (
  tenant_id     uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  class_id      uuid NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  subject_name  text NOT NULL DEFAULT '',
  PRIMARY KEY (user_id, class_id, subject_name)
);
CREATE INDEX tca_class_idx ON teacher_class_assignments (class_id);

CREATE TABLE students (
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id          uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  student_code       text NOT NULL,                    -- identitas unik untuk track record
  nis                text,
  nisn               text,
  name               text NOT NULL,
  name_normalized    text NOT NULL,                    -- lower, trim, spasi ganda dihapus (pencocokan login tanpa akun)
  gender             text NOT NULL CHECK (gender IN ('L','P')),
  class_id           uuid REFERENCES classes(id) ON DELETE SET NULL,
  username           text,
  password_hash      text,
  email              citext,
  guardian_phone     text,
  photo_key          text,
  status             text NOT NULL DEFAULT 'active' CHECK (status IN ('active','inactive','archived')),
  is_guest           boolean NOT NULL DEFAULT false,
  merged_into_id     uuid REFERENCES students(id),     -- hasil merge peserta tamu (FR-1.3)
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, student_code),
  CHECK (merged_into_id IS NULL OR is_guest)
);
CREATE UNIQUE INDEX students_username_uq ON students (tenant_id, username) WHERE username IS NOT NULL;
CREATE UNIQUE INDEX students_nis_uq ON students (tenant_id, nis) WHERE nis IS NOT NULL AND NOT is_guest;
CREATE INDEX students_class_idx ON students (tenant_id, class_id) WHERE status = 'active';
CREATE INDEX students_match_idx ON students (tenant_id, name_normalized, class_id);
CREATE INDEX students_name_trgm ON students USING gin (name gin_trgm_ops);
CREATE INDEX students_merged_idx ON students (merged_into_id) WHERE merged_into_id IS NOT NULL;

CREATE TABLE student_class_history (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  student_id  uuid NOT NULL REFERENCES students(id) ON DELETE CASCADE,
  class_id    uuid NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  from_date   date NOT NULL,
  to_date     date,
  CHECK (to_date IS NULL OR to_date >= from_date)
);
CREATE INDEX sch_student_idx ON student_class_history (student_id, from_date DESC);

-- Refresh token (rotasi) untuk user & siswa (FR-1.6)
CREATE TABLE refresh_tokens (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id     uuid REFERENCES tenants(id) ON DELETE CASCADE,
  subject_type  text NOT NULL CHECK (subject_type IN ('user','student')),
  user_id       uuid REFERENCES users(id) ON DELETE CASCADE,
  student_id    uuid REFERENCES students(id) ON DELETE CASCADE,
  family_id     uuid NOT NULL,                       -- deteksi reuse token lama => cabut satu family
  token_hash    text NOT NULL UNIQUE,
  device_hash   text,                                -- sesi tamu diikat ke perangkat (cookie httpOnly)
  ip            inet,
  user_agent    text,
  expires_at    timestamptz NOT NULL,
  revoked_at    timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now(),
  CHECK ((subject_type = 'user' AND user_id IS NOT NULL) OR (subject_type = 'student' AND student_id IS NOT NULL))
);
CREATE INDEX refresh_tokens_family_idx ON refresh_tokens (family_id);
CREATE INDEX refresh_tokens_user_idx ON refresh_tokens (user_id) WHERE user_id IS NOT NULL;
CREATE INDEX refresh_tokens_student_idx ON refresh_tokens (student_id) WHERE student_id IS NOT NULL;
