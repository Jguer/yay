export GO111MODULE=on
GOPROXY ?= https://proxy.golang.org
export GOPROXY

BUILD_TAG = devel
ARCH ?= $(shell uname -m)
BIN := yay
DESTDIR :=
GO ?= go
PKGNAME := yay
PREFIX := /usr/local

MAJORVERSION := 9
MINORVERSION := 4
PATCHVERSION := 2
VERSION ?= ${MAJORVERSION}.${MINORVERSION}.${PATCHVERSION}

GOFLAGS := -v -mod=mod
EXTRA_GOFLAGS ?=
LDFLAGS := $(LDFLAGS) -X "main.yayVersion=${VERSION}"

RELEASE_DIR := ${PKGNAME}_${VERSION}_${ARCH}
PACKAGE := $(RELEASE_DIR).tar.gz
SOURCES ?= $(shell find . -name "*.go" -type f ! -path "./vendor/*")

.PHONY: default
default: build

.PHONY: all
all: | clean release

.PHONY: clean
clean:
	$(GO) clean $(GOFLAGS) -i ./...
	rm -rf $(BIN) $(PKGNAME)_*

.PHONY: test
test:
	$(GO) vet $(GOFLAGS) ./...
	@test -z "$$(gofmt -l *.go)" || (echo "Files need to be linted. Use make fmt" && false)
	$(GO) test $(GOFLAGS) --race -covermode=atomic . ./pkg/...

.PHONY: build
build: $(BIN)

.PHONY: release
release: $(PACKAGE)

$(BIN): $(SOURCES)
	$(GO) build $(GOFLAGS) -ldflags '-s -w $(LDFLAGS)' $(EXTRA_GOFLAGS) -o $@

$(RELEASE_DIR):
	mkdir $(RELEASE_DIR)

$(PACKAGE): $(BIN) $(RELEASE_DIR)
	cp -t $(RELEASE_DIR) ${BIN} doc/${PKGNAME}.8 completions/*
	tar -czvf $(PACKAGE) $(RELEASE_DIR)

.PHONY: docker-release-all
docker-release-all:
	make docker-release-armv7h ARCH=armv7h
	make docker-release-x86_64 ARCH=x86_64
	make docker-release-aarch64 ARCH=aarch64

.PHONY: docker-release-armv7h
docker-release-armv7h:
	docker build --build-arg="BUILD_TAG=arm32v7-devel" -t yay-$(ARCH):${VERSION} .
	docker run -e="ARCH=$(ARCH)" --name yay-$(ARCH) yay-$(ARCH):${VERSION} make release VERSION=${VERSION}
	docker cp yay-$(ARCH):/app/${PACKAGE} $(PACKAGE)
	docker container rm yay-$(ARCH)

.PHONY: docker-release-aarch64
docker-release-aarch64:
	docker build --build-arg="BUILD_TAG=arm64v8-devel" -t yay-$(ARCH):${VERSION} .
	docker run -e="ARCH=$(ARCH)" --name yay-$(ARCH) yay-$(ARCH):${VERSION} make release VERSION=${VERSION}
	docker cp yay-$(ARCH):/app/${PACKAGE} $(PACKAGE)
	docker container rm yay-$(ARCH)

.PHONY: docker-release-x86_64
docker-release-x86_64:
	docker build --build-arg="BUILD_TAG=devel" -t yay-$(ARCH):${VERSION} .
	docker run -e="ARCH=$(ARCH)" --name yay-$(ARCH) yay-$(ARCH):${VERSION} make release VERSION=${VERSION}
	docker cp yay-$(ARCH):/app/${PACKAGE} $(PACKAGE)
	docker container rm yay-$(ARCH)

.PHONY: docker-build
docker-build:
	docker build -t yay-$(ARCH):${VERSION} .
	docker run -e="ARCH=$(ARCH)" --name yay-$(ARCH) yay-$(ARCH):${VERSION} make build VERSION=${VERSION}
	docker cp yay-$(ARCH):/app/${BIN} $(BIN)
	docker container rm yay-$(ARCH)

.PHONY: test-vendor
test-vendor: vendor
	@diff=$$(git diff vendor/); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make vendor' and commit the result:"; \
		echo "$${diff}"; \
		exit 1; \
	fi;

.PHONY: lint
lint:
	golangci-lint run
	golint -set_exit_status . ./pkg/...

.PHONY: fmt
fmt:
	#go fmt -mod=vendor $(GOFILES) ./... Doesn't work yet but will be supported soon
	gofmt -s -w $(SOURCES)

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
