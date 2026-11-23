-- Keyspace para links e eventos
CREATE KEYSPACE IF NOT EXISTS esmaga_link 
WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};

-- 1. Caminho de Redirecionamento (Lookup rápido O(1))
CREATE TABLE IF NOT EXISTS esmaga_link.links_by_code (
    short_code text,
    original_url text,
    user_id uuid,          -- NULL para links anônimos
    is_active boolean,
    created_at timestamp,
    PRIMARY KEY (short_code)
);

-- 2. Listagem de links por usuário (Dashboard)
CREATE TABLE IF NOT EXISTS esmaga_link.links_by_user (
    user_id uuid,
    created_at timestamp,
    short_code text,
    original_url text,
    is_active boolean,
    PRIMARY KEY ((user_id), created_at, short_code)
) WITH CLUSTERING ORDER BY (created_at DESC, short_code ASC);

-- 3. Métricas e Logs de Cliques (Time-Series particionado por dia)
CREATE TABLE IF NOT EXISTS esmaga_link.click_events (
    short_code text,
    bucket_date date,       -- ex: 2026-11-08 (evita que a partição cresça descontroladamente)
    clicked_at timestamp,
    event_id timeuuid,
    ip_address text,
    user_agent text,
    referrer text,
    country text,
    device_type text,
    PRIMARY KEY ((short_code, bucket_date), clicked_at, event_id)
) WITH CLUSTERING ORDER BY (clicked_at DESC);

-- 4. Checkpoints de Sequência de IDs (Single Source of Truth para Disaster Recovery)
CREATE TABLE IF NOT EXISTS esmaga_link.id_checkpoints (
    sequence_name text,
    last_allocated_id bigint,
    updated_at timestamp,
    PRIMARY KEY (sequence_name)
);
