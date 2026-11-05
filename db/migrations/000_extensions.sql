CREATE EXTENSION IF NOT EXISTS age;
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS btree_gist;
-- 새 세션마다 age 를 자동 로드하고 ag_catalog 를 검색 경로에 둔다.
ALTER DATABASE sl SET session_preload_libraries = 'age';
ALTER DATABASE sl SET search_path = ag_catalog, "$user", public;
