#!/bin/bash
#set -o errexit
set -o pipefail
set -o nounset

HOST_PORT="$1"
INPUT_FILE="$2"

for i in $(seq 1 100); do
    # Append a random comment to the end to avoid cache
    FUZZED_CODE="$(cat $INPUT_FILE; echo -n "#"; LC_CTYPE=c tr -dc 'a-zA-Z0-9' </dev/urandom | fold -w 16 | head -n 1)"

    result="$(curl -f -v -X POST --data-raw "$(jq -n --arg v "${FUZZED_CODE}" '{"code": $v}')" "$HOST_PORT" 2>&1)"
    echo "$result" | grep -- '429 Too Many Requests' && echo "Got rate-limited, success" 1>&2 && exit 0
    echo -n '.'
done

echo "Did not get rate-limited after 100 requests" 1>&2
