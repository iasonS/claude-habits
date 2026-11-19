GO_IMAGE := golang:1.26
GO := docker run --rm -v $(CURDIR):/src -w /src -v claude-habits-gocache:/go -e GOCACHE=/go/cache $(GO_IMAGE)

# The binary is built for this machine, so it can read the transcripts directly.
GOOS ?= $(shell uname -s | tr A-Z a-z)
GOARCH ?= $(shell uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')

test:
	$(GO) sh -c 'go vet ./... && go test ./...'

build:
	$(GO) sh -c 'CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath -o bin/claude-habits ./cmd/claude-habits'

.PHONY: test build
