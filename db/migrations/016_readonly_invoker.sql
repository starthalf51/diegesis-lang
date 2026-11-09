-- 016: sl_ro privileges reorganized (final review C1). 015's LIKE 'graph\_%' loop also made the dynamic-SQL helper graph_rows
--   SECURITY DEFINER (owner sl = superuser) and granted sl_ro execute, so sl_ro could inject SQL with sl privileges through the q argument.
--   Rule: there is no SECURITY DEFINER for the dashboard. All functions run as INVOKER, and sl_ro execute is granted only to the names listed below.
SET LOCAL search_path = public, ag_catalog;

-- 1) Make every SECURITY DEFINER in public INVOKER (including metric_*, graph_*, lint_short that 015 changed).
DO $$ DECLARE f regprocedure; BEGIN
  FOR f IN SELECT p.oid::regprocedure FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
            WHERE n.nspname = 'public' AND p.prosecdef LOOP
    EXECUTE format('ALTER FUNCTION %s SECURITY INVOKER', f);
  END LOOP;
END $$;

-- 2) Execute privilege only for the name list. The rest (graph_rows, graph_name, derive and other write/dynamic-SQL functions) are not allowed for PUBLIC or sl_ro.
REVOKE EXECUTE ON ALL FUNCTIONS IN SCHEMA public FROM PUBLIC, sl_ro;
-- Functions sl creates later also come without PUBLIC execute. A per-schema REVOKE cannot undo the global default (PUBLIC EXECUTE), so apply it globally.
ALTER DEFAULT PRIVILEGES FOR ROLE sl REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC;
ALTER DEFAULT PRIVILEGES FOR ROLE sl IN SCHEMA public REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC;

-- The functions the dashboard panels (deploy/grafana/dashboards/diegesis.json) call + the read-only helpers they call in 011, 013 and 014.
--   Every helper is sql/plpgsql that only does SELECT, with no EXECUTE (dynamic SQL) or writes. Panels 9 and 10 read tables and views directly.
GRANT EXECUTE ON FUNCTION
  metric_ords(text), metric_presence(text), metric_asymmetry(text), metric_open_setups(text),
  metric_active_deceptions(text), metric_tension(text), metric_anachrony(text), metric_pace(text),
  graph_nodes(text), graph_edges(text), lint_short(text),
  -- Helpers (call graph: asymmetry → reader_knows·truth_at·char_knows·lint_exempt, deceptions → point_ord·lint_canon, edges → assertive)
  reader_knows(text, text), reader_events(text), truth_at(text, text, text), char_knows(text, text, text, boolean, text),
  lint_exempt(text, text), point_ord(text, text), lint_canon(text, text), lint_neg(text, text),
  lint_self_and_groups(text, text), precedes(text, text, text), precedes(text, text), prop_norm(text, text),
  deceptive_act(text, text, text), assertive(text)
TO sl_ro;

-- 3) Put pg_temp last in the metric functions' search_path (so temp objects cannot shadow public names).
ALTER FUNCTION metric_ords(text) SET search_path = public, ag_catalog, pg_temp;
ALTER FUNCTION metric_presence(text) SET search_path = public, ag_catalog, pg_temp;
ALTER FUNCTION metric_asymmetry(text) SET search_path = public, ag_catalog, pg_temp;
ALTER FUNCTION metric_open_setups(text) SET search_path = public, ag_catalog, pg_temp;
ALTER FUNCTION metric_active_deceptions(text) SET search_path = public, ag_catalog, pg_temp;
ALTER FUNCTION metric_tension(text) SET search_path = public, ag_catalog, pg_temp;
ALTER FUNCTION metric_anachrony(text) SET search_path = public, ag_catalog, pg_temp;
ALTER FUNCTION metric_pace(text) SET search_path = public, ag_catalog, pg_temp;
ALTER FUNCTION graph_nodes(text) SET search_path = public, ag_catalog, pg_temp;
ALTER FUNCTION graph_edges(text) SET search_path = public, ag_catalog, pg_temp;

-- 4) The dashboard does not read the AGE graph (graph_nodes and graph_edges look only at public tables). Revoke ag_catalog usage.
--   sl_ro also has no schema creation or temp tables (the PG15+ default, but made explicit; sl is a superuser so it is unaffected).
REVOKE USAGE ON SCHEMA ag_catalog FROM sl_ro;
REVOKE CREATE ON SCHEMA public FROM PUBLIC, sl_ro;
DO $$ BEGIN
  EXECUTE format('REVOKE TEMPORARY ON DATABASE %I FROM PUBLIC, sl_ro', current_database());
END $$;
