#!/bin/bash

. cico_setup.sh

BUILDER="f8osoproxy-builder"
PACKAGE_NAME="github.com/containous/traefik"

GOPATH_IN_CONTAINER=/tmp/go
PACKAGE_PATH=$GOPATH_IN_CONTAINER/src/$PACKAGE_NAME

docker build -t "$BUILDER" -f Dockerfile.builder .

docker run --detach=true -t \
    --name="$BUILDER-run" \
    -v $(pwd):$PACKAGE_PATH:Z \
    -u $(id -u $USER):$(id -g $USER) \
    -e GOPATH=$GOPATH_IN_CONTAINER \
    -w $PACKAGE_PATH \
    $BUILDER

docker exec -t "$BUILDER-run" bash -ec 'go get github.com/jteeuwen/go-bindata/...'
docker exec -t "$BUILDER-run" bash -ec 'go generate'
docker exec -t "$BUILDER-run" bash -ec 'go build -o dist/traefik ./cmd/traefik'
docker exec -t "$BUILDER-run" bash -ec 'go test -v ./middlewares/osio/'
docker exec -t "$BUILDER-run" bash -ec 'go test -v ./provider/osio/'
docker exec -t "$BUILDER-run" bash -ec 'go test -v ./integration/ -integration -osio'
