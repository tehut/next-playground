#!/bin/bash
set -o errexit
set -o pipefail
set -o nounset

HOST_PORT="$1"
INPUT_FILE="$2"

curl -sf -X POST --data-raw "$(jq -n --arg v "$(cat $INPUT_FILE)" '{"code": $v}')" "$HOST_PORT"; echo
