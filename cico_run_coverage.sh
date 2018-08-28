#!/bin/bash

. cico_setup.sh

COVERAGE="f8osoproxy-coverage"
PACKAGE_NAME="github.com/containous/traefik"

GOPATH_IN_CONTAINER=/tmp/go
PACKAGE_PATH=$GOPATH_IN_CONTAINER/src/$PACKAGE_NAME

docker build -t "$COVERAGE" -f Dockerfile.builder .

docker run --detach=true -t \
    --name="$COVERAGE-run" \
    -v $(pwd):$PACKAGE_PATH:Z \
    -u $(id -u $USER):$(id -g $USER) \
    -e GOPATH=$GOPATH_IN_CONTAINER \
    -w $PACKAGE_PATH \
    $COVERAGE

docker exec -t "$COVERAGE-run" bash -ec 'go get github.com/jteeuwen/go-bindata/...'
docker exec -t "$COVERAGE-run" bash -ec 'go generate'
docker exec -t "$COVERAGE-run" bash -ec 'go build -o dist/traefik ./cmd/traefik'

docker exec -t "$COVERAGE-run" bash -ec 'go test -v ./middlewares/osio/ -coverprofile coverage.middlewares -covermode=set -timeout 5m'
docker exec -t "$COVERAGE-run" bash -ec 'go test -v ./provider/osio/ -coverprofile coverage.provider -covermode=set -timeout 5m'
docker exec -t "$COVERAGE-run" bash -ec 'go test -v ./integration/ -integration -osio'

# Upload coverage to codecov.io
# -t <upload_token> copy from https://codecov.io/gh/fabric8-services/fabric8-oso-proxy/settings
bash <(curl -s https://codecov.io/bash) -t 3a135505-4f56-4dce-900e-e451b95601e5

echo "CICO: ran tests and uploaded coverage"
