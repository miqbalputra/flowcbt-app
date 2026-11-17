DO $$ DECLARE t text; BEGIN
  FOR t IN SELECT c.table_name FROM information_schema.columns c
           JOIN information_schema.tables tb ON tb.table_schema = c.table_schema AND tb.table_name = c.table_name
           WHERE c.table_schema = 'public' AND c.column_name = 'tenant_id' AND tb.table_type = 'BASE TABLE'
  LOOP
    EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON %I', t);
    EXECUTE format('ALTER TABLE %I DISABLE ROW LEVEL SECURITY', t);
  END LOOP;
END $$;
DROP POLICY IF EXISTS tenant_self ON tenants;
ALTER TABLE tenants DISABLE ROW LEVEL SECURITY;
DROP ROLE IF EXISTS flowcbt_app;
DROP ROLE IF EXISTS flowcbt_system;
