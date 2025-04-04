#!/bin/bash

set -e

# Define databases to create
POSTGRES_MULTIPLE_DATABASES="canister,device,sales,sales_readonly,truck"

function create_user_and_database() {
    local database=$1
    echo "  Creating user and database '$database'"
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-EOSQL
        CREATE DATABASE $database;
        GRANT ALL PRIVILEGES ON DATABASE $database TO $POSTGRES_USER;
EOSQL
}

# Check if POSTGRES_USER is set, default to postgres
if [ -z ${POSTGRES_USER+x} ]; then
    POSTGRES_USER=postgres
fi

echo "Multiple database creation requested: $POSTGRES_MULTIPLE_DATABASES"
for db in $(echo $POSTGRES_MULTIPLE_DATABASES | tr ',' ' '); do
    create_user_and_database $db
done
echo "Multiple databases created"