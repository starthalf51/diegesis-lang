-- 015: read-only role sl_ro for Grafana (Plan 2 Task 9). The migration runner cannot take environment variables, so no password is kept here.
--   The role is created NOLOGIN, and the login password is set by the ro-init service in deploy/compose.yaml via ALTER ROLE.
--   Roles are cluster-wide, so skip if it already exists (tests recreate the DB and reapply this file).
SET LOCAL search_path = public, ag_catalog;

DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'sl_ro') THEN
    CREATE ROLE sl_ro NOLOGIN;
  END IF;
  EXECUTE format('GRANT CONNECT ON DATABASE %I TO sl_ro', current_database());
END $$;

-- SELECT only on tables and views (panels 9 and 10 read pattern_result and diagnostic directly). Tables sl creates later are readable too.
GRANT USAGE ON SCHEMA public, ag_catalog TO sl_ro;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO sl_ro;
ALTER DEFAULT PRIVILEGES FOR ROLE sl IN SCHEMA public GRANT SELECT ON TABLES TO sl_ro;

-- Functions: only metric_*, graph_* and the display helper lint_short are executable. The other helpers (including write functions like derive) are not executable by PUBLIC or sl_ro.
-- The metric functions are made SECURITY DEFINER (owner sl privileges; search_path is pinned in each function) so sl_ro needs no privilege on the helpers they call.
-- ponytail: PUBLIC execute is revoked, so a role other than sl needs a separate GRANT to call the helpers (there is only sl for now).
REVOKE EXECUTE ON ALL FUNCTIONS IN SCHEMA public FROM PUBLIC, sl_ro;
DO $$ DECLARE f regprocedure; BEGIN
  FOR f IN SELECT p.oid::regprocedure FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
            WHERE n.nspname = 'public' AND p.prokind = 'f' AND (p.proname LIKE 'metric\_%' OR p.proname LIKE 'graph\_%' OR p.proname = 'lint_short') LOOP
    EXECUTE format('ALTER FUNCTION %s SECURITY DEFINER', f);
    EXECUTE format('GRANT EXECUTE ON FUNCTION %s TO sl_ro', f);
  END LOOP;
END $$;
