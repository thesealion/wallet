#!/bin/bash
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE TABLE accounts (
        id TEXT,
        balance NUMERIC,
        currency TEXT,
        PRIMARY KEY(id)
    );
    CREATE TABLE payments (
        id SERIAL,
        from_account_id TEXT,
        to_account_id TEXT,
        amount NUMERIC,
        PRIMARY KEY(id)
    );

    INSERT INTO accounts VALUES ('bob123', 100.0, 'USD');
    INSERT INTO accounts VALUES ('alice456', 0.01, 'USD');
    INSERT INTO accounts VALUES ('eve789', 5000, 'RUB');
EOSQL
