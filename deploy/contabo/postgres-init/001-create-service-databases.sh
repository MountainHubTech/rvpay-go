#!/bin/sh
# Runs once, on first container start, because the named volume is empty.
# The official postgres image only auto-creates the database named by
# POSTGRES_DB; Clients and Transactions each own their own logical database
# on this shared instance, so the extra ones are created here.
set -eu

for db in clients transactions; do
  psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres \
    -c "SELECT 'CREATE DATABASE ${db}' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '${db}')\gexec"
done
