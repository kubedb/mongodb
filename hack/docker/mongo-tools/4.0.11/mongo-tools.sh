#!/bin/bash

# Copyright AppsCode Inc. and Contributors
#
# Licensed under the AppsCode Community License 1.0.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://github.com/appscode/licenses/raw/1.0.0/AppsCode-Community-1.0.0.md
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -eou pipefail

# ref: https://stackoverflow.com/a/7069755/244009
# ref: https://jonalmeida.com/posts/2013/05/26/different-ways-to-implement-flags-in-bash/
# ref: http://tldp.org/LDP/abs/html/comparison-ops.html

show_help() {
    echo "mongo-tools.sh - run tools"
    echo " "
    echo "mongo-tools.sh COMMAND [options]"
    echo " "
    echo "options:"
    echo "-h, --help                         show brief help"
    echo "    --data-dir=DIR                 path to directory holding db data (default: /var/data)"
    echo "    --host=HOST                    database host"
    echo "    --user=USERNAME                database username"
    echo "    --bucket=BUCKET                name of bucket"
    echo "    --folder=FOLDER                name of folder in bucket"
    echo "    --snapshot=SNAPSHOT            name of snapshot"
    echo "    --skip-config=true/false       skip config db of sharded cluster dump (default: false)"
    echo "    --enable-analytics=ENABLE_ANALYTICS   send analytical events to Google Analytics (default true)"
}

RETVAL=0
DEBUG=${DEBUG:-}
DB_HOST=${DB_HOST:-}
DB_PORT=${DB_PORT:-27017}
DB_USER=${DB_USER:-}
DB_PASSWORD=${DB_PASSWORD:-}
DB_BUCKET=${DB_BUCKET:-}
DB_FOLDER=${DB_FOLDER:-}
DB_SNAPSHOT=${DB_SNAPSHOT:-}
DB_SKIP_CONFIG=${DB_SKIP_CONFIG:-'false'}
DB_DATA_DIR=${DB_DATA_DIR:-/var/data}
OSM_CONFIG_FILE=/etc/osm/config
ENABLE_ANALYTICS=${ENABLE_ANALYTICS:-true}

op=$1
shift

while test $# -gt 0; do
    case "$1" in
        -h | --help)
            show_help
            exit 0
            ;;
        --data-dir*)
            export DB_DATA_DIR=$(echo $1 | sed -e 's/^[^=]*=//g')
            shift
            ;;
        --host*)
            export DB_HOST=$(echo $1 | sed -e 's/^[^=]*=//g')
            shift
            ;;
        --user*)
            export DB_USER=$(echo $1 | sed -e 's/^[^=]*=//g')
            shift
            ;;
        --bucket*)
            export DB_BUCKET=$(echo $1 | sed -e 's/^[^=]*=//g')
            shift
            ;;
        --folder*)
            export DB_FOLDER=$(echo $1 | sed -e 's/^[^=]*=//g')
            shift
            ;;
        --snapshot*)
            export DB_SNAPSHOT=$(echo $1 | sed -e 's/^[^=]*=//g')
            shift
            ;;
        --skip-config*)
            export DB_SKIP_CONFIG=$(echo $1 | sed -e 's/^[^=]*=//g')
            shift
            ;;
        --analytics* | --enable-analytics*)
            export ENABLE_ANALYTICS=$(echo $1 | sed -e 's/^[^=]*=//g')
            shift
            ;;
        --)
            shift
            break
            ;;
        *)
            show_help
            exit 1
            ;;
    esac
done

if [ -n "$DEBUG" ]; then
    env | sort | grep DB_*
    echo ""
fi

# Wait for mongodb to start
# ref: http://unix.stackexchange.com/a/5279
while ! mongo --host "$DB_HOST" --port $DB_PORT --eval "db.adminCommand('ping')"; do
    echo "Waiting... database is not ready yet"
    sleep 5
done

# cleanup data dump dir
mkdir -p "$DB_DATA_DIR"
cd "$DB_DATA_DIR"
rm -rf *

case "$op" in
    backup)
        mongodump --host "$DB_HOST" --port $DB_PORT --username "$DB_USER" --password "$DB_PASSWORD" --out "$DB_DATA_DIR" "$@"
        osm push --enable-analytics="$ENABLE_ANALYTICS" --osmconfig="$OSM_CONFIG_FILE" -c "$DB_BUCKET" "$DB_DATA_DIR" "$DB_FOLDER/$DB_SNAPSHOT"
        ;;
    restore)
        osm pull --enable-analytics="$ENABLE_ANALYTICS" --osmconfig="$OSM_CONFIG_FILE" -c "$DB_BUCKET" "$DB_FOLDER/$DB_SNAPSHOT" "$DB_DATA_DIR"
        if [[ -d "$DB_DATA_DIR/config" ]]; then
            if [[ "$DB_SKIP_CONFIG" == "false" ]]; then
                mongorestore --host "$DB_HOST" --port $DB_PORT --username "$DB_USER" --password "$DB_PASSWORD" \
                    --authenticationDatabase admin -d config "$DB_DATA_DIR/config" "$@"
            fi
            rm -rf "$DB_DATA_DIR/config"
        fi
        mongorestore --host "$DB_HOST" --port $DB_PORT --username "$DB_USER" --password "$DB_PASSWORD" "$DB_DATA_DIR" "$@"
        ;;
    *)
        (10)
        echo $"Unknown op!"
        RETVAL=1
        ;;
esac
exit "$RETVAL"
