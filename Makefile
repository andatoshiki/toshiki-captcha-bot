SHELL := /bin/sh

APP_NAME ?= toshiki-captcha-bot
CONFIG ?= ./config.yaml
CMD_PATH ?= .
BUILD_DIR ?= ./bin
BIN ?= $(BUILD_DIR)/$(APP_NAME)

GO ?= go
GO_PACKAGES ?= ./...

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo none)
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS ?= -s -w -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)

.PHONY: help tidy fmt vet test test-race coverage run build install clean check ci goreleaser-check snapshot release

.DEFAULT_GOAL := help

help:
	@echo "Toshiki Captcha Bot Make targets"
	@echo ""
	@echo "Usage:"
	@echo "  make <target> [VAR=value]"
	@echo ""
	@echo "Core:"
	@echo "  run              Run the bot with CONFIG=$(CONFIG)"
	@echo "  build            Build binary at $(BIN)"
	@echo "  test             Run unit tests"
	@echo "  coverage         Run tests with coverage report (coverage.out)"
	@echo "  clean            Remove build and coverage artifacts"
	@echo ""
	@echo "Quality:"
	@echo "  fmt              Format Go code"
	@echo "  vet              Run go vet"
	@echo "  test-race        Run tests with race detector"
	@echo "  check / ci       Run fmt + vet + test"
	@echo ""
	@echo "Deps:"
	@echo "  tidy             Run go mod tidy"
	@echo ""
	@echo "Release:"
	@echo "  goreleaser-check Validate GoReleaser config"
	@echo "  snapshot         Build release snapshot locally"
	@echo "  release          Publish release via GoReleaser"

tidy:
	$(GO) mod tidy

fmt:
	$(GO) fmt $(GO_PACKAGES)

vet:
	$(GO) vet $(GO_PACKAGES)

test:
	$(GO) test $(GO_PACKAGES)

test-race:
	$(GO) test -race $(GO_PACKAGES)

coverage:
	$(GO) test $(GO_PACKAGES) -coverprofile=coverage.out
	$(GO) tool cover -func=coverage.out

run:
	$(GO) run $(CMD_PATH) -c $(CONFIG)

build:
	mkdir -p $(BUILD_DIR)
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) $(CMD_PATH)

install:
	$(GO) install -trimpath -ldflags "$(LDFLAGS)" $(CMD_PATH)

clean:
	rm -rf $(BUILD_DIR) coverage.out

check: fmt vet test

ci: check

goreleaser-check:
	goreleaser check --config .goreleaser.yaml

snapshot:
	goreleaser release --snapshot --clean

release:
	goreleaser release --clean
