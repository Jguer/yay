FROM lopsided/archlinux-arm32v7:devel

ARG QEMU_STATIC=build/qemu-arm-static
ADD ${QEMU_STATIC} /usr/bin

LABEL maintainer="Jguer,joaogg3 at google mail"

RUN pacman -Sy; pacman --noconfirm -S go git ca-certificates-utils

ADD . .

ARG MAKE_ARG=package
RUN make ${MAKE_ARG}
