#!/bin/bash

. cico_setup.sh

./cico_run_tests.sh

docker build -t f8osoproxy-deploy -f Dockerfile.deploy .

login
tag_push ${REGISTRY}/fabric8-services/fabric8-oso-proxy:$TAG
tag_push ${REGISTRY}/fabric8-services/fabric8-oso-proxy:latest

echo 'CICO: Image pushed, ready to update deployed app'