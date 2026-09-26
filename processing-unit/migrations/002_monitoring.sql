-- psql init script: passwords are read from the container environment.
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
\getenv monitor_user MONITOR_USER
\getenv monitor_password MONITOR_PASSWORD
SELECT format('CREATE ROLE %I LOGIN PASSWORD %L', :'monitor_user', :'monitor_password') \gexec
SELECT format('GRANT pg_monitor TO %I', :'monitor_user') \gexec
