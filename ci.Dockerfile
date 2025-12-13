FROM docker.io/gmanka/archlinuxarm:base-devel
LABEL maintainer="Jguer,docker@jguer.space"

ENV GO111MODULE=on
WORKDIR /app

COPY go.mod .

ARG EXTRA_PKGS=""
RUN set -eux; \
    pacman-key --init; \
    pacman -Syu --noconfirm --needed archlinux-keyring pacman go git gcc make base-devel sudo; \
    if [ -n "${EXTRA_PKGS}" ]; then pacman -S --noconfirm --needed ${EXTRA_PKGS}; fi; \
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v2.4.0; \
    go mod download; \
    rm -rf /var/lib/pacman/sync/* /var/cache/pacman/* /tmp/* /var/tmp/*; \
    rm -rf /usr/share/man/* /usr/share/doc/* || true; \
    yes | pacman -Scc >/dev/null 2>&1 || true
