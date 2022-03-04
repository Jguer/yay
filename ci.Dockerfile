FROM docker.io/lopsided/archlinux:devel

ENV GO111MODULE=on
WORKDIR /app

COPY go.mod .

RUN pacman -Syu --overwrite=* --needed --noconfirm go git && \
    rm -rfv /var/cache/pacman/* /var/lib/pacman/sync/* && \
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.44.2 && \
    go mod download
