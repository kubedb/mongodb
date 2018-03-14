#!/bin/bash
set -eou pipefail

# https://stackoverflow.com/a/677212/244009
if [[ ! -z "$(command -v onessl)" ]]; then
    export ONESSL=onessl
else
    echo 'onessl command not found. follow: https://github.com/kubepack/onessl '
fi


export KUBEDB_NAMESPACE=kube-system

export KUBE_CA=$($ONESSL get kube-ca | $ONESSL base64)

cat hack/dev/admission-config.yaml | $ONESSL envsubst | kubectl apply -f -
