FROM docker.io/jguer/yay-builder:latest

ENV GO111MODULE=on
WORKDIR /app

COPY go.mod .

RUN pacman-key --init && pacman -Sy && pacman -S --overwrite=* --noconfirm archlinux-keyring && \
    pacman -Su --overwrite=* --needed --noconfirm doxygen meson asciidoc go git gcc make sudo base-devel && \
    rm -rfv /var/cache/pacman/* /var/lib/pacman/sync/* && \
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.50.1 && \
    go mod download
