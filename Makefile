.PHONY: all default install uninstall test build release clean package

PREFIX := /usr
DESTDIR :=

MAJORVERSION := 8
MINORVERSION ?= 2
PATCHVERSION := 0
VERSION ?= ${MAJORVERSION}.${MINORVERSION}.${PATCHVERSION}

LDFLAGS := -ldflags '-s -w -X main.version=${VERSION}'
ARCH := $(shell uname -m)
PKGNAME := yay
BINNAME := yay
PACKAGE := ${PKGNAME}_${VERSION}_${ARCH}

export GOPATH=$(shell pwd)/.go

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
	go build -v ${LDFLAGS} -o ${BINNAME}

release: | test build
	mkdir ${PACKAGE}
	cp ./${BINNAME} ${PACKAGE}/
	cp ./doc/${PKGNAME}.8 ${PACKAGE}/
	cp ./completions/zsh ${PACKAGE}/
	cp ./completions/fish ${PACKAGE}/
	cp ./completions/bash ${PACKAGE}/

package: release
	tar -czvf ${PACKAGE}.tar.gz ${PACKAGE}
clean:
	rm -rf ${PKGNAME}_*
	rm -f ${BINNAME}

