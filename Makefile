.PHONY: all default install test build release clean
VERSION := $(shell git rev-list --count master)
LDFLAGS=-ldflags '-s -w -X main.version=3.${VERSION}'
GOFILES := $(shell ls *.go | grep -v /vendor/)
ARCH=$(shell uname -m)
PKGNAME=yay

PACKAGE=${PKGNAME}_3.${VERSION}_${ARCH}

default: build

all: clean build release package

install:
	go install -v ${LDFLAGS} ${GO_FILES}
test:
	go test ./...
build:
	go build -v ${LDFLAGS}
release:
	mkdir ${PACKAGE}
	cp ./yay ${PACKAGE}/
	cp ./yay.8 ${PACKAGE}/
	cp ./zsh-completion ${PACKAGE}/
	cp ./yay.fish ${PACKAGE}/
	cp ./bash-completion ${PACKAGE}/
package:
	tar -czvf ${PACKAGE}.tar.gz ${PACKAGE}
clean:
	-rm -rf ${PKGNAME}_*

