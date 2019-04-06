.PHONY: all default install uninstall test build release clean package

PREFIX := /usr/local
DESTDIR :=

MAJORVERSION := 9
MINORVERSION ?= 2
PATCHVERSION := 0
VERSION ?= ${MAJORVERSION}.${MINORVERSION}.${PATCHVERSION}

LDFLAGS := -ldflags '-s -w -X main.version=${VERSION}'
MOD := -mod=vendor
export GO111MODULE=on
ARCH := $(shell uname -m)
GOCC := $(shell go version)
PKGNAME := yay
BINNAME := yay
PACKAGE := ${PKGNAME}_${VERSION}_${ARCH}

ifneq (,$(findstring gccgo,$(GOCC)))
	export GOPATH=$(shell pwd)/.go
	LDFLAGS := -gccgoflags '-s -w'
	MOD :=
endif

default: build

all: | clean package

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

test:
	gofmt -l *.go
	@test -z "$$(gofmt -l *.go)" || (echo "Files need to be linted" && false)
	go vet
	go test -v

build:
	go build -v ${LDFLAGS} -o ${BINNAME} ${MOD}

release: | test build
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

docker-release-x86_64:
	docker build -f build/x86_64.Dockerfile -t yay-x86_64:${VERSION} .
	docker create --name yay-x86_64 yay-x86_64:${VERSION}
	docker cp yay-x86_64:${PKGNAME}_${VERSION}_x86_64.tar.gz ${PKGNAME}_${VERSION}_x86_64.tar.gz
	docker container rm yay-x86_64

docker-release: | docker-release-x86_64 docker-release-aarch64

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

