PREFIX := /usr
DESTDIR :=
LOCALEDIR := locale
SYSTEMLOCALEPATH := $(DESTDIR)$(PREFIX)/share/locale

ifndef VERSION
MAJORVERSION := 6
MINORVERSION ?= $(shell git rev-list --count master)
endif
VERSION := ${MAJORVERSION}.${MINORVERSION}

LDFLAGS := -ldflags '-s -w -X main.version=${VERSION} -X main.localePath=${SYSTEMLOCALEPATH}'
GOFILES := $(shell ls *.go | grep -v /vendor/)
ARCH := $(shell uname -m)
PKGNAME := yay
BINNAME := yay
PACKAGE := ${PKGNAME}_${VERSION}_${ARCH}

LANGS := fr
POTFILE := ${PKGNAME}.pot
POFILES := $(addprefix $(LOCALEDIR)/,$(addsuffix .po,$(LANGS)))
MOFILES := $(POFILES:.po=.mo)

export GOPATH=$(shell pwd)/.go
export GOROOT=/usr/lib/go

.PHONY: all default install uninstall test build release clean locale
.PRECIOUS: ${LOCALEDIR}/%.po

default: build ${MOFILES}

all: | clean package

install: build ${MOFILES}
	install -Dm755 ${BINNAME} $(DESTDIR)$(PREFIX)/bin/${BINNAME}
	install -Dm644 doc/${PKGNAME}.8 $(DESTDIR)$(PREFIX)/share/man/man8/${PKGNAME}.8
	install -Dm644 completions/bash $(DESTDIR)$(PREFIX)/share/bash-completion/completions/${PKGNAME}
	install -Dm644 completions/zsh $(DESTDIR)$(PREFIX)/share/zsh/site-functions/_${PKGNAME}
	install -Dm644 completions/fish $(DESTDIR)$(PREFIX)/share/fish/vendor_completions.d/${PKGNAME}.fish

	for lang in ${LANGS}; do \
		install -Dm644 ${LOCALEDIR}/$${lang}.mo $(DESTDIR)$(PREFIX)/share/locale/$$lang/LC_MESSAGES/${PKGNAME}.mo; \
	done

uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/${BINNAME}
	rm -f $(DESTDIR)$(PREFIX)/share/man/man8/${PKGNAME}.8
	rm -f $(DESTDIR)$(PREFIX)/share/bash-completion/completions/${PKGNAME}
	rm -f $(DESTDIR)$(PREFIX)/share/zsh/site-functions/_${PKGNAME}
	rm -f $(DESTDIR)$(PREFIX)/share/fish/vendor_completions.d/${PKGNAME}.fish

	for lang in ${LANGS}; do \
		rm -f $(DESTDIR)$(PREFIX)/share/locale/$$lang/LC_MESSAGES/${PKGNAME}.mo; \
	done

test:
	gofmt -l *.go
	@test -z "$$(gofmt -l *.go)" || (echo "Files need to be linted" && false)
	go vet
	go test -v

build:
	go build -v ${LDFLAGS} -o ${BINNAME}

release: | test build ${MOFILES}
	mkdir ${PACKAGE}
	cp ./${BINNAME} ${PACKAGE}/
	cp ./doc/${PKGNAME}.8 ${PACKAGE}/
	cp ./completions/zsh ${PACKAGE}/
	cp ./completions/fish ${PACKAGE}/
	cp ./completions/bash ${PACKAGE}/
	cp  ${MOFILES} ${PACKAGE}/

package: release
	tar -czvf ${PACKAGE}.tar.gz ${PACKAGE}

clean:
	rm -rf ${PKGNAME}_*
	rm -f ${BINNAME}
	rm -f ${POTFILE}
	rm -f ${MOFILES}

locale: ${MOFILES}

${LOCALEDIR}/${POTFILE}: ${GOFILES}
	xgettext --from-code=UTF-8 -Lc -sc -d ${PKGNAME} -kGet -o locale/${PKGNAME}.pot ${GOFILES}

${LOCALEDIR}/%.po: ${LOCALEDIR}/${POTFILE}
	test -f $@ || msginit -l $* -i $< -o $@
	msgmerge -U $@ $<
	touch $@

${LOCALEDIR}/%.mo: ${LOCALEDIR}/%.po
	msgfmt $< -o $@

