ARG BUILD_TAG=devel
FROM samip537/archlinux:${BUILD_TAG}
LABEL maintainer="Jguer,joaogg3 at google mail"

WORKDIR /app

RUN pacman -Sy --overwrite=* --needed --noconfirm \
    go git


ENV ARCH=x86_64
COPY . .
