CREATE EXTENSION IF NOT EXISTS age;
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS btree_gist;
-- Auto-load age in every new session and put ag_catalog on the search path.
ALTER DATABASE sl SET session_preload_libraries = 'age';
ALTER DATABASE sl SET search_path = ag_catalog, "$user", public;
