#!/bin/bash
# Remove any test artifacts created by tests
set -o errexit

here="$(dirname "$(readlink --canonicalize "${BASH_SOURCE[0]}")")"
root="$(readlink --canonicalize "$here/..")"
tmp_dir="${root}/test/tmp"

echo "removing '${tmp_dir}' and '${root}/bin'"
rm -rf --preserve-root "${tmp_dir:?}" "${root:?}bin"
