#!/bin/bash

# Copyright The KubeDB Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -eo pipefail

# scripts inside /docker-entrypoint-initdb.d/ are executed alphabetically.
# so, 000 prefix is added in filename to try executing this file first.

# create client certificate as $external user

client_pem="${MONGO_CLIENT_CERTIFICATE_PATH:-/data/configdb/client.pem}"
ca_crt="${MONGO_CA_CERTIFICATE_PATH:-/data/configdb/ca.cert}"

if [[ ${SSL_MODE} != "disabled" ]] && [[ -f "$client_pem" ]] && [[ -f "$ca_crt" ]]; then
  admin_user="${MONGO_INITDB_ROOT_USERNAME:-}"
  admin_password="${MONGO_INITDB_ROOT_PASSWORD:-}"
  admin_creds=(-u "$admin_user" -p "$admin_password" --authenticationDatabase admin)
  ssl_args=(--ssl --sslCAFile "$ca_crt" --sslPEMKeyFile "$client_pem")

  user=$(openssl x509 -in "$client_pem" -inform PEM -subject -nameopt RFC2253 -noout)
  # the output is similar to `subject= CN=root,O=kubedb:client`. so, cut out 'subject= '
  user=${user#"subject= "}
  echo "Creating root user $user for SSL..." #xref: https://docs.mongodb.com/manual/tutorial/configure-x509-client-authentication/#procedures
  mongo admin --host localhost "${admin_creds[@]}" "${ssl_args[@]}" --eval "db.getSiblingDB(\"\$external\").runCommand({createUser: \"$user\",roles:[{role: 'root', db: 'admin'}],})"
fi
