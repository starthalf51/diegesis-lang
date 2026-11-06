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

-- 읽기만: 테이블·뷰 SELECT, 함수 EXECUTE(metric_*·graph_* 와 그 안에서 부르는 헬퍼). 쓰기·DDL 권한은 없다.
GRANT USAGE ON SCHEMA public, ag_catalog TO sl_ro;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO sl_ro;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO sl_ro;
