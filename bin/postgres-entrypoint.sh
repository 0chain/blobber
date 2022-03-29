#!/bin/bash

set -e

psql=( psql --username "$POSTGRES_USER" --port "$POSTGRES_PORT" --host "$POSTGRES_HOST" )

until pg_isready -h $POSTGRES_HOST
do
	echo "Sleep 1s and try again..."
	sleep 1
done

export PGPASSWORD="$POSTGRES_PASSWORD"
