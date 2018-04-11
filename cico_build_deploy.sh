#!/bin/bash

. cico_setup.sh

./cico_run_tests.sh

login

docker build -t f8osoproxy-deploy -f "${DOCKERFILE_DEPLOY}" .

if [ "$TARGET" = "rhel" ]; then
    tag_push ${REGISTRY}/osio-prod/fabric8-services/fabric8-oso-proxy:$TAG
    tag_push ${REGISTRY}/osio-prod/fabric8-services/fabric8-oso-proxy:latest
else
    tag_push ${REGISTRY}/fabric8-services/fabric8-oso-proxy:$TAG
    tag_push ${REGISTRY}/fabric8-services/fabric8-oso-proxy:latest
fi

echo 'CICO: Image pushed, ready to update deployed app'
