#!/bin/bash

set -eu
set -o pipefail

THIS_FILE_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
CI="${THIS_FILE_DIR}/../../wg-app-platform-runtime-ci"
. "$CI/shared/helpers/git-helpers.bash"
REPO_NAME=$(git_get_remote_name)
REPO_PATH="${THIS_FILE_DIR}/../"
unset THIS_FILE_DIR

IMAGE="cloudfoundry/tas-runtime-build"
CONTAINER_NAME="$REPO_NAME-docker-container"

if [[ -z "${*}" ]]; then
  ARGS="-it"
else
  ARGS="${*}"
fi

docker pull "${IMAGE}"
docker rm -f $CONTAINER_NAME
docker run -it \
  --env "REPO_NAME=$REPO_NAME" \
  --env "REPO_PATH=/repo" \
  --rm \
  --name "$CONTAINER_NAME" \
  -v "${REPO_PATH}:/repo" \
  -v "${CI}:/ci" \
  ${ARGS} \
  "${IMAGE}" \
  /bin/bash
  
