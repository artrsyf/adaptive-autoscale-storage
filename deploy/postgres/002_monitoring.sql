CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
CREATE ROLE monitor LOGIN PASSWORD 'local-monitor';
GRANT pg_monitor TO monitor;
