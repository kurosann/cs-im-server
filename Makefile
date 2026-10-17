VERSION     ?= $(shell git describe --tags --always --dirty)
BRANCH      ?= $(shell git branch --show-current)
REVISION    ?= $(shell git rev-parse --short HEAD)
BUILD_TIME  ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS     := -w -s \
               -X main.Version=$(VERSION) \
               -X main.Branch=$(BRANCH) \
               -X main.Revision=$(REVISION) \
               -X main.BuildTime=$(BUILD_TIME)

.PHONY: run test build

run:
	go run ./cmd

test:
	go test ./...

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/cs-im-server ./cmd
