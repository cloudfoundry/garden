#!/bin/bash

set -eu
set -o pipefail

. "/ci/shared/helpers/git-helpers.bash"

function test() {
  local package="${1:?Provide a package}"
  local sub_package="${2:-}"

  export DIR=${package}
  . <(/ci/shared/helpers/extract-default-params-for-task.bash /ci/shared/tasks/run-bin-test/linux.yml)

  export GOFLAGS="-buildvcs=false"
  /ci/shared/tasks/run-bin-test/task.bash "${sub_package}"
}

pushd / > /dev/null
if [[ -n "${1:-}" ]]; then
  test "${1}" "${2:-}"
else
  test "."
fi
popd > /dev/null
