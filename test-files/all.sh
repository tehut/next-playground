#!/bin/bash
set -o errexit
set -o pipefail
set -o nounset

HOST_PORT="$1"

DIR="$(dirname "$0")"
FAILED="0"

fail() {
    echo "$1" 1>&2
    FAILED="1"
}

set +e

"${DIR}/curl.sh" "${HOST_PORT}" "${DIR}"/valid.jsonnet || fail "valid.jsonnet failed"
"${DIR}/curl.sh" "${HOST_PORT}" "${DIR}"/import.jsonnet || fail "import.jsonnet failed"

"${DIR}/curl.sh" "${HOST_PORT}" "${DIR}"/invalid.jsonnet && fail "invalid.jsonnet should have failed but did not"
"${DIR}/curl.sh" "${HOST_PORT}" "${DIR}"/slow.jsonnet && fail "slow.jsonnet should have failed but did not"

if [[ "${FAILED}" -eq "1" ]]; then
    exit 1
fi
