# The packages to build
GO_PACKAGES ?= cmd/apply-setters cmd/digester cmd/gatekeeper-set-enforcement-action cmd/helm-upgrader cmd/kubeconform cmd/remove-local-config-resources cmd/render-helm-chart cmd/set-annotations cmd/set-labels cmd/source-helm-chart cmd/package-compositor # cmd/template-kyaml

KO_PACKAGES ?= cmd/apply-setters cmd/digester cmd/gatekeeper-set-enforcement-action cmd/helm-upgrader cmd/kubeconform cmd/remove-local-config-resources cmd/render-helm-chart cmd/set-annotations cmd/set-labels cmd/source-helm-chart cmd/package-compositor

# The platforms we support
PLATFORMS ?= linux/amd64,linux/arm64 # linux/arm

# The "FROM" part of the Dockerfile.  Must supports all of the platforms listed in KO_DEFAULTPLATFORMS.
BUILDER_IMAGE ?= alpine:3.23.3
BASE_IMAGE ?= alpine:3.23.3
BASE_IMAGE_DISTROLESS ?= gcr.io/distroless/static
KO_DEFAULTBASEIMAGE ?= gcr.io/distroless/static
export KO_DEFAULTBASEIMAGE

CGO_ENABLED ?= 0
GCFLAGS ?=
LD_FLAGS += -s -w
LD_FLAGS += -X '$(shell go list -m)/pkg/version.Version=$(VERSION)'

BIN_DIR ?= bin
MAKEFLAGS += --no-print-directory

# For functions building on top of Helm
# HELM_VERSION=v3.20.0
HELM_VERSION=v4.1.1

# REGISTRY ?= ghcr.io/krm-functions
REGISTRY ?= ko.local
CONTAINER_PUSH ?= false

VERSION ?= $(shell git describe --tags --always --dirty)

SHELL := /usr/bin/env bash -o errexit -o pipefail -o nounset

-include Makefile.local
include Makefile.test
include Makefile.cosign

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
	export KO_DEFAULTPLATFORMS="$(PLATFORMS)"; \
	for package in $(KO_PACKAGES); do \
		ko build ./$$package --base-import-paths --push=$(CONTAINER_PUSH); \
	done

# --platform $(PLATFORMS)\
.PHONY: base-image
base-image:
	docker buildx build \
		--build-arg ARG_FROM=$(BASE_IMAGE) --build-arg ARG_BUILDER_IMAGE=$(BUILDER_IMAGE) \
		--build-arg ARG_HELM_VERSION=$(HELM_VERSION) base-images/helm/

.PHONY: clean
clean:
	rm -rf $(BIN_DIR)

