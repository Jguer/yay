FROM lopsided/archlinux:latest

ENV GO111MODULE=on
WORKDIR /app

COPY go.mod .

RUN pacman -Syu --overwrite=* --needed --noconfirm go fakeroot binutils gcc make git gettext && \
    rm -rfv /var/cache/pacman/* /var/lib/pacman/sync/* && \
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.42.0 && \
    go mod download
