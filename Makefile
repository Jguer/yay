export GO111MODULE=on

ARCH ?= $(shell uname -m)
BIN := yay
DESTDIR :=
GO ?= go
PKGNAME := yay
PREFIX := /usr/local

MAJORVERSION := 9
MINORVERSION := 3
PATCHVERSION := 2
VERSION ?= ${MAJORVERSION}.${MINORVERSION}.${PATCHVERSION}

GOFLAGS := -v
EXTRA_GOFLAGS ?=
LDFLAGS := $(LDFLAGS) -X "main.version=${VERSION}"

RELEASE_DIR := ${PKGNAME}_${VERSION}_${ARCH}
PACKAGE := $(RELEASE_DIR).tar.gz
SOURCES ?= $(shell find . -path ./vendor -prune -o -name "*.go" -type f)

.PHONY: all
all: | clean release

.PHONY: default
default: build

.PHONY: clean
clean:
	$(GO) clean -i ./...
	rm -rf $(BIN) $(PKGNAME)_$(VERSION)_*

.PHONY: test
test: test-vendor
	$(GO) vet ./...
	@test -z "$$(gofmt -l *.go)" || (echo "Files need to be linted. Use make fmt" && false)
	$(GO) test -mod=vendor --race -covermode=atomic -v . ./pkg/...

.PHONY: build
build: $(BIN)

.PHONY: release
release: $(PACKAGE)

$(BIN): $(SOURCES)
	$(GO) build -mod=vendor -ldflags '-s -w $(LDFLAGS)' $(GOFLAGS) $(EXTRA_GOFLAGS) -o $@

$(RELEASE_DIR):
	mkdir $(RELEASE_DIR)

$(PACKAGE): $(BIN) $(RELEASE_DIR)
	cp -t $(RELEASE_DIR) ${BIN} doc/${PKGNAME}.8 completions/*
	tar -czvf $(PACKAGE) $(RELEASE_DIR)

.PHONY: docker-release-all
docker-release-all:
	make docker-release ARCH=x86_64
	make docker-release ARCH=armv7h
	make docker-release ARCH=aarch64

.PHONY: docker-release
docker-release:
	docker build --target builder_env --build-arg BUILD_ARCH="$(ARCH)" -t yay-$(ARCH):${VERSION} .
	docker run -e="ARCH=$(ARCH)" --name yay-$(ARCH) yay-$(ARCH):${VERSION} make release
	docker cp yay-$(ARCH):/app/${PACKAGE} $(PACKAGE)
	docker container rm yay-$(ARCH)

.PHONY: docker-build
docker-build:
	docker build --target builder --build-arg BUILD_ARCH="$(ARCH)" -t yay-build-$(ARCH):${VERSION} .
	docker run -e="ARCH=$(ARCH)" --name yay-build-${ARCH} yay-build-${ARCH}:${VERSION} /bin/sh
	docker cp yay-build-${ARCH}:/app/${BIN} ${BIN}
	docker container rm yay-build-${ARCH}

.PHONY: test-vendor
test-vendor: vendor
	@diff=$$(git diff vendor/); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make vendor' and commit the result:"; \
		echo "$${diff}"; \
		exit 1; \
	fi;

.PHONY: vendor
vendor:
	$(GO) mod tidy && $(GO) mod vendor

.PHONY: install
install:
	install -Dm755 ${BIN} $(DESTDIR)$(PREFIX)/bin/${BIN}
	install -Dm644 doc/${PKGNAME}.8 $(DESTDIR)$(PREFIX)/share/man/man8/${PKGNAME}.8
	install -Dm644 completions/bash $(DESTDIR)$(PREFIX)/share/bash-completion/completions/${PKGNAME}
	install -Dm644 completions/zsh $(DESTDIR)$(PREFIX)/share/zsh/site-functions/_${PKGNAME}
	install -Dm644 completions/fish $(DESTDIR)$(PREFIX)/share/fish/vendor_completions.d/${PKGNAME}.fish

.PHONY: uninstall
uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/${BIN}
	rm -f $(DESTDIR)$(PREFIX)/share/man/man8/${PKGNAME}.8
	rm -f $(DESTDIR)$(PREFIX)/share/bash-completion/completions/${PKGNAME}
	rm -f $(DESTDIR)$(PREFIX)/share/zsh/site-functions/_${PKGNAME}
	rm -f $(DESTDIR)$(PREFIX)/share/fish/vendor_completions.d/${PKGNAME}.fish
