CREATE TABLE IF NOT EXISTS assignment_state (
    singleton_id boolean PRIMARY KEY DEFAULT true CHECK (singleton_id),
    epoch bigint NOT NULL CHECK (epoch > 0),
    partition_count integer NOT NULL CHECK (partition_count BETWEEN 1 AND 1024),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE IF NOT EXISTS assignment_nodes (
    epoch bigint NOT NULL CHECK (epoch > 0),
    node_id text NOT NULL CHECK (octet_length(node_id) BETWEEN 1 AND 128),
    url text NOT NULL CHECK (octet_length(url) BETWEEN 1 AND 2048),
    PRIMARY KEY (epoch, node_id)
);

CREATE TABLE IF NOT EXISTS partition_assignments (
    epoch bigint NOT NULL CHECK (epoch > 0),
    partition_id integer NOT NULL CHECK (partition_id >= 0),
    owner_node_id text NOT NULL,
    PRIMARY KEY (epoch, partition_id),
    FOREIGN KEY (epoch, owner_node_id) REFERENCES assignment_nodes(epoch, node_id)
);

INSERT INTO assignment_state(singleton_id, epoch, partition_count)
VALUES (true, 1, 128)
ON CONFLICT (singleton_id) DO NOTHING;

INSERT INTO assignment_nodes(epoch, node_id, url)
VALUES
    (1, 'processing-unit-1', 'http://processing-unit-1:8080'),
    (1, 'processing-unit-2', 'http://processing-unit-2:8080'),
    (1, 'processing-unit-3', 'http://processing-unit-3:8080')
ON CONFLICT (epoch, node_id) DO NOTHING;

INSERT INTO partition_assignments(epoch, partition_id, owner_node_id)
SELECT 1, partition_id, 'processing-unit-' || ((partition_id % 3) + 1)::text
FROM generate_series(0, 127) AS partition_id
ON CONFLICT (epoch, partition_id) DO NOTHING;
