CREATE TABLE IF NOT EXISTS scaling_operations (
    operation_id uuid PRIMARY KEY,
    direction text NOT NULL CHECK (direction IN ('out', 'in')),
    source_assignment_epoch bigint NOT NULL CHECK (source_assignment_epoch > 0),
    source_replicas integer NOT NULL CHECK (source_replicas > 0),
    target_replicas integer NOT NULL CHECK (target_replicas > 0),
    phase text NOT NULL CHECK (phase IN (
        'planning',
        'provisioning',
        'waiting_for_members',
        'publishing_assignment',
        'waiting_for_routers',
        'removing_members',
        'completed',
        'failed'
    )),
    status text NOT NULL CHECK (status IN ('in_progress', 'succeeded', 'failed')),
    version bigint NOT NULL CHECK (version >= 0),
    attempt integer NOT NULL CHECK (attempt >= 0),
    last_error_code text NULL CHECK (last_error_code IS NULL OR octet_length(last_error_code) BETWEEN 1 AND 128),
    planned_assignment_epoch bigint NULL CHECK (planned_assignment_epoch > 0),
    planned_partition_count integer NULL CHECK (planned_partition_count BETWEEN 1 AND 1024),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK (
        (planned_assignment_epoch IS NULL AND planned_partition_count IS NULL) OR
        (planned_assignment_epoch IS NOT NULL AND planned_partition_count IS NOT NULL)
    )
);
CREATE UNIQUE INDEX IF NOT EXISTS scaling_operations_one_active
    ON scaling_operations ((true))
    WHERE status = 'in_progress';

CREATE TABLE IF NOT EXISTS scaling_operation_members (
    operation_id uuid NOT NULL REFERENCES scaling_operations(operation_id) ON DELETE CASCADE,
    node_id text NOT NULL CHECK (octet_length(node_id) BETWEEN 1 AND 128),
    role text NOT NULL CHECK (role IN ('target', 'removed')),
    PRIMARY KEY (operation_id, node_id, role)
);

CREATE TABLE IF NOT EXISTS scaling_operation_assignment_nodes (
    operation_id uuid NOT NULL REFERENCES scaling_operations(operation_id) ON DELETE CASCADE,
    node_id text NOT NULL CHECK (octet_length(node_id) BETWEEN 1 AND 128),
    url text NOT NULL CHECK (octet_length(url) BETWEEN 1 AND 2048),
    PRIMARY KEY (operation_id, node_id)
);

CREATE TABLE IF NOT EXISTS scaling_operation_partition_assignments (
    operation_id uuid NOT NULL REFERENCES scaling_operations(operation_id) ON DELETE CASCADE,
    partition_id integer NOT NULL CHECK (partition_id >= 0),
    owner_node_id text NOT NULL,
    PRIMARY KEY (operation_id, partition_id),
    FOREIGN KEY (operation_id, owner_node_id)
        REFERENCES scaling_operation_assignment_nodes(operation_id, node_id)
);
