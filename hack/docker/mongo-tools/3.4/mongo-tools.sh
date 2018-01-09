#!/bin/bash
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
    echo "    --host=HOST                    database host"
    echo "    --user=USERNAME                database username"
    echo "    --bucket=BUCKET                name of bucket"
    echo "    --folder=FOLDER                name of folder in bucket"
    echo "    --snapshot=SNAPSHOT            name of snapshot"
}

RETVAL=0
DEBUG=${DEBUG:-}
DB_HOST=${DB_HOST:-}
DB_USER=${DB_USER:-}
DB_PASSWORD=${DB_PASSWORD:-}
DB_BUCKET=${DB_BUCKET:-}
DB_FOLDER=${DB_FOLDER:-}
DB_SNAPSHOT=${DB_SNAPSHOT:-}

op=$1
shift

while test $# -gt 0; do
    case "$1" in
        -h|--help)
            show_help
            exit 0
            ;;
        --host*)
            export DB_HOST=`echo $1 | sed -e 's/^[^=]*=//g'`
            shift
            ;;
        --user*)
            export DB_USER=`echo $1 | sed -e 's/^[^=]*=//g'`
            shift
            ;;
        --bucket*)
            export DB_BUCKET=`echo $1 | sed -e 's/^[^=]*=//g'`
            shift
            ;;
        --folder*)
            export DB_FOLDER=`echo $1 | sed -e 's/^[^=]*=//g'`
            shift
            ;;
        --snapshot*)
            export DB_SNAPSHOT=`echo $1 | sed -e 's/^[^=]*=//g'`
            shift
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
while ! nc -q 1 $DB_HOST 27017 </dev/null; do echo "Waiting... database is not ready yet"; sleep 5; done

case "$op" in
    backup)
        path=/var/dump-backup
        mkdir -p "$path"
        cd "$path"
        rm -rf *
        mongodump --host "$DB_HOST" --port 27017 --username "$DB_USER" --password "$DB_PASSWORD" --out $path
        osm push --osmconfig=/etc/osm/config -c "$DB_BUCKET" "$path" "$DB_FOLDER/$DB_SNAPSHOT"
        ;;
    restore)
        path=/var/dump-restore
        mkdir -p "$path"
        cd "$path"
        rm -rf *
        osm pull --osmconfig=/etc/osm/config -c "$DB_BUCKET" "$DB_FOLDER/$DB_SNAPSHOT" "$path"
        mongorestore --host "$DB_HOST" --port 27017 --username "$DB_USER" --password "$DB_PASSWORD"  $path
        ;;
    *)  (10)
        echo $"Unknown op!"
        RETVAL=1
esac
exit "$RETVAL"
