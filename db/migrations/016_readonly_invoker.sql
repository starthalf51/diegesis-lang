-- 016: sl_ro 권한 재정리(최종 리뷰 C1). 015 의 LIKE 'graph\_%' 루프가 동적 SQL 헬퍼 graph_rows 까지
--   SECURITY DEFINER(소유자 sl = 슈퍼유저)로 만들고 sl_ro 에 실행권을 줘서, sl_ro 가 q 인자로 sl 권한 SQL 을 주입할 수 있었다.
--   규칙: 대시보드용 SECURITY DEFINER 는 없다. 함수는 전부 INVOKER 로 돌리고, sl_ro 실행권은 아래 이름 목록에만 준다.
SET LOCAL search_path = public, ag_catalog;

-- 1) public 의 SECURITY DEFINER 를 전부 INVOKER 로(015 가 바꾼 metric_*·graph_*·lint_short 포함).
DO $$ DECLARE f regprocedure; BEGIN
  FOR f IN SELECT p.oid::regprocedure FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
            WHERE n.nspname = 'public' AND p.prosecdef LOOP
    EXECUTE format('ALTER FUNCTION %s SECURITY INVOKER', f);
  END LOOP;
END $$;

-- 2) 실행권은 이름 목록에만. 나머지(graph_rows·graph_name·derive 등 쓰기·동적 SQL 함수)는 PUBLIC·sl_ro 모두 불가.
REVOKE EXECUTE ON ALL FUNCTIONS IN SCHEMA public FROM PUBLIC, sl_ro;
-- 앞으로 sl 이 만드는 함수도 PUBLIC 실행권 없이 생긴다. 스키마별 REVOKE 는 전역 기본값(PUBLIC EXECUTE)을 못 지우므로 전역으로 건다.
ALTER DEFAULT PRIVILEGES FOR ROLE sl REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC;
ALTER DEFAULT PRIVILEGES FOR ROLE sl IN SCHEMA public REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC;

-- 대시보드 패널(deploy/grafana/dashboards/diegesis.json)이 부르는 함수 + 그 함수들이 011·013·014 에서 부르는 읽기 전용 헬퍼.
--   헬퍼는 전부 SELECT 만 하는 sql/plpgsql 이고 EXECUTE(동적 SQL)·쓰기가 없다. 패널 9·10 은 테이블·뷰를 직접 읽는다.
GRANT EXECUTE ON FUNCTION
  metric_ords(text), metric_presence(text), metric_asymmetry(text), metric_open_setups(text),
  metric_active_deceptions(text), metric_tension(text), metric_anachrony(text), metric_pace(text),
  graph_nodes(text), graph_edges(text), lint_short(text),
  -- 헬퍼(호출 그래프: asymmetry → reader_knows·truth_at·char_knows·lint_exempt, deceptions → point_ord·lint_canon, edges → assertive)
  reader_knows(text, text), reader_events(text), truth_at(text, text, text), char_knows(text, text, text, boolean, text),
  lint_exempt(text, text), point_ord(text, text), lint_canon(text, text), lint_neg(text, text),
  lint_self_and_groups(text, text), precedes(text, text, text), precedes(text, text), prop_norm(text, text),
  deceptive_act(text, text, text), assertive(text)
TO sl_ro;

-- 3) 지표 함수의 search_path 는 pg_temp 를 마지막에 둔다(임시 객체가 public 이름을 가리지 못하게).
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

-- 4) 대시보드는 AGE 그래프를 읽지 않는다(graph_nodes·graph_edges 는 public 테이블만 본다). ag_catalog 사용권은 거둔다.
--   스키마 생성·임시 테이블도 sl_ro 에 없다(PG15+ 기본값이지만 명시한다; sl 은 슈퍼유저라 영향 없다).
REVOKE USAGE ON SCHEMA ag_catalog FROM sl_ro;
REVOKE CREATE ON SCHEMA public FROM PUBLIC, sl_ro;
DO $$ BEGIN
  EXECUTE format('REVOKE TEMPORARY ON DATABASE %I FROM PUBLIC, sl_ro', current_database());
END $$;
