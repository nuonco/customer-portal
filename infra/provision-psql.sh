#!/bin/bash

set -eou pipefail
set -x

OUTPUT=$(terraform output -json)

DB_ADDR="$(echo $OUTPUT | jq -r .db_instance_host.value)"
DB_PORT="5432"

DB_NAME="customer_dashboard"
DB_USER="customer_dashboard"

ADMIN_USER="$(echo $OUTPUT | jq -r .db_admin_username.value)"
ADMIN_PW="$(echo $OUTPUT | jq -r .db_admin_password.value)"
ADMIN_DB="$(echo $OUTPUT | jq -r .db_instance_name.value)"

echo "creating customer_dashboard user with IAM auth..."
cat <<EOF | PGPASSWORD="$ADMIN_PW" psql \
    -h "$DB_ADDR" \
    -p "$DB_PORT" \
    -U "$ADMIN_USER" \
    -d "$ADMIN_DB" \
    --no-psqlrc \
    -f -

DO \$\$
BEGIN
CREATE USER customer_dashboard WITH LOGIN;
EXCEPTION WHEN duplicate_object THEN RAISE NOTICE '%, skipping', SQLERRM USING ERRCODE = SQLSTATE;
END
\$\$;

GRANT rds_iam TO customer_dashboard;
CREATE DATABASE customer_dashboard;
EOF

echo "granting permissions on customer_dashboard database..."
cat <<EOF | PGPASSWORD="$ADMIN_PW" psql \
    -h "$DB_ADDR" \
    -p "$DB_PORT" \
    -U "$ADMIN_USER" \
    -d "customer_dashboard" \
    --no-psqlrc \
    -f -

GRANT ALL ON SCHEMA public TO customer_dashboard;
GRANT ALL ON ALL TABLES IN SCHEMA public TO customer_dashboard;
GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO customer_dashboard;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO customer_dashboard;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO customer_dashboard;
EOF

echo "transferring ownership of existing tables to customer_dashboard..."
cat <<EOF | PGPASSWORD="$ADMIN_PW" psql \
    -h "$DB_ADDR" \
    -p "$DB_PORT" \
    -U "$ADMIN_USER" \
    -d "customer_dashboard" \
    --no-psqlrc \
    -f -

-- Transfer ownership of all tables in public schema to customer_dashboard
DO \$\$
DECLARE
    r RECORD;
BEGIN
    FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = 'public') LOOP
        EXECUTE 'ALTER TABLE public.' || quote_ident(r.tablename) || ' OWNER TO customer_dashboard';
    END LOOP;
END
\$\$;

-- Transfer ownership of all sequences in public schema to customer_dashboard
DO \$\$
DECLARE
    r RECORD;
BEGIN
    FOR r IN (SELECT sequencename FROM pg_sequences WHERE schemaname = 'public') LOOP
        EXECUTE 'ALTER SEQUENCE public.' || quote_ident(r.sequencename) || ' OWNER TO customer_dashboard';
    END LOOP;
END
\$\$;
EOF

echo "provisioning complete - customer_dashboard user created with full permissions and ownership"
