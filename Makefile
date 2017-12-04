.PHONY: build doc fmt lint run test vendor_clean vendor_get vendor_update vet

VERSION := $(shell git rev-list --count master)
LDFLAGS=-ldflags "-s -w -X main.version=${VERSION}"
GOFILES := $(shell ls *.go | grep -v /vendor/)
ARCH=$(shell uname -m)
PKGNAME=yay

OUTPUT="${PKGNAME}_2.${VERSION}_${ARCH}/"
PACKAGE="${PKGNAME}_2.${VERSION}_${ARCH}"

default: build

install:
	go install -v ${LDFLAGS} ${GO_FILES}
test:
	go test ./...
build:
	go build -v -o ${OUTPUT}/${PKGNAME} ${LDFLAGS}
release:
	GOARCH=${ARCH64} go build -v -o ${OUTPUT}/${PKGNAME} ${LDFLAGS}
	cp ./yay.8 ${OUTPUT}
	cp ./zsh-completion ${OUTPUT}
	cp ./yay.fish ${OUTPUT}
	cp ./bash-completion ${OUTPUT}
	tar -czvf ${PACKAGE}.tar.gz ${PACKAGE}
	rm -r ${OUTPUT}
clean:
	go clean
	rm -r ./${PKGNAME}_*

