FROM agners/archlinuxarm-arm64v8

LABEL maintainer="Jguer,joaogg3 at google mail"

# ARG QEMU_STATIC=build/qemu-arm-static
# ADD ${QEMU_STATIC} /usr/bin

RUN pacman -Sy; pacman --noconfirm -S gcc go git tar make

ADD . .

ARG MAKE_ARG=package
RUN make ${MAKE_ARG}
