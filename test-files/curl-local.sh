#! /bin/bash

INPUT_FILE="$1"

curl -X POST --data-raw "$(jq -n --arg v "$(cat $INPUT_FILE)" '{"code": $v}')" localhost:8080