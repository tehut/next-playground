#!/bin/bash
set -o errexit
set -o pipefail

# Must be set by CI system
if [[ -z "${IMAGE}" ]]; then
    echo "The \$IMAGE environment variable must be defined and set to the docker image under test." 1>&2
    exit 1
fi

cd "$(dirname "$0")/.."
CONTAINER_NAME=ksonnet-playground-test-$RANDOM
TEST_IMAGE=ksonnet-playground-tester-$RANDOM

function cleanup() {
    set +e
    docker rmi "${TEST_IMAGE}" >/dev/null 2>&1
    docker kill "${CONTAINER_NAME}" >/dev/null 2>&1
    docker rm "${CONTAINER_NAME}" >/dev/null 2>&1
}
trap cleanup EXIT

# Build the image
(
  cd test-files/docker
  docker build -t "${TEST_IMAGE}" .
)

## Tests pt1
# Run the image in the background
docker run -d --name "${CONTAINER_NAME}" -v "$(pwd):$(pwd)" "${IMAGE}"

# Run a tester against it, using docker linked containers (we can't guarantee
# we can actually listen on ports)
docker run --rm -t \
    -v "$(pwd):$(pwd)" \
    --link "${CONTAINER_NAME}:${CONTAINER_NAME}" \
    "${TEST_IMAGE}" \
    "$(pwd)/test-files/all.sh" "${CONTAINER_NAME}:8080"

docker kill "${CONTAINER_NAME}"
docker rm "${CONTAINER_NAME}"

## Tests pt2
# Run the image in the background, with smaller rate limits
docker run -d --name "${CONTAINER_NAME}" -v "$(pwd):$(pwd)" "${IMAGE}" /ksonnet-playground --rate-limit 1 --rate-limit-burst 1

# Run a tester against it, using docker linked containers (we can't guarantee
# we can actually listen on ports)
docker run --rm -t \
    -v "$(pwd):$(pwd)" \
    --link "${CONTAINER_NAME}:${CONTAINER_NAME}" \
    "${TEST_IMAGE}" \
    "$(pwd)/test-files/hit-rate-limit.sh" "${CONTAINER_NAME}:8080" "$(pwd)/test-files/sample.jsonnet"
