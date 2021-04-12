export GO111MODULE=on
GOPROXY ?= direct,https://proxy.golang.org
export GOPROXY

BUILD_TAG = devel
ARCH ?= $(shell uname -m)
BIN := yay
DESTDIR :=
GO ?= go
PKGNAME := yay
PREFIX := /usr/local

MAJORVERSION := 10
MINORVERSION := 2
PATCHVERSION := 1
VERSION ?= ${MAJORVERSION}.${MINORVERSION}.${PATCHVERSION}

LOCALEDIR := po
SYSTEMLOCALEPATH := $(PREFIX)/share/locale/

LANGS := pt pt_BR en es eu fr_FR ja pl_PL ru_RU zh_CN
POTFILE := default.pot
POFILES := $(addprefix $(LOCALEDIR)/,$(addsuffix .po,$(LANGS)))
MOFILES := $(POFILES:.po=.mo)

GOFLAGS ?= -trimpath -mod=readonly -modcacherw
GOFLAGS += $(shell pacman -T 'pacman>6' && echo "-tags six")
EXTRA_GOFLAGS ?= -buildmode=pie
LDFLAGS := -X "main.yayVersion=${VERSION}" -X "main.localePath=${SYSTEMLOCALEPATH}" -linkmode=external

RELEASE_DIR := ${PKGNAME}_${VERSION}_${ARCH}
PACKAGE := $(RELEASE_DIR).tar.gz
SOURCES ?= $(shell find . -name "*.go" -type f)

.PRECIOUS: ${LOCALEDIR}/%.po

.PHONY: default
default: build

.PHONY: all
all: | clean release

.PHONY: clean
clean:
	$(GO) clean $(GOFLAGS) -i ./...
	rm -rf $(BIN) $(PKGNAME)_*

.PHONY: test_lint
test_lint: test lint

.PHONY: test
test:
	$(GO) vet $(GOFLAGS) ./...
	@test -z "$$(gofmt -l $(SOURCES))" || (echo "Files need to be linted. Use make fmt" && false)
	$(GO) test $(GOFLAGS) ./...

.PHONY: build
build: $(BIN)

.PHONY: release
release: $(PACKAGE)

.PHONY: docker-release-all
docker-release-all:
	make docker-release-armv7h ARCH=armv7h
	make docker-release-x86_64 ARCH=x86_64
	make docker-release-aarch64 ARCH=aarch64

docker-release:
	docker create --name yay-$(ARCH) yay:${ARCH}
	docker cp yay-$(ARCH):/app/${PACKAGE} $(PACKAGE)
	docker container rm yay-$(ARCH)

.PHONY: docker-build
docker-build:
	docker build -t yay-$(ARCH):${VERSION} .
	docker run -e="ARCH=$(ARCH)" --name yay-$(ARCH) yay-$(ARCH):${VERSION} make build VERSION=${VERSION} PREFIX=${PREFIX}
	docker cp yay-$(ARCH):/app/${BIN} $(BIN)
	docker container rm yay-$(ARCH)

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: install
install: build ${MOFILES}
	install -Dm755 ${BIN} $(DESTDIR)$(PREFIX)/bin/${BIN}
	install -Dm644 doc/${PKGNAME}.8 $(DESTDIR)$(PREFIX)/share/man/man8/${PKGNAME}.8
	install -Dm644 completions/bash $(DESTDIR)$(PREFIX)/share/bash-completion/completions/${PKGNAME}
	install -Dm644 completions/zsh $(DESTDIR)$(PREFIX)/share/zsh/site-functions/_${PKGNAME}
	install -Dm644 completions/fish $(DESTDIR)$(PREFIX)/share/fish/vendor_completions.d/${PKGNAME}.fish
	for lang in ${LANGS}; do \
		install -Dm644 ${LOCALEDIR}/$${lang}.mo $(DESTDIR)$(PREFIX)/share/locale/$$lang/LC_MESSAGES/${PKGNAME}.mo; \
	done

.PHONY: uninstall
uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/${BIN}
	rm -f $(DESTDIR)$(PREFIX)/share/man/man8/${PKGNAME}.8
	rm -f $(DESTDIR)$(PREFIX)/share/bash-completion/completions/${PKGNAME}
	rm -f $(DESTDIR)$(PREFIX)/share/zsh/site-functions/_${PKGNAME}
	rm -f $(DESTDIR)$(PREFIX)/share/fish/vendor_completions.d/${PKGNAME}.fish
	for lang in ${LANGS}; do \
		rm -f $(DESTDIR)$(PREFIX)/share/locale/$$lang/LC_MESSAGES/${PKGNAME}.mo; \
	done

$(BIN): $(SOURCES)
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' $(EXTRA_GOFLAGS) -o $@

$(RELEASE_DIR):
	mkdir $(RELEASE_DIR)

$(PACKAGE): $(BIN) $(RELEASE_DIR) ${MOFILES}
	strip ${BIN}
	cp -t $(RELEASE_DIR) ${BIN} doc/${PKGNAME}.8 completions/* ${MOFILES}
	tar -czvf $(PACKAGE) $(RELEASE_DIR)

locale:
	xgotext -in . -out po
	for lang in ${LANGS}; do \
		test -f po/$$lang.po || msginit -l po/$$lang.po -i po/${POTFILE} -o po/$$lang.po \
		msgmerge -U po/$$lang.po po/${POTFILE}; \
		touch po/$$lang.po; \
	done

${LOCALEDIR}/%.mo: ${LOCALEDIR}/%.po
	msgfmt $< -o $@
