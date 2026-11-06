-- 015: Grafana 용 읽기 전용 역할 sl_ro(플랜 2 Task 9). 마이그레이션 러너는 환경변수를 못 받으므로 비밀번호는 여기 두지 않는다.
--   역할은 NOLOGIN 으로 만들고, 로그인 비밀번호는 deploy/compose.yaml 의 ro-init 서비스가 ALTER ROLE 로 건다.
--   역할은 클러스터 전역이라 이미 있으면 건너뛴다(테스트가 DB 를 새로 만들어 이 파일을 다시 적용한다).
SET LOCAL search_path = public, ag_catalog;

DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'sl_ro') THEN
    CREATE ROLE sl_ro NOLOGIN;
  END IF;
  EXECUTE format('GRANT CONNECT ON DATABASE %I TO sl_ro', current_database());
END $$;

-- 테이블·뷰는 SELECT 만(패널 9·10 이 pattern_result·diagnostic 을 직접 읽는다). 앞으로 sl 이 만드는 테이블도 읽는다.
GRANT USAGE ON SCHEMA public, ag_catalog TO sl_ro;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO sl_ro;
ALTER DEFAULT PRIVILEGES FOR ROLE sl IN SCHEMA public GRANT SELECT ON TABLES TO sl_ro;

-- 함수는 metric_*·graph_* 와 표시용 lint_short 만 실행한다. 나머지 헬퍼(derive 등 쓰기 함수 포함)는 PUBLIC·sl_ro 모두 실행 불가.
-- 지표 함수는 SECURITY DEFINER(소유자 sl 권한, search_path 는 각 함수에 고정돼 있다)로 바꿔 안에서 부르는 헬퍼에 sl_ro 권한이 필요 없게 한다.
-- ponytail: PUBLIC 실행권을 거두므로 sl 외 역할이 헬퍼를 부르려면 별도 GRANT 가 필요하다(지금은 sl 하나뿐).
REVOKE EXECUTE ON ALL FUNCTIONS IN SCHEMA public FROM PUBLIC, sl_ro;
DO $$ DECLARE f regprocedure; BEGIN
  FOR f IN SELECT p.oid::regprocedure FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
            WHERE n.nspname = 'public' AND p.prokind = 'f' AND (p.proname LIKE 'metric\_%' OR p.proname LIKE 'graph\_%' OR p.proname = 'lint_short') LOOP
    EXECUTE format('ALTER FUNCTION %s SECURITY DEFINER', f);
    EXECUTE format('GRANT EXECUTE ON FUNCTION %s TO sl_ro', f);
  END LOOP;
END $$;
