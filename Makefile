.PHONY: build doc fmt lint run test vendor_clean vendor_get vendor_update vet

# Prepend our _vendor directory to the system GOPATH
# so that import path resolution will prioritize
# our third party snapshots.
VERSION := $(shell git rev-list --count master)
LDFLAGS=-ldflags "-s -w -X main.version=${VERSION}"
GOFILES := $(shell ls *.go | grep -v /vendor/)
PKGNAME=yay
BINARY=./bin/${PKGNAME}

ARCH64="amd64"
ARCH86="386"

default: build

install:
	go install -v ${LDFLAGS} ${GO_FILES}
test:
	go test ./...
build:
	go build -v -o ${BINARY} ${LDFLAGS} ./cmd/yay/
release:
	GOARCH=${ARCH64} go build -v -o ./${PKGNAME}_1.${VERSION}_${ARCH64}/${PKGNAME} ${LDFLAGS} ./cmd/yay/
	cp ./LICENSE ./${PKGNAME}_1.${VERSION}_${ARCH64}/
	cp ./yay.fish ./${PKGNAME}_1.${VERSION}_${ARCH64}/
	cp ./zsh-completion ./${PKGNAME}_1.${VERSION}_${ARCH64}/
	cp ./bash-completion ./${PKGNAME}_1.${VERSION}_${ARCH64}/
	tar -czvf ${PKGNAME}_1.${VERSION}_${ARCH64}.tar.gz ${PKGNAME}_1.${VERSION}_${ARCH64}
	#GOARCH=${ARCH86} go build -v -o ./${PKGNAME}_1.${VERSION}_${ARCH86}/${PKGNAME} ${LDFLAGS} ./cmd/yay/

run:
	build
	${BINARY}

clean:
	go clean
	rm -r ./${PKGNAME}_1*

