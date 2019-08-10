#!/bin/bash

# ref: https://github.com/kubernetes/charts/blob/master/stable/mongodb-replicaset/init/on-start.sh

replica_set="$REPLICA_SET"
script_name=${0##*/}

if [[ "$AUTH" == "true" ]]; then
  admin_user="$MONGO_INITDB_ROOT_USERNAME"
  admin_password="$MONGO_INITDB_ROOT_PASSWORD"
  admin_creds=(-u "$admin_user" -p "$admin_password" --authenticationDatabase admin)
  auth_args=(--clusterAuthMode ${CLUSTER_AUTH_MODE} --sslMode ${SSL_MODE} --auth --keyFile=/data/configdb/key.txt)
fi

log() {
  local msg="$1"
  local timestamp
  timestamp=$(date --iso-8601=ns)
  echo "[$timestamp] [$script_name] $msg" | tee -a /work-dir/log.txt
}

function shutdown_mongo() {
  if [[ $# -eq 1 ]]; then
    args="timeoutSecs: $1"
  else
    args='force: true'
  fi
  log "Shutting down MongoDB ($args)..."
  mongo admin "${admin_creds[@]}" "${ssl_args[@]}" --eval "db.shutdownServer({$args})"
}

my_hostname=$(hostname)
log "Bootstrapping MongoDB replica set member: $my_hostname"

log "Reading standard input..."
while read -ra line; do
  if [[ "${line}" == *"${my_hostname}"* ]]; then
    service_name="$line"
    continue
  fi
  peers=("${peers[@]}" "$line")
done

# Generate the ca cert
if [[ ${SSL_MODE} != "disabled" ]]; then
  ca_crt=/data/configdb/ca.cert
  ca_key=/data/configdb/ca.key
  client_pem=/data/configdb/client.pem
  if [[ ! -f "$ca_crt" ]] || [[ ! -f "$ca_key" ]]; then
    log "ENABLE_SSL is set to true, but $ca_crt or $ca_key file does not exists "
    exit 1
  fi

  log "Generating certificate"

  pem=/data/configdb/mongo.pem
  ssl_args=(--ssl --sslCAFile "$ca_crt" --sslPEMKeyFile "$pem")
  auth_args=(--clusterAuthMode ${CLUSTER_AUTH_MODE} --sslMode ${SSL_MODE} --sslCAFile "$ca_crt" --sslPEMKeyFile "$pem" --keyFile=/data/configdb/key.txt)

  # extract pod-name.gvr-svc-name.namespace.svc from service_name, which is pod-name.gvr-svc-name.namespace.svc.cluster.local
  svc_name="$(echo "${service_name%.svc.*}").svc"

  # Move into /work-dir
  pushd /work-dir

  cat >openssl.cnf <<EOL
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage  = serverAuth, clientAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = $(echo -n "$my_hostname" | sed s/-[0-9]*$//)
DNS.2 = $my_hostname
DNS.3 = $service_name
DNS.4 = $svc_name
DNS.5 = localhost
DNS.6 = 127.0.0.1
EOL

  # Generate the certs
  export RANDFILE=/work-dir/.rnd
  openssl genrsa -out mongo.key 2048
  openssl req -new -key mongo.key -out mongo.csr -subj "/OU=MongoDB/CN=$my_hostname" -config openssl.cnf
  openssl x509 -req -in mongo.csr \
    -CA "$ca_crt" -CAkey "$ca_key" -CAcreateserial \
    -out mongo.crt -days 3650 -extensions v3_req -extfile openssl.cnf

  rm mongo.csr
  cat mongo.crt mongo.key >$pem
  rm mongo.key mongo.crt
fi

log "Peers: ${peers[*]}"

log "Starting a MongoDB instance..."
mongod --config /data/configdb/mongod.conf --dbpath=/data/db --shardsvr --replSet="$replica_set" --port=27017 "${auth_args[@]}" --bind_ip=0.0.0.0 2>&1 | tee -a /work-dir/log.txt &

log "Waiting for MongoDB to be ready..."
until mongo "${ssl_args[@]}" --eval "db.adminCommand('ping')"; do
  log "Retrying..."
  sleep 2
done

log "Initialized."

# try to find a master and add yourself to its replica set.
for peer in "${peers[@]}"; do
  if mongo admin --host "$peer" "${admin_creds[@]}" "${ssl_args[@]}" --eval "rs.isMaster()" | grep '"ismaster" : true'; then
    log "Found master: $peer"
    log "Adding myself ($service_name) to replica set..."
    mongo admin --host "$peer" "${admin_creds[@]}" "${ssl_args[@]}" --eval "rs.add('$service_name')"

    sleep 3

    log 'Waiting for replica to reach SECONDARY state...'
    until printf '.' && [[ $(mongo admin "${admin_creds[@]}" "${ssl_args[@]}" --quiet --eval "rs.status().myState") == '2' ]]; do
      sleep 1
    done

    log '✓ Replica reached SECONDARY state.'

    shutdown_mongo "60"
    log "Good bye."
    exit 0
  fi
done

# else initiate a replica set with yourself.
if mongo "${ssl_args[@]}" --eval "rs.status()" | grep "no replset config has been received"; then
  log "Initiating a new replica set with myself ($service_name)..."
  mongo "${ssl_args[@]}" --eval "rs.initiate({'_id': '$replica_set', 'members': [{'_id': 0, 'host': '$service_name'}]})"

  sleep 3

  log 'Waiting for replica to reach PRIMARY state...'
  until printf '.' && [[ $(mongo "${ssl_args[@]}" --quiet --eval "rs.status().myState") == '1' ]]; do
    sleep 1
  done

  log '✓ Replica reached PRIMARY state.'

  if [[ "$AUTH" == "true" ]]; then
    log "Creating admin user..."
    mongo admin "${ssl_args[@]}" --eval "db.createUser({user: '$admin_user', pwd: '$admin_password', roles: [{role: 'root', db: 'admin'}]})"
  fi

  if [[ ${SSL_MODE} != "disabled" ]] && [[ -f "$client_pem" ]]; then
    user=$(openssl x509 -in "$client_pem" -inform PEM -subject -nameopt RFC2253 -noout)
    user=${user#"subject= "}
    log "Creating root user $user for SSL..." #xref: https://docs.mongodb.com/manual/tutorial/configure-x509-client-authentication/#procedures
    mongo admin "${admin_creds[@]}" "${ssl_args[@]}" --eval "db.getSiblingDB(\"\$external\").runCommand({createUser: \"$user\",roles:[{role: 'root', db: 'admin'}],})"
  fi

  log "Done."
fi

shutdown_mongo
log "Good bye."
