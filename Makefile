.PHONY: build doc fmt lint run test vendor_clean vendor_get vendor_update vet

# Prepend our _vendor directory to the system GOPATH
# so that import path resolution will prioritize
# our third party snapshots.
VERSION := $(shell git rev-list --count master)
LDFLAGS=-ldflags "-s -w -X main.version=${VERSION}"
GOFILES := $(shell ls *.go | grep -v /vendor/)
BINARY=./bin/yay

default: build

install:
	go install -v ${LDFLAGS} ${GO_FILES}

build:
	go build -v -o ${BINARY} ${LDFLAGS} ${GO_FILES}
release:
	go build -v -o ${BINARY} ./src/main.go

run: build
	${BINARY}

clean:
	go clean

