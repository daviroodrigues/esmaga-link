#!/usr/bin/env bash
set -euo pipefail

BACKUP_DIR="${BACKUP_DIR:-./backups}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-7}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-esmaga-postgres}"
POSTGRES_USER="${POSTGRES_USER:-esmaga_user}"
POSTGRES_DB="${POSTGRES_DB:-esmaga_auth}"
CASSANDRA_CONTAINER="${CASSANDRA_CONTAINER:-esmaga-cassandra}"
CASSANDRA_KEYSPACE="${CASSANDRA_KEYSPACE:-esmaga_link}"

TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
PG_DIR="${BACKUP_DIR}/postgres"
mkdir -p "${PG_DIR}"

echo "[Backup] Iniciando backup do PostgreSQL (${TIMESTAMP})..."
docker exec "${POSTGRES_CONTAINER}" pg_dump -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" \
  | gzip > "${PG_DIR}/postgres_${TIMESTAMP}.sql.gz"

echo "[Backup] Removendo backups PostgreSQL com mais de ${RETENTION_DAYS} dias..."
find "${PG_DIR}" -type f -name "postgres_*.sql.gz" -mtime +"${RETENTION_DAYS}" -delete

echo "[Backup] Criando snapshot do Cassandra (${CASSANDRA_KEYSPACE})..."
docker exec "${CASSANDRA_CONTAINER}" nodetool snapshot -t "snap_${TIMESTAMP}" "${CASSANDRA_KEYSPACE}"

echo "[Backup] Concluído com sucesso em ${TIMESTAMP}."
