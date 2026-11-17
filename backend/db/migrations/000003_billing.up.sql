-- 000003: paket, langganan, kredit ujian, pembayaran, invoice
CREATE TABLE plans (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code           text NOT NULL UNIQUE,
  name           text NOT NULL,
  type           text NOT NULL CHECK (type IN ('free','per_exam','monthly')),
  price_idr      integer NOT NULL CHECK (price_idr >= 0),
  student_min    integer NOT NULL CHECK (student_min >= 1),
  student_max    integer NOT NULL,
  exam_quota     integer,                 -- free & per_exam; NULL = tanpa batas (monthly)
  duration_days  integer,                 -- free = 14, monthly = 30
  limits_json    jsonb NOT NULL DEFAULT '{}',
  is_active      boolean NOT NULL DEFAULT true,
  sort_order     integer NOT NULL DEFAULT 0,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  CHECK (student_max >= student_min)
);

CREATE TABLE payments (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id        uuid REFERENCES tenants(id) ON DELETE SET NULL,   -- data penagihan disimpan walau tenant dihapus
  tenant_name_snap text NOT NULL,
  plan_id          uuid NOT NULL REFERENCES plans(id),
  purpose          text NOT NULL CHECK (purpose IN ('per_exam','monthly','upgrade')),
  provider         text NOT NULL CHECK (provider IN ('midtrans','mayar')),
  order_id         text NOT NULL UNIQUE,
  provider_ref     text,
  idempotency_key  text,
  amount_idr       integer NOT NULL CHECK (amount_idr >= 0),
  status           text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','paid','failed','expired','refunded')),
  checkout_url     text,
  paid_at          timestamptz,
  meta             jsonb NOT NULL DEFAULT '{}',   -- mis. prorata upgrade, subscription_id target
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX payments_provider_ref_uq ON payments (provider, provider_ref) WHERE provider_ref IS NOT NULL;
CREATE UNIQUE INDEX payments_idem_uq ON payments (tenant_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX payments_tenant_idx ON payments (tenant_id, created_at DESC);
CREATE INDEX payments_pending_idx ON payments (status, created_at) WHERE status = 'pending';  -- rekonsiliasi harian

-- Log webhook mentah; unik per event => idempoten. Tanpa tenant_id (hanya diakses role sistem)
CREATE TABLE payment_webhook_events (
  id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  provider       text NOT NULL,
  event_id       text NOT NULL,
  signature_ok   boolean NOT NULL,
  payload        jsonb NOT NULL,
  received_at    timestamptz NOT NULL DEFAULT now(),
  processed_at   timestamptz,
  error          text,
  UNIQUE (provider, event_id)
);

-- Langganan: trial (paket gratis) & bulanan. Pembelian per ujian => exam_credits.
CREATE TABLE subscriptions (
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id          uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  plan_id            uuid NOT NULL REFERENCES plans(id),
  type               text NOT NULL CHECK (type IN ('trial','monthly')),
  status             text NOT NULL CHECK (status IN ('pending','active','scheduled','expired','cancelled')),
  student_limit      integer NOT NULL,                  -- snapshot batas tier saat dibeli
  exam_quota         integer,                           -- trial: 1
  exams_used         integer NOT NULL DEFAULT 0,
  starts_at          timestamptz NOT NULL,
  ends_at            timestamptz NOT NULL,
  payment_id         uuid REFERENCES payments(id),
  replaces_id        uuid REFERENCES subscriptions(id), -- upgrade/downgrade (downgrade = status 'scheduled')
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX subscriptions_one_active_uq ON subscriptions (tenant_id) WHERE status = 'active';
CREATE INDEX subscriptions_tenant_idx ON subscriptions (tenant_id, ends_at DESC);
CREATE INDEX subscriptions_expiry_idx ON subscriptions (ends_at) WHERE status = 'active';

-- Kredit per ujian: tidak kedaluwarsa (Keputusan #12)
CREATE TABLE exam_credits (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  plan_id        uuid NOT NULL REFERENCES plans(id),
  payment_id     uuid REFERENCES payments(id),
  quantity       integer NOT NULL CHECK (quantity > 0),
  used           integer NOT NULL DEFAULT 0,
  student_limit  integer NOT NULL,
  created_at     timestamptz NOT NULL DEFAULT now(),
  CHECK (used >= 0 AND used <= quantity)
);
CREATE INDEX exam_credits_tenant_idx ON exam_credits (tenant_id) WHERE used < quantity;

CREATE TABLE invoices (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id        uuid REFERENCES tenants(id) ON DELETE SET NULL,
  payment_id       uuid NOT NULL UNIQUE REFERENCES payments(id),
  invoice_no       text NOT NULL UNIQUE,
  billing_name     text NOT NULL,
  billing_email    citext,
  amount_idr       integer NOT NULL,
  pdf_object_key   text,
  issued_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX invoices_tenant_idx ON invoices (tenant_id, issued_at DESC);
