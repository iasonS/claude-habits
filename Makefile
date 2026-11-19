GO_IMAGE := golang:1.26
# Go runs as the calling user, not as root: git refuses a repository owned by someone else
# (so `go build` could not read the VCS status on Linux), and bin/ would come out owned by root.
# The cache folder is per user, because a cache that root made earlier cannot be written to.
UID := $(shell id -u)
GID := $(shell id -g)
GO := docker run --rm --user $(UID):$(GID) -e HOME=/tmp -v $(CURDIR):/src -w /src -v claude-habits-gocache:/go -e GOCACHE=/go/cache-$(UID) $(GO_IMAGE)

# The binary is built for this machine, so it can read the transcripts directly.
GOOS ?= $(shell uname -s | tr A-Z a-z)
GOARCH ?= $(shell uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')

test:
	$(GO) sh -c 'go vet ./... && go test ./...'

build:
	$(GO) sh -c 'CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath -o bin/claude-habits ./cmd/claude-habits'

.PHONY: test build
