FROM quay.io/gmanka/archlinuxarm:base-devel
LABEL maintainer="Jguer,docker@jguer.space"

ENV GO111MODULE=on
WORKDIR /app

COPY go.mod .

# asciidoc, doxygen, meson needed for pacman-git
RUN set -eux; \
    pacman-key --init; \
    sed -i 's/^#DisableSandboxFilesystem/DisableSandboxFilesystem/' /etc/pacman.conf; \
    sed -i 's/^#DisableSandboxSyscalls/DisableSandboxSyscalls/' /etc/pacman.conf; \
    pacman -Syu --noconfirm --needed pacman-contrib fish git-delta openssh bat go github-cli archlinux-keyring pacman go git gcc make base-devel sudo asciidoc doxygen meson; \
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v2.10.1; \
    go mod download; \
    rm -rf /var/lib/pacman/sync/* /var/cache/pacman/* /tmp/* /var/tmp/*; \
    rm -rf /usr/share/man/* /usr/share/doc/* || true; \
    yes | pacman -Scc >/dev/null 2>&1 || true


# Create a non-root user first
RUN useradd -m -s /bin/bash docker

# Set passwordless sudo for the docker user
RUN echo "docker ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/docker

# Switch to the docker user
USER docker

# Install xgotext
RUN go install github.com/leonelquinteros/gotext/cli/xgotext@latest

# Add /app/bin to the PATH
ENV PATH="/app/bin:/home/docker/go/bin:$PATH"

# Set the working directory
WORKDIR /workspace

# Command to run when starting the container
CMD ["fish"]