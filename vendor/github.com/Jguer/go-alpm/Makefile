export GO111MODULE=on

GOFLAGS := -v -mod=vendor
EXTRA_GOFLAGS ?=
LDFLAGS := $(LDFLAGS)
GO ?= go

SOURCES ?= $(shell find . -name "*.go" -type f ! -path "./vendor/*")

.PHONY: default
default: build

.PHONY: build
build:
	$(GO) build $(GOFLAGS) -ldflags '-s -w $(LDFLAGS)' $(EXTRA_GOFLAGS)

.PHONY: test
test:
	$(GO) vet $(GOFLAGS) .
	@test -z "$$(gofmt -l *.go)" || (echo "Files need to be linted. Use make fmt" && false)
	$(GO) test $(GOFLAGS)  .

.PHONY: fmt
fmt:
	gofmt -s -w $(SOURCES)
