# apache/age PG18 1.8.0 (Docker Hub) + pgvector v0.8.7 (github.com/pgvector/pgvector tags) 소스 빌드
FROM apache/age:release_PG18_1.8.0
ARG PGVECTOR_TAG=v0.8.7
RUN apt-get update \
 && apt-get install -y --no-install-recommends build-essential git ca-certificates postgresql-server-dev-18 \
 && git clone --depth 1 --branch ${PGVECTOR_TAG} https://github.com/pgvector/pgvector.git /tmp/pgvector \
 && make -C /tmp/pgvector && make -C /tmp/pgvector install \
 && rm -rf /tmp/pgvector /var/lib/apt/lists/* \
 && apt-get purge -y build-essential git postgresql-server-dev-18 && apt-get autoremove -y
