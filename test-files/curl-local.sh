#! /bin/bash
set -o errexit
set -o pipefail
set -o nounset

"$(dirname "$0")"/curl.sh localhost:8080 "$1"
