#!/bin/bash

VERSION="10.0.1"

docker build -t yay-builder --target builder . || exit 1

# Our unit test and packaging container
docker run yay-builder make test || exit 1

# docker run yay-builder make lint || exit 1

docker build -t yay .

docker run --name artifact_factory yay-builder make package || exit 1

docker cp artifact_factory:/app/yay yay
docker cp artifact_factory:/app/yay_${VERSION}_x86_64.tar.gz .

docker rm artifact_factory