.PHONY: all default install uninstall test build release clean package

PREFIX := /usr/local
DESTDIR :=

MAJORVERSION := 10
MINORVERSION ?= 0
PATCHVERSION := 1
VERSION ?= ${MAJORVERSION}.${MINORVERSION}.${PATCHVERSION}

GOFLAGS := -v
EXTRA_GOFLAGS ?=
LDFLAGS := $(LDFLAGS) -X "runtime.version=${VERSION}"

ARCH := $(shell uname -m)
GOCC := $(shell go version)
PKGNAME := yay
BINNAME := yay
PACKAGE := ${PKGNAME}_${VERSION}_${ARCH}

export GO111MODULE=on

SRC = $(shell find . -type f -name '*.go' -not -path "./vendor/*")

default: build

all: | clean build package

install:
	install -Dm755 ${BINNAME} $(DESTDIR)$(PREFIX)/bin/${BINNAME}
	install -Dm644 doc/${PKGNAME}.8 $(DESTDIR)$(PREFIX)/share/man/man8/${PKGNAME}.8
	install -Dm644 completions/bash $(DESTDIR)$(PREFIX)/share/bash-completion/completions/${PKGNAME}
	install -Dm644 completions/zsh $(DESTDIR)$(PREFIX)/share/zsh/site-functions/_${PKGNAME}
	install -Dm644 completions/fish $(DESTDIR)$(PREFIX)/share/fish/vendor_completions.d/${PKGNAME}.fish

uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/${BINNAME}
	rm -f $(DESTDIR)$(PREFIX)/share/man/man8/${PKGNAME}.8
	rm -f $(DESTDIR)$(PREFIX)/share/bash-completion/completions/${PKGNAME}
	rm -f $(DESTDIR)$(PREFIX)/share/zsh/site-functions/_${PKGNAME}
	rm -f $(DESTDIR)$(PREFIX)/share/fish/vendor_completions.d/${PKGNAME}.fish

lint:
	golint -set_exit_status ./...

fmt:
	@gofmt -s -l -w $(SRC)

test:
	go vet ./...
	@test -z "$$(gofmt -l $(SRC))" || (echo "Files need to be linted. Use make fmt" && false)
	go test --race -covermode=atomic -v . ./pkg/...

build:
	go build $(GOFLAGS) $(EXTRA_GOFLAGS) -ldflags '-s -w $(LDFLAGS)' -o ${BINNAME}

release:
	mkdir ${PACKAGE}
	cp ./${BINNAME} ${PACKAGE}/
	cp ./doc/${PKGNAME}.8 ${PACKAGE}/
	cp ./completions/zsh ${PACKAGE}/
	cp ./completions/fish ${PACKAGE}/
	cp ./completions/bash ${PACKAGE}/

docker-release-aarch64:
	docker build -f build/aarch64.Dockerfile -t yay-aarch64:${VERSION} .
	docker run --name yay-aarch64 yay-aarch64:${VERSION}
	docker cp yay-aarch64:${PKGNAME}_${VERSION}_aarch64.tar.gz ${PKGNAME}_${VERSION}_aarch64.tar.gz
	docker container rm yay-aarch64

docker-release-armv7h:
	docker build -f build/armv7h.Dockerfile -t yay-armv7h:${VERSION} .
	docker create --name yay-armv7h yay-armv7h:${VERSION}
	docker cp yay-armv7h:${PKGNAME}_${VERSION}_armv7l.tar.gz ${PKGNAME}_${VERSION}_armv7h.tar.gz
	docker container rm yay-armv7h

docker-release-x86_64:
	docker build -f build/x86_64.Dockerfile -t yay-x86_64:${VERSION} .
	docker create --name yay-x86_64 yay-x86_64:${VERSION}
	docker cp yay-x86_64:${PKGNAME}_${VERSION}_x86_64.tar.gz ${PKGNAME}_${VERSION}_x86_64.tar.gz
	docker container rm yay-x86_64

docker-release: | docker-release-x86_64 docker-release-aarch64 docker-release-armv7h

docker-build:
	docker build -f build/${ARCH}.Dockerfile --build-arg MAKE_ARG=build -t yay-build-${ARCH}:${VERSION} .
	docker create --name yay-build-${ARCH} yay-build-${ARCH}:${VERSION}
	docker cp yay-build-${ARCH}:${BINNAME} ${BINNAME}
	docker container rm yay-build-${ARCH}

package: release
	tar -czvf ${PACKAGE}.tar.gz ${PACKAGE}
clean:
	rm -rf ${PKGNAME}_*
	rm -f ${BINNAME}
