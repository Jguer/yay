#!/bin/bash
set -evx

# Objective of this script is to be the most vendor agnostic possible
# It builds and tests yay independently of hardware

export VERSION=$(git describe --long --tags | sed 's/^v//;s/\([^-]*-g\)/r\1/;s/-/./g')
export ARCH="x86_64"
echo '::set-env name=VERSION::$VERSION'
echo '::set-env name=ARCH::$ARCH'

docker build --build-arg BUILD_ARCH=${ARCH} --target builder_env -t yay-builder_env .
docker build --build-arg BUILD_ARCH=${ARCH} --target builder -t yay-builder .

# Our unit test and packaging container
docker run --name yay-go-tests yay-builder_env:latest make test && golint && golangci-lint run
docker rm yay-go-tests

# docker run yay-builder make lint

# Build image for integration testing
docker build -t yay .

# Do integration testing
# TODO

# Create a release asset
docker run --name artifact_factory yay-builder make release ARCH=${ARCH} VERSION=${VERSION}

# Copy bin and release to artifacts folder
mkdir artifacts
docker cp artifact_factory:/app/yay_${VERSION}_${ARCH}.tar.gz ./artifacts/

# Cleanup docker
docker rm artifact_factory
