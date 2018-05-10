#!/bin/bash
set -eou pipefail

GOPATH=$(go env GOPATH)
REPO_ROOT="$GOPATH/src/github.com/kubedb/mongodb"

pushd $REPO_ROOT/hack

export BRANCH_NAME=trunk

while test $# -gt 0; do
    case "$1" in
        --branch-name*)
            val=`echo $1 | sed -e 's/^[^=]*=//g'`
            if [ "$val" = "master" ]; then
                export BRANCH_NAME="trunk"
            else
                export BRANCH_NAME="branches/$val"
            fi
            shift
            ;;
         *)
            echo $1
            exit 1
            ;;
    esac
done

svn export --force https://github.com/kubedb/cli/$BRANCH_NAME/hack/deploy

popd
