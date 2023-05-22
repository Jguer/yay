FROM docker.io/jguer/yay-builder:latest
LABEL maintainer="Jguer,docker@jguer.space"

ARG VERSION
ARG PREFIX
ARG ARCH

WORKDIR /app

COPY . .

RUN make release VERSION=${VERSION} PREFIX=${PREFIX} ARCH=${ARCH}
