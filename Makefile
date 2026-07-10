MODULE  := github.com/melonyzu/slick-code-cli
BINARY  := slickcode
VERSION := $(shell git describe --tags --dirty --always 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -X $(MODULE)/pkg/version.Version=$(VERSION) \
           -X $(MODULE)/pkg/version.Commit=$(COMMIT) \
           -X $(MODULE)/pkg/version.Date=$(DATE)

.PHONY: build
build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/slickcode

.PHONY: install
install:
	go install -ldflags "$(LDFLAGS)" ./cmd/slickcode

.PHONY: test
test:
	go test ./...

.PHONY: fmt
fmt:
	gofmt -l -w .

.PHONY: vet
vet:
	go vet ./...

.PHONY: lint
lint:
	golangci-lint run

.PHONY: check
check: fmt vet lint test

.PHONY: clean
clean:
	rm -rf bin dist
