#!/bin/bash
# Script to fix broken PostgreSQL symlinks in pg_tblspc

BLOBBER=${BLOBBER:-1}
DATA_DIR="./blobber${BLOBBER}/data/postgresql"

if [ ! -d "$DATA_DIR" ]; then
    echo "Error: Data directory not found: $DATA_DIR"
    echo "Please set BLOBBER environment variable if using a different blobber number"
    exit 1
fi

PG_TBLSPC_DIR="$DATA_DIR/pg_tblspc"

if [ -d "$PG_TBLSPC_DIR" ]; then
    echo "Checking for broken symlinks in $PG_TBLSPC_DIR..."
    broken_links=$(find "$PG_TBLSPC_DIR" -type l ! -exec test -e {} \; -print)
    
    if [ -z "$broken_links" ]; then
        echo "No broken symlinks found."
    else
        echo "Found broken symlinks. Removing them..."
        find "$PG_TBLSPC_DIR" -type l ! -exec test -e {} \; -delete
        echo "Broken symlinks removed."
    fi
else
    echo "pg_tblspc directory not found. This is normal for a fresh database."
fi

echo "Done!"

