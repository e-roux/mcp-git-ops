SHELL := /bin/bash
.SILENT:
.ONESHELL:
.DEFAULT_GOAL := help

BINARY  := mcp-git-ops
VERSION := $(shell awk '/^\#\# [0-9]/ {print $$2; exit}' CHANGELOG.md)
TAG     := v$(VERSION)
override GOBIN := $(or $(XDG_BIN_HOME),$(HOME)/.local/bin)

.PHONY: help sync fmt lint typecheck check qa clean distclean
.PHONY: test test.unit test.integration test.e2e
.PHONY: build install release-notes tag release

check: fmt lint typecheck
qa: check test
test: test.unit

sync:
	go mod download
	printf "  OK dependencies downloaded\n"

fmt:
	gofmt -l . | grep -q . && { gofmt -w .; printf "  OK formatted\n"; } || printf "  OK already formatted\n"

lint:
	go vet ./...
	printf "  OK vet passed\n"

typecheck:
	go build ./... && printf "  OK typecheck passed\n"

build:
	go build -ldflags "-s -w" -o $(BINARY) .
	printf "  OK built ./$(BINARY)\n"

test.unit:
	go test -v -race ./...

test.integration:
	printf "  OK no integration tests\n"

test.e2e:
	printf "  OK no e2e tests\n"

install:
	GOBIN=$(GOBIN) go install .
	printf "  OK installed → %s/$(BINARY)\n" "$(GOBIN)"

clean:
	rm -f $(BINARY)
	printf "  OK cleaned\n"

distclean: clean

release-notes:
	awk '/## $(VERSION)/{flag=1;next}/## [0-9]/{flag=0}flag' CHANGELOG.md > release_notes.tmp
	printf "  OK release notes generated\n"

tag:
	git tag -a $(TAG) -m "Release $(TAG)" && git push origin $(TAG)
	printf "  OK tag $(TAG) created and pushed\n"

release: build release-notes
	gh release create $(TAG) ./$(BINARY) --title "$(TAG)" --notes-file release_notes.tmp
	rm -f release_notes.tmp
	printf "  OK release $(TAG) created on GitHub\n"

help:
	printf "\033[36m"
	printf "╔═╗ ╦ ╔╦╗   ╔═╗╔═╗╔═╗\n"
	printf "║╠╗ ║  ║    ║ ║╠═╝╚═╗\n"
	printf "╚═╝ ╩  ╝    ╚═╝╝  ╚═╝\n"
	printf "\033[0m\n"
	printf "Usage: make [target]\n\n"
	printf "\033[1;35mSetup:\033[0m\n"
	printf "  sync         - Download Go dependencies\n"
	printf "  install      - Build and install binary to XDG_BIN_HOME\n"
	printf "\n"
	printf "\033[1;35mDevelopment:\033[0m\n"
	printf "  build        - Build binary locally\n"
	printf "  fmt          - Format Go source\n"
	printf "  lint         - go vet\n"
	printf "  check        - fmt + lint + typecheck\n"
	printf "  qa           - check + test (quality gate)\n"
	printf "\n"
	printf "\033[1;35mTesting:\033[0m\n"
	printf "  test         - Run all tests\n"
	printf "  test.unit    - Unit tests with race detector\n"
	printf "\n"
	printf "\033[1;35mReleasing:\033[0m\n"
	printf "  release-notes - Extract changes for current version\n"
	printf "  tag          - Create and push git tag\n"
	printf "  release      - Build and create GitHub release with macOS binary\n"
	printf "\n"
	printf "\033[1;35mCleanup:\033[0m\n"
	printf "  clean        - Remove local binary\n"
	printf "  distclean    - Deep clean\n"
