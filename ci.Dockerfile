FROM quay.io/gmanka/archlinuxarm:base-devel@sha256:1f2eccbd8730a0e199e86808ea9728a2c29efb290401cfb8dcd12da849d93e1f
LABEL maintainer="Jguer,docker@jguer.space"

ENV GO111MODULE=on
WORKDIR /app

COPY go.mod .

# asciidoc, doxygen, meson needed for pacman-git
RUN set -eux; \
    pacman-key --init; \
    sed -i 's/^#DisableSandboxFilesystem/DisableSandboxFilesystem/' /etc/pacman.conf; \
    sed -i 's/^#DisableSandboxSyscalls/DisableSandboxSyscalls/' /etc/pacman.conf; \
    pacman -Syu --noconfirm --needed archlinux-keyring pacman go git gcc make base-devel sudo asciidoc doxygen meson; \
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@v2.12.2; \
    go mod download; \
    rm -rf /var/lib/pacman/sync/* /var/cache/pacman/* /tmp/* /var/tmp/*; \
    rm -rf /usr/share/man/* /usr/share/doc/* || true; \
    yes | pacman -Scc >/dev/null 2>&1 || true
