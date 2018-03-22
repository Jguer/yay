.PHONY: all default install test build release clean
VERSION := 5.$(shell git rev-list --count master)
LDFLAGS=-ldflags '-s -w -X main.version=${VERSION}'
GOFILES := $(shell ls *.go | grep -v /vendor/)
ARCH=$(shell uname -m)
PKGNAME=yay

PACKAGE=${PKGNAME}_${VERSION}_${ARCH}

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
	cp ./doc/yay.8 ${PACKAGE}/
	cp ./completions/zsh ${PACKAGE}/
	cp ./completions/fish ${PACKAGE}/
	cp ./completions/bash ${PACKAGE}/
package:
	tar -czvf ${PACKAGE}.tar.gz ${PACKAGE}
clean:
	-rm -rf ${PKGNAME}_*

