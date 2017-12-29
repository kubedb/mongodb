import sys, os, subprocess, time
from datetime import datetime

Flag = {}

secret_data_dir = "/var/credentials"
secret_Path = "/srv/mongodb/secrets/.admin"


def get_auth():
    Flag["username"] = "root"
    try:
        with open(secret_Path) as data_file:
            for line in data_file:
                Flag["password"] = line
    except:
        print "fail"
        exit(1)


def continuous_exec(process):
    code = 1
    start = datetime.utcnow()
    while True:
        code = subprocess.call(['./utils.sh', process, Flag["host"], Flag["username"], Flag["password"]])
        if code == 0:
            break
        now = datetime.utcnow()
        duration = (now - start).seconds
        if duration > 120:
            break
        time.sleep(30)

    if code != 0:
        print "Fail " + process + " process"
        exit(1)


def main(argv):
    for flag in argv:
        if flag[:2]!= "--":
            continue
        v = flag.split("=", 1)
        Flag[v[0][2:]]=v[1]

    for flag in ["process", "host", "bucket", "folder", "snapshot"]:
        if flag not in Flag:
            print '--%s is required'%flag
            exit(1)
            return

    get_auth()

    if Flag["process"] == "backup":
        continuous_exec("backup")
        code = subprocess.call(['./utils.sh', "push", Flag["bucket"], Flag["folder"], Flag["snapshot"]])
        if code != 0:
            print "Fail to push"
            exit(1)

    if Flag["process"] == "restore":
        get_auth()
        code = subprocess.call(['./utils.sh', "pull", Flag["bucket"], Flag["folder"], Flag["snapshot"]])
        if code != 0:
            print "Fail to pull data"
            exit(1)

        continuous_exec("restore")

    print "success"


if __name__ == "__main__":
    main(sys.argv[1:])
