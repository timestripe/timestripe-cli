BINARY := timestripe
PKG    := github.com/timestripe/timestripe-cli
CMD    := ./cmd/timestripe

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X $(PKG)/internal/cli.version=$(VERSION) \
	-X $(PKG)/internal/cli.commit=$(COMMIT) \
	-X $(PKG)/internal/cli.date=$(DATE)

.PHONY: build install test lint fmt vet tidy gen clean run

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(CMD)

install:
	go install -trimpath -ldflags "$(LDFLAGS)" $(CMD)

run: build
	./bin/$(BINARY)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -s -w .
	goimports -w . 2>/dev/null || true

lint:
	golangci-lint run

tidy:
	go mod tidy

gen:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1 \
		-config api/oapi-codegen.yaml api/openapi.yaml

clean:
	rm -rf bin
