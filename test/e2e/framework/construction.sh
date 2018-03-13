#!/bin/bash
set -eou pipefail

# https://stackoverflow.com/a/677212/244009
if [[ ! -z "$(command -v onessl)" ]]; then
    export ONESSL=onessl
else
    echo 'onessl command not found. follow: https://github.com/kubepack/onessl '
fi


export KUBEDB_NAMESPACE=kube-system


# create necessary TLS certificates:
# - a local CA key and cert
# - a webhook server key and cert signed by the local CA
$ONESSL create ca-cert --cert-dir=hack/config --overwrite
$ONESSL create server-cert server --domains=kubedb-operator.$KUBEDB_NAMESPACE.svc --cert-dir=hack/config --overwrite
export SERVICE_SERVING_CERT_CA=$(cat hack/config/ca.crt | $ONESSL base64)
export KUBE_CA=$($ONESSL get kube-ca | $ONESSL base64)
mv hack/config/server.crt hack/config/tls.crt
mv hack/config/server.key hack/config/tls.key

cat test/e2e/framework/admission-config.yaml | $ONESSL envsubst | kubectl apply -f -
