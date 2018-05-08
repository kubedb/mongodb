#!/bin/bash

set -x -e

#install python pip
apt-get update > /dev/null
apt-get install -y python python-pip > /dev/null

#install kubectl
curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl &> /dev/null
chmod +x ./kubectl
mv ./kubectl /bin/kubectl

#install pharmer
mkdir -p $GOPATH/src/github.com/pharmer
pushd $GOPATH/src/github.com/pharmer
git clone https://github.com/pharmer/pharmer &> /dev/null
cd pharmer
go get -u golang.org/x/tools/cmd/goimports
./hack/builddeps.sh &> /dev/null
./hack/make.py &> /dev/null
pharmer
popd

#delete cluster on exit
function cleanup {
    pharmer get cluster
    pharmer delete cluster $NAME
    pharmer get cluster
    sleep 120
    pharmer apply $NAME
    pharmer get cluster
}
trap cleanup EXIT

# name of the cluster
# nameing is based on repo+commit_hash
pushd mongodb
NAME=mongodb-$(git rev-parse --short HEAD)
popd

#create credential file for pharmer
cat > cred.json <<EOF
{
        "token" : "$TOKEN"
}
EOF

#create cluster using pharmer
#note: make sure the zone supports volumes, not all regions support that
#"We're sorry! Volumes are not available for Droplets on legacy hardware in the NYC3 region"
pharmer create credential --from-file=cred.json --provider=DigitalOcean cred
pharmer create cluster $NAME --provider=digitalocean --zone=nyc1 --nodes=2gb=1 --credential-uid=cred --kubernetes-version=v1.10.0
pharmer apply $NAME &> /dev/null
pharmer use cluster $NAME
#wait for cluster to be ready
sleep 300
kubectl get nodes

#create storageclass
cat > sc.yaml <<EOF
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: standard
parameters:
  zone: nyc1
provisioner: external/pharmer
EOF

#create storage-class
kubectl create -f sc.yaml
sleep 120
kubectl get storageclass

#copy mongodb to $GOPATH
mkdir -p $GOPATH/src/github.com/kubedb
cp -r mongodb $GOPATH/src/github.com/kubedb
pushd $GOPATH/src/github.com/kubedb/mongodb

#run tests
./hack/builddeps.sh
./hack/make.py test e2e --v=1
