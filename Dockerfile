ARG BUILD_TAG=devel
FROM samip537/archlinux:${BUILD_TAG}
LABEL maintainer="Jguer,joaogg3 at google mail"

WORKDIR /app

RUN pacman -Syu --overwrite=* --needed --noconfirm \
    go git

COPY go.mod .
COPY go.sum .

RUN go mod download

ENV ARCH=x86_64
COPY . .
