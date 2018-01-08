#!/bin/bash
set -xeou pipefail

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
    echo "    --host=HOST                    database host"
    echo "    --user=USERNAME                database username"
    echo "    --bucket=BUCKET                name of bucket"
    echo "    --folder=FOLDER                name of folder in bucket"
    echo "    --snapshot=SNAPSHOT            name of snapshot"
}

RETVAL=0
DEBUG=${DEBUG:-}

MONGO_HOST=${MONGO_HOST:-}
MONGO_USER=${MONGO_USER:-}
MONGO_PASSWORD=${MONGO_PASSWORD:-}
MONGO_BUCKET=${MONGO_BUCKET:-}
MONGO_FOLDER=${MONGO_FOLDER:-}
MONGO_SNAPSHOT=${MONGO_SNAPSHOT:-}

cmd=$1
shift

while test $# -gt 0; do
    case "$1" in
        -h|--help)
            show_help
            exit 0
            ;;
        --host*)
            export MONGO_HOST=`echo $1 | sed -e 's/^[^=]*=//g'`
            shift
            ;;
        --user*)
            export MONGO_USER=`echo $1 | sed -e 's/^[^=]*=//g'`
            shift
            ;;
        --bucket*)
            export MONGO_BUCKET=`echo $1 | sed -e 's/^[^=]*=//g'`
            shift
            ;;
        --folder*)
            export MONGO_FOLDER=`echo $1 | sed -e 's/^[^=]*=//g'`
            shift
            ;;
        --snapshot*)
            export MONGO_SNAPSHOT=`echo $1 | sed -e 's/^[^=]*=//g'`
            shift
            ;;
        *)
            show_help
            exit 1
            ;;
    esac
done

if [ -n "$DEBUG" ]; then
    env | sort | grep MONGO_*
    echo ""
fi

# Wait for mongodb to start
# ref: http://unix.stackexchange.com/a/5279
while ! nc -q 1 $MONGO_HOST 27017 </dev/null; do echo "Waiting... database is not ready yet"; sleep 5; done

case "$cmd" in
    backup)
        path=/var/dump-backup
        mkdir -p "$path"
        cd "$path"
        rm -rf *
        mongodump --host "$MONGO_HOST" --port 27017 --username "$MONGO_USER" --password "$MONGO_PASSWORD" --out $path
        osm push --osmconfig=/etc/osm/config -c "$MONGO_BUCKET" "$path" "$MONGO_FOLDER/$MONGO_SNAPSHOT"
        ;;
    restore)
        path=/var/dump-restore
        mkdir -p "$path"
        cd "$path"
        rm -rf *
        osm pull --osmconfig=/etc/osm/config -c "$MONGO_BUCKET" "$MONGO_FOLDER/$MONGO_SNAPSHOT" "$path"
        mongorestore --host "$MONGO_HOST" --port 27017 --username "$MONGO_USER" --password "$MONGO_PASSWORD"  $path
        ;;
    *)  (10)
        echo $"Unknown cmd!"
        RETVAL=1
esac
exit "$RETVAL"
