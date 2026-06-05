FROM --platform=linux/amd64 archlinux@sha256:80ee11bda3cb40c46061bbc91db00e3eb8e6147cb17189b4dc9554e501c791a0
LABEL maintainer="Jguer,docker@jguer.space"

ENV GO111MODULE=on
WORKDIR /app

COPY go.mod .

# Disable pacman's landlock/seccomp sandbox: it doesn't work inside BuildKit.
# asciidoc, doxygen, meson needed for pacman-git.
RUN set -eux; \
    sed -i 's/^#\?DisableSandbox.*/DisableSandbox/' /etc/pacman.conf || true; \
    sed -i 's/^#\?DownloadUser.*/#DownloadUser = alpm/' /etc/pacman.conf || true; \
    pacman -Syu --noconfirm --needed \
        pacman-contrib zsh git-delta openssh bat go github-cli \
        archlinux-keyring git gcc make sudo asciidoc doxygen meson curl; \
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v2.12.2; \
    go mod download; \
    rm -rf /var/lib/pacman/sync/* /var/cache/pacman/* /tmp/* /var/tmp/*; \
    rm -rf /usr/share/man/* /usr/share/doc/* || true

# Non-root user with passwordless sudo
RUN useradd -m -s /bin/bash docker \
    && echo "docker ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/docker

USER docker

# Install xgotext
RUN go install github.com/leonelquinteros/gotext/cli/xgotext@latest

ENV PATH="/app/bin:/home/docker/go/bin:$PATH"

WORKDIR /workspace

CMD ["zsh"]