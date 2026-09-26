CREATE TABLE IF NOT EXISTS documents (
    partition_key text NOT NULL CHECK (octet_length(partition_key) BETWEEN 1 AND 256),
    id text NOT NULL CHECK (octet_length(id) BETWEEN 1 AND 256),
    payload jsonb NOT NULL CHECK (jsonb_typeof(payload) = 'object'),
    revision uuid NOT NULL DEFAULT gen_random_uuid(),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (partition_key, id)
);
