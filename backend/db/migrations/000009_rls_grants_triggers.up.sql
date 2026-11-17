-- 000009: role DB, Row-Level Security multi-tenant, trigger updated_at
-- Role login dibuat ops: ALTER ROLE flowcbt_app LOGIN PASSWORD '...'; ALTER ROLE flowcbt_system LOGIN PASSWORD '...';
--  * flowcbt_app    : dipakai API; tunduk RLS (SET LOCAL app.tenant_id per transaksi)
--  * flowcbt_system : BYPASSRLS; dipakai login (cari user/siswa lintas tenant), webhook, worker retensi, Super Admin
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'flowcbt_app')    THEN CREATE ROLE flowcbt_app NOLOGIN; END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'flowcbt_system') THEN CREATE ROLE flowcbt_system NOLOGIN BYPASSRLS; END IF;
END $$;

-- trigger updated_at untuk semua tabel yang punya kolom updated_at
DO $$ DECLARE t text; BEGIN
  FOR t IN SELECT c.table_name FROM information_schema.columns c
           JOIN information_schema.tables tb ON tb.table_schema = c.table_schema AND tb.table_name = c.table_name
           WHERE c.table_schema = 'public' AND c.column_name = 'updated_at' AND tb.table_type = 'BASE TABLE'
  LOOP
    EXECUTE format('CREATE TRIGGER trg_%s_updated BEFORE UPDATE ON %I FOR EACH ROW EXECUTE FUNCTION set_updated_at()', t, t);
  END LOOP;
END $$;

-- RLS: semua tabel ber-kolom tenant_id
DO $$ DECLARE t text; BEGIN
  FOR t IN SELECT c.table_name FROM information_schema.columns c
           JOIN information_schema.tables tb ON tb.table_schema = c.table_schema AND tb.table_name = c.table_name
           WHERE c.table_schema = 'public' AND c.column_name = 'tenant_id' AND tb.table_type = 'BASE TABLE'
  LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id = current_tenant_id()) WITH CHECK (tenant_id = current_tenant_id())', t);
  END LOOP;
END $$;

ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_self ON tenants USING (id = current_tenant_id()) WITH CHECK (id = current_tenant_id());

-- Hak akses role aplikasi
GRANT USAGE ON SCHEMA public TO flowcbt_app, flowcbt_system;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO flowcbt_app, flowcbt_system;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO flowcbt_app, flowcbt_system;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO flowcbt_app, flowcbt_system;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO flowcbt_app, flowcbt_system;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO flowcbt_app, flowcbt_system;

-- Tabel lintas-tenant: aplikasi hanya membaca paket; webhook hanya role sistem
REVOKE INSERT, UPDATE, DELETE ON plans FROM flowcbt_app;
REVOKE ALL ON payment_webhook_events, platform_settings FROM flowcbt_app;
-- Append-only: log audit & event jawaban tidak boleh diubah/dihapus oleh API
REVOKE UPDATE, DELETE ON audit_logs FROM flowcbt_app;
REVOKE UPDATE, DELETE ON attempt_events FROM flowcbt_app;
-- Fungsi purge hanya role sistem
REVOKE EXECUTE ON FUNCTION purge_tenant_data(uuid), tenant_object_keys(uuid) FROM flowcbt_app, PUBLIC;
GRANT  EXECUTE ON FUNCTION purge_tenant_data(uuid), tenant_object_keys(uuid) TO flowcbt_system;
