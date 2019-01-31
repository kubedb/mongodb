#!/bin/bash
set -xeou pipefail

DOCKER_REGISTRY=${DOCKER_REGISTRY:-kubedb}

IMG=mongo-tools
DB_VERSION=4.0
PATCH=4.0.5

TAG="$DB_VERSION" # suffix will be added later
BASE_TAG="$PATCH" # suffix will be added later


docker pull "$DOCKER_REGISTRY/$IMG:$BASE_TAG"

docker tag "$DOCKER_REGISTRY/$IMG:$BASE_TAG" "$DOCKER_REGISTRY/$IMG:$TAG"
docker push "$DOCKER_REGISTRY/$IMG:$TAG"
