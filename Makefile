SHELL := /bin/sh

GO ?= go
BINARY ?= bin/mkdone
IMAGE ?= mkdone:latest
ARGS ?= ./
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/ryan/mkdone/internal/app.Version=$(VERSION)
GO_FILES := $(shell find . -name '*.go' -not -path './vendor/*')

.PHONY: all build run test test-race vet fmt fmt-check check clean docker-build docker-run docker-test

all: build

build:
	@mkdir -p $(dir $(BINARY))
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) .

run: build
	./$(BINARY) $(ARGS)

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w $(GO_FILES)

fmt-check:
	@files="$$(gofmt -l $(GO_FILES))"; \
	if [ -n "$$files" ]; then \
		echo "gofmt is required for:"; echo "$$files"; exit 1; \
	fi

check: fmt-check vet test

clean:
	rm -rf bin

docker-build:
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE) .

docker-run:
	docker run --rm -v "$(CURDIR):/work" -w /work $(IMAGE) $(ARGS)

docker-test:
	docker build --target test -t $(IMAGE)-test .
