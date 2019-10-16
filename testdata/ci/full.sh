#!/bin/bash
#set -evx

# Objective of this script is to be the most vendor agnostic possible
# It builds and tests yay independently of hardware

VERSION="$(git describe --long --tags | sed 's/^v//;s/\([^-]*-g\)/r\1/;s/-/./g')"
export VERSION
export ARCH="x86_64"

docker build --build-arg BUILD_ARCH=${ARCH} --target builder_env -t yay-builder_env . || exit $?
docker build --build-arg BUILD_ARCH=${ARCH} --target builder -t yay-builder . || exit $?

# Our unit test and packaging container
docker run --rm --name yay-go-tests yay-builder_env:latest make test || exit $?

# Lint project
docker run --rm --name yay-go-lint yay-builder_env:latest make lint || exit $?

# Build image for integration testing
# docker build -t yay . || exit $?
# Do integration testing
# TODO

# Create a release asset
docker run --name artifact_factory yay-builder make release ARCH=${ARCH} VERSION="${VERSION}"
rc=$?
if [[ $rc != 0 ]]; then
  docker rm artifact_factory
  exit $rc
fi

# Copy bin and release to artifacts folder
mkdir artifacts
docker cp artifact_factory:/app/yay_"${VERSION}"_${ARCH}.tar.gz ./artifacts/

# Cleanup docker
docker rm artifact_factory
