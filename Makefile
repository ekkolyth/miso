.PHONY: build install uninstall run test tidy fmt clean

BINARY ?= miso
PKG    := ./cmd

GOBIN ?= $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell go env GOPATH)/bin
endif

build:
	@mkdir -p bin
	go build -o bin/$(BINARY) $(PKG)

install:
	go install $(PKG)

uninstall:
	rm -f $(GOBIN)/$(BINARY)

go:
	go run $(PKG) $(ARGS)

test:
	go test ./...

tidy:
	go mod tidy

fmt:
	gofmt -w $$(go list -f '{{.Dir}}' ./...)

clean:
	rm -rf bin
