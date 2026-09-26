GO       ?= go
BIN      ?= bin/contemper
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -X main.version=$(VERSION)

.PHONY: build test vet stubs example e2e clean

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/contemper

test:
	$(GO) vet ./...
	$(GO) test ./...

vet:
	$(GO) vet ./...

stubs:
	./hack/fetch-stubs.sh

example:
	podman build -t contemper-example:dev examples/alpine

e2e:
	@echo "e2e is environment-specific; run it from outside the repo (see dev notes)."

clean:
	rm -rf bin _out
