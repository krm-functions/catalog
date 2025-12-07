-include Makefile.local
include Makefile.test
include Makefile.cosign

# The packages to build
GO_PACKAGES ?= cmd/apply-setters cmd/digester cmd/gatekeeper-set-enforcement-action cmd/helm-upgrader cmd/kubeconform cmd/remove-local-config-resources cmd/render-helm-chart cmd/set-annotations cmd/set-labels cmd/source-helm-chart cmd/package-compositor # cmd/template-kyaml

KO_PACKAGES ?= cmd/kubeconform cmd/remove-local-config-resources

# The platforms we support
#ALL_PLATFORMS ?= linux/amd64 linux/arm linux/arm64 linux/ppc64le linux/s390x
ALL_PLATFORMS ?= linux/amd64 linux/arm linux/arm64

# The "FROM" part of the Dockerfile.  This should be a manifest-list which
# supports all of the platforms listed in ALL_PLATFORMS.
BUILDER_IMAGE ?= alpine:3.20.3
BASE_IMAGE ?= alpine:3.20.3
BASE_IMAGE_DISTROLESS ?= gcr.io/distroless/static

CGO_ENABLED ?= 0
GCFLAGS ?=
LD_FLAGS += -s -w
LD_FLAGS += -X '$(shell go list -m)/pkg/version.Version=$(VERSION)'

BIN_DIR ?= bin
MAKEFLAGS += --no-print-directory

# For functions building on top of Helm
HELM_VERSION=v3.16.1

# REGISTRY ?= ghcr.io/krm-functions
REGISTRY ?= ko.local
CONTAINER_PUSH ?= false

# This version-strategy uses git tags to set the version string
VERSION ?= $(shell git describe --tags --always --dirty)
#
# This version-strategy uses a manual value to set the version string
#VERSION ?= 1.2.3

SHELL := /usr/bin/env bash -o errexit -o pipefail -o nounset

## test: run all tests
.PHONY: test
test:
	go test ./...

## test: run golangci-lint
.PHONY: lint
lint:
	golangci-lint run ./...

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

## Build all go packages
.PHONY: build
build: $(BIN_DIR)
	for package in $(GO_PACKAGES); do \
	  make build-package PACKAGE=$$package; \
	done

.PHONY: build-package
build-package:
	@echo "Building $(PACKAGE)"
	@if [ -f "$(PACKAGE)/Makefile" ]; then \
		echo make -C $(PACKAGE) build; \
	else \
		CGO_ENABLED=$(CGO_ENABLED) \
		go build \
			-ldflags '$(LD_FLAGS)' \
			-gcflags '$(GCFLAGS)' \
			-o $(BIN_DIR)/$(notdir $(PACKAGE)) \
			./$(PACKAGE); \
	fi

## Build all containers
.PHONY: containers
containers:
	export KO_DOCKER_REPO=$(REGISTRY); \
	for package in $(KO_PACKAGES); do \
		ko build ./$$package --base-import-paths --push=$(CONTAINER_PUSH); \
	done

.PHONY: clean
clean:
	rm -rf $(BIN_DIR)

