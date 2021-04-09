#!/bin/bash
# Remove any test artifacts created by tests
set -o errexit

root="$(dirname "$0")/../"
tmp_dir="${root:?}test/tmp"

echo "removing '${tmp_dir}' and '${root}bin'"
rm -rf --preserve-root "${tmp_dir:?}" "${root:?}bin"
