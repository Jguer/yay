.PHONY: build doc fmt lint run test vendor_clean vendor_get vendor_update vet

# Prepend our _vendor directory to the system GOPATH
# so that import path resolution will prioritize
# our third party snapshots.
LDFLAGS=-ldflags "-s -w"
GOFILES=$(shell ls *.go)
BINARY=./bin/yay

default: build

build:
	go build -v -o ${BINARY} ${LDFLAGS} ${GOFILES}
release:
	go build -v -o ${BINARY} ./src/main.go

run: build
	${BINARY}

clean:
	go clean

