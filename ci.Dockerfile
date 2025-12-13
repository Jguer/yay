FROM docker.io/gmanka/archlinuxarm:base-devel
LABEL maintainer="Jguer,docker@jguer.space"

ENV GO111MODULE=on
WORKDIR /app

COPY go.mod .

RUN set -eux; \
    pacman-key --init; \
    pacman -Syu --noconfirm --needed archlinux-keyring pacman go git gcc make base-devel sudo; \
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v2.7.2; \
    go mod download; \
    rm -rf /var/lib/pacman/sync/* /var/cache/pacman/* /tmp/* /var/tmp/*; \
    rm -rf /usr/share/man/* /usr/share/doc/* || true; \
    yes | pacman -Scc >/dev/null 2>&1 || true
