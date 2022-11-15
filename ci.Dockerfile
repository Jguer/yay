FROM docker.io/jguer/yay-builder:latest

ENV GO111MODULE=on
WORKDIR /app

COPY go.mod .

RUN pacman -Sy && pacman -S --overwrite=* --noconfirm archlinux-keyring && \
    pacman -Su --overwrite=* --needed --noconfirm go git gcc make && \
    rm -rfv /var/cache/pacman/* /var/lib/pacman/sync/* && \
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.50.1 && \
    go mod download
