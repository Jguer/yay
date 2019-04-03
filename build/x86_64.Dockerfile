FROM archlinux/base

LABEL maintainer="Jguer,joaogg3 at google mail"

RUN pacman -Sy; pacman --noconfirm -S gcc go git tar make

ADD . .

ARG MAKE_ARG=package
RUN make ${MAKE_ARG}
