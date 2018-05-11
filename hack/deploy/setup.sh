#!/bin/bash
set -o pipefail

GOPATH=$(go env GOPATH)

REPO_ROOT="$GOPATH/src/github.com/kubedb/mongodb"
CLI_ROOT="$GOPATH/src/github.com/kubedb/cli"

pushd $REPO_ROOT

source "$REPO_ROOT/hack/deploy/settings"
source "$REPO_ROOT/hack/libbuild/common/lib.sh"

export APPSCODE_ENV=${APPSCODE_ENV:-prod}
export KUBEDB_SCRIPT="curl -fsSL https://raw.githubusercontent.com/kubedb/cli/0.8.0-beta.3/"


if [ "$APPSCODE_ENV" = "dev" ]; then
    detect_tag
    export KUBEDB_SCRIPT="cat "
    export CUSTOM_OPERATOR_TAG=$TAG
    echo ""

    if [[ ! -d $CLI_ROOT ]]; then
        echo ">>> Cloning cli repo"
        git clone -b $CLI_BRANCH https://github.com/kubedb/cli.git "${CLI_ROOT}"
    else
        echo ">>> Reusing CLI repo at ${CLI_ROOT}"
    fi
    pushd $CLI_ROOT
fi

${KUBEDB_SCRIPT}hack/deploy/kubedb.sh | bash -s -- --operator-name=mg-operator "$@"
