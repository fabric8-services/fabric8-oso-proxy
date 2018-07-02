#!/bin/bash

REGISTRY="quay.io"

if [ "$TARGET" = "rhel" ]; then
  DOCKERFILE_DEPLOY="Dockerfile.deploy.rhel"
else
  DOCKERFILE_DEPLOY="Dockerfile.deploy"
fi

function tag_push() {
  local tag=$1

  docker tag f8osoproxy-deploy $tag
  docker push $tag
}

# Source environment variables of the jenkins slave
# that might interest this worker.
function load_jenkins_vars() {
  if [ -e "jenkins-env.json" ]; then
    eval "$(./env-toolkit load -f jenkins-env.json DEVSHIFT_TAG_LEN QUAY_USERNAME QUAY_PASSWORD JENKINS_URL GIT_BRANCH GIT_COMMIT BUILD_NUMBER ghprbSourceBranch ghprbActualCommit BUILD_URL ghprbPullId)"
  fi
}

function login() {
  if [ -n "${QUAY_USERNAME}" -a -n "${QUAY_PASSWORD}" ]; then
    docker login -u ${QUAY_USERNAME} -p ${QUAY_PASSWORD} ${REGISTRY}
  else
    echo "Could not login, missing credentials for the registry"
  fi
}

# Output command before executing
set -x

# Exit on error
set -e

 # We need to disable selinux for now, XXX
/usr/sbin/setenforce 0 || :

# Get all the deps in
yum -y install \
   docker \
   make \
   git \
   curl

load_jenkins_vars

TAG=$(echo $GIT_COMMIT | cut -c1-${DEVSHIFT_TAG_LEN})

service docker start
