CREATE KEYSPACE IF NOT EXISTS esmaga_link 
WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};

CREATE TABLE IF NOT EXISTS esmaga_link.links_by_code (
    short_code text,
    original_url text,
    user_id uuid,
    is_active boolean,
    created_at timestamp,
    PRIMARY KEY (short_code)
);

CREATE TABLE IF NOT EXISTS esmaga_link.links_by_user (
    user_id uuid,
    created_at timestamp,
    short_code text,
    original_url text,
    is_active boolean,
    PRIMARY KEY ((user_id), created_at, short_code)
) WITH CLUSTERING ORDER BY (created_at DESC, short_code ASC);

CREATE TABLE IF NOT EXISTS esmaga_link.click_events (
    short_code text,
    bucket_date date,
    clicked_at timestamp,
    event_id timeuuid,
    ip_address text,
    user_agent text,
    referrer text,
    country text,
    device_type text,
    PRIMARY KEY ((short_code, bucket_date), clicked_at, event_id)
) WITH CLUSTERING ORDER BY (clicked_at DESC);

CREATE TABLE IF NOT EXISTS esmaga_link.id_checkpoints (
    sequence_name text,
    last_allocated_id bigint,
    updated_at timestamp,
    PRIMARY KEY (sequence_name)
);
