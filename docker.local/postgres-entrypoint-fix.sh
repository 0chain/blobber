#!/bin/bash
set -e

# Fix broken symlinks in pg_tblspc before PostgreSQL starts
# This prevents the "chown: cannot dereference" error
if [ -d "/var/lib/postgresql/data/pg_tblspc" ]; then
    echo "Checking for broken symlinks in pg_tblspc..."
    find /var/lib/postgresql/data/pg_tblspc -type l ! -exec test -e {} \; -print | while read -r broken_link; do
        echo "Removing broken symlink: $broken_link"
        rm -f "$broken_link"
    done
fi

# Ensure the tablespace directory exists if SLOW_TABLESPACE_PATH is set
if [ -n "$SLOW_TABLESPACE_PATH" ]; then
    echo "Ensuring tablespace directory exists: $SLOW_TABLESPACE_PATH"
    mkdir -p "$SLOW_TABLESPACE_PATH"
    chown -R postgres:postgres "$SLOW_TABLESPACE_PATH" 2>/dev/null || true
fi

# Call the original PostgreSQL entrypoint
exec /usr/local/bin/docker-entrypoint.sh "$@"

