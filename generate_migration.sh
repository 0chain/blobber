#!/bin/bash
MIGRATION_DIRECTORY="goose/migrations"
ts=$(date +%s)
fname=$1

if [ -z "$fname" ]; then
    echo "Usage: $0 <filename>"
    exit 1
fi


# Write boilerplate of the migration file
echo "-- +goose Up
-- +goose StatementBegin

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- +goose StatementEnd" > "$MIGRATION_DIRECTORY/""$ts""_$fname.sql"