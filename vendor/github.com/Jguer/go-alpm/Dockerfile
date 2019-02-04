FROM archlinux:latest
LABEL maintainer="Jguer,joaogg3 at google mail"

ENV GO111MODULE=on
WORKDIR /app

RUN pacman -Sy --overwrite=* --needed --noconfirm \
    archlinux-keyring pacman make gcc gcc-go awk pacman-contrib && paccache -rfk0

# Dependency for linting
# RUN curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s -- -b /bin v1.20.0
# RUN go get golang.org/x/lint/golint && mv /root/go/bin/golint /bin/

COPY . .
