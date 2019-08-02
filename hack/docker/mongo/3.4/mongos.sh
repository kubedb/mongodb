#!/bin/bash
set -eo pipefail

# ref: https://github.com/kubernetes/charts/blob/master/stable/mongodb-replicaset/init/on-start.sh

export CONFIGDB_REPSET=${CONFIGDB_REPSET:-}
export SHARD_REPSETS=${SHARD_REPSETS:-}
export SERVICE_NAME=${SERVICE_NAME:-}
domain=$(awk -v s=search '{if($1 == s)print $3}' /etc/resolv.conf)
FULL_SVC="$SERVICE_NAME.$(awk -v s=search '{if($1 == s)print $2}' /etc/resolv.conf)"
SHARD_REPSETS=${SHARD_REPSETS//svc/$domain} # replace svc with $domain. xref: https://stackoverflow.com/a/13210909/4628962
SHARD_REPSETS_LIST=(${SHARD_REPSETS// / })  # make array that splits by space. https://stackoverflow.com/a/15400047/4628962

# awk -v s=search '{if($1 == s)print $3}' /etc/resolv.conf

if [[ "$AUTH" == "true" ]]; then
  admin_user="$MONGO_INITDB_ROOT_USERNAME"
  admin_password="$MONGO_INITDB_ROOT_PASSWORD"
  admin_creds=(-u "$admin_user" -p "$admin_password")
  auth_args=(--clusterAuthMode ${CLUSTER_AUTH_MODE} --sslMode ${SSL_MODE} --keyFile=/data/configdb/key.txt)
fi

log() {
  local msg="$1"
  local timestamp
  timestamp=$(date --iso-8601=ns)
  echo "[$timestamp] $msg" | tee -a /work-dir/log.txt
}

function shutdown_mongo() {
  if [[ $# -eq 1 ]]; then
    args="timeoutSecs: $1"
  else
    args='force: true'
  fi
  log "Shutting down mongos ($args)..."
  mongo admin "${admin_creds[@]}" "${ssl_args[@]}" --eval "db.shutdownServer({$args})"
}

# Generate the ca cert
if [[ ${SSL_MODE} != "disabled" ]]; then
  ca_crt=/data/configdb/ca.cert
  ca_key=/data/configdb/ca.key
  if [[ ! -f "$ca_crt" ]] || [[ ! -f "$ca_key" ]]; then
    log "ENABLE_SSL is set to true, but $ca_crt or $ca_key file does not exists "
    exit 1
  fi

  log "Generating certificate"

  pem=/data/configdb/mongo.pem
  ssl_args=(--ssl --sslCAFile "$ca_crt" --sslPEMKeyFile "$pem")
  auth_args=(--clusterAuthMode ${CLUSTER_AUTH_MODE} --sslMode ${SSL_MODE} --sslCAFile "$ca_crt" --sslPEMKeyFile "$pem" --keyFile=/data/configdb/key.txt)

  # extract svc-name.namespace.svc from service_name, which is pod-name.gvr-svc-name.namespace.svc.cluster.local
  svc_name="$(echo "${FULL_SVC%.svc.*}").svc"

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
DNS.2 = $SERVICE_NAME
DNS.3 = $FULL_SVC
DNS.4 = $svc_name
DNS.5 = localhost
DNS.6 = 127.0.0.1
EOL

  # Generate the certs
  export RANDFILE=/work-dir/.rnd
  openssl genrsa -out mongo.key 2048
  openssl req -new -key mongo.key -out mongo.csr -subj "/OU=MongoDB/CN=$SERVICE_NAME" -config openssl.cnf
  openssl x509 -req -in mongo.csr \
    -CA "$ca_crt" -CAkey "$ca_key" -CAcreateserial \
    -out mongo.crt -days 3650 -extensions v3_req -extfile openssl.cnf

  rm mongo.csr
  cat mongo.crt mongo.key >$pem
  rm mongo.key mongo.crt
fi

log "Ping Config Server replicaset : $CONFIGDB_REPSET"
until mongo --quiet --host "$CONFIGDB_REPSET" "${admin_creds[@]}" "${ssl_args[@]}" --eval "db.adminCommand('ping')"; do
  sleep 1
  log "Ping to Config Server replicaset fails."
done

log "Check if Config Server primary node is UP!!"
until [[ $(mongo --quiet --host "$CONFIGDB_REPSET" "${admin_creds[@]}" "${ssl_args[@]}" --eval "rs.status().hasOwnProperty('myState') && rs.status().myState==1;" | tail -1) == true ]]; do
  log "Primary Node of Config Server replicaset is not up"
  sleep 1
done

log "Starting a mongos instance..."
mongos --config /data/configdb/mongod.conf --configdb="$CONFIGDB_REPSET" --port=27017 "${auth_args[@]}" --bind_ip=0.0.0.0 | tee -a /work-dir/log.txt &

log "Waiting for mongos to be ready..."
until mongo "${ssl_args[@]}" --eval "db.adminCommand('ping')"; do
  log "Retrying..."
  sleep 2
done

log "Add shard instances"
total=${#SHARD_REPSETS_LIST[*]}

log "Shard list $total: ${SHARD_REPSETS_LIST[*]}"

for ((i = 0; i < $total; i++)); do
  repSet=${SHARD_REPSETS_LIST[$i]}
  log "Add shard: $repSet"
  mongo "${admin_creds[@]}" "${ssl_args[@]}" --eval "sh.addShard('$repSet');"
done

log "Ensure admin user credentials"
if [[ $(mongo admin "${admin_creds[@]}" "${ssl_args[@]}" --eval "db.system.users.find({user:'$admin_user'}).count()" | tail -1) == 0 ]]; then
  log "Creating admin user..."
  mongo admin "${ssl_args[@]}" --eval "db.createUser({user: '$admin_user', pwd: '$admin_password', roles: [{role: 'root', db: 'admin'}]})"
fi

# Initialize Part for KubeDB. ref: https://github.com/docker-library/mongo/blob/a499e81e743b05a5237e2fd700c0284b17d3d416/3.4/docker-entrypoint.sh#L302
# Start
log "Ensure Initializing init scripts"
if [[ $(mongo admin "${admin_creds[@]}" "${ssl_args[@]}" --eval "db.kubedb.find({'_id' : 'kubedb','kubedb' : 'initialized'}).count()" | tail -1) == 0 ]] &&
  [[ $(mongo admin "${admin_creds[@]}" "${ssl_args[@]}" --eval "db.kubedb.insert({'_id' : 'kubedb','kubedb' : 'initialized'});" |
    grep -c "E11000 duplicate key error collection: admin.kubedb") -eq 0 ]]; then

  export MONGO_INITDB_DATABASE="${MONGO_INITDB_DATABASE:-test}"
  log "Initialize init scripts"
  echo
  ls -la /docker-entrypoint-initdb.d
  for f in /docker-entrypoint-initdb.d/*; do
    case "$f" in
      *.sh)
        echo "$0: running $f"
        . "$f"
        ;;
      *.js)
        echo "$0: running $f 1"
        mongo "$MONGO_INITDB_DATABASE" "${admin_creds[@]}" "${ssl_args[@]}" --authenticationDatabase admin "$f"
        ;;
      *) echo "$0: ignoring $f" ;;
    esac
    echo
  done
  # END

  log "Done."
fi

shutdown_mongo
log "Good bye."
