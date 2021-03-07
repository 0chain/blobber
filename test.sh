#!/usr/bin/env bash
set -e

for mod_file in $(find * -name go.mod); do
    mod_dir=$(dirname $mod_file)
    (cd $mod_dir; go test ./...)
done
