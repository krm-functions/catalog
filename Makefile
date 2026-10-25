-include Makefile.local
include Makefile.test

DBG_MAKEFILE ?=
ifeq ($(DBG_MAKEFILE),1)
    $(warning ***** starting Makefile for goal(s) "$(MAKECMDGOALS)")
    $(warning ***** $(shell date))
else
    # If we're not debugging the Makefile, don't echo recipes.
    MAKEFLAGS += -s
endif

# The binaries to build (just the basenames)
BINS ?= apply-setters digester gatekeeper-set-enforcement-action helm-upgrader kubeconform remove-local-config-resources render-helm-chart set-annotations set-labels source-helm-chart package-compositor # template-kyaml

# Binaries that need external CLIs at runtime. Their images use the 'base-helm' image
# built by apko, see 'build/base-helm.apko.yaml'. All other images use the
# 'defaultBaseImage' from '.ko.yaml'. Currently empty, the Helm-based functions use the
# Helm Go SDK.
HELM_BINS ?=

# The registry to push to. To test 'make push' locally, run a throwaway registry with
# 'crane registry serve --address localhost:5001' and use REGISTRY=localhost:5001/krmfn
# (or REGISTRY=ttl.sh/<random> VERSION=1h for short-lived public images).
REGISTRY ?= ghcr.io/krm-functions

# The version string, from git tags
VERSION ?= $(shell git describe --tags --always --dirty)

# Set this to 1 to build a debugger-friendly binaries.
DBG ?=

###
### These variables should not need tweaking.
###

# We don't need make's built-in rules.
MAKEFLAGS += --no-builtin-rules
# Be pedantic about undefined variables.
MAKEFLAGS += --warn-undefined-variables
.SUFFIXES:

# It's necessary to set this because some environments don't link sh -> bash.
SHELL := /usr/bin/env bash -o errexit -o pipefail -o nounset

# Used to pass a literal ',' to $(call)
comma := ,

# Satisfy --warn-undefined-variables.
GOFLAGS ?=
GOOS ?=
GOARCH ?=

# Used internally.  Users should pass GOOS and/or GOARCH.
OS := $(if $(GOOS),$(GOOS),$(shell GOTOOLCHAIN=local go env GOOS))
ARCH := $(if $(GOARCH),$(GOARCH),$(shell GOTOOLCHAIN=local go env GOARCH))

.DEFAULT_GOAL := build
.PHONY: build kodata base-helm container push version test lint fmt clean help sign verify

ifeq ($(DBG),1)
# Debugging - disable optimizations and inlining
GO_BUILD_FLAGS := -gcflags="all=-N -l"
GO_LDFLAGS :=
else
# Not debugging - trim paths, disable symbols and DWARF (same as .ko.yaml)
GO_BUILD_FLAGS := -trimpath
GO_LDFLAGS := -s -w
endif

build: # @HELP builds binaries for one platform ($OS/$ARCH) into bin/$OS_$ARCH
build:
	echo "# building for $(OS)/$(ARCH)"
	mkdir -p bin/$(OS)_$(ARCH)
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) GOFLAGS="$(GOFLAGS)"                    \
	    go build -o bin/$(OS)_$(ARCH)/ $(GO_BUILD_FLAGS)                            \
	    -ldflags="-X $$(go list -m)/pkg/version.Version=$(VERSION) $(GO_LDFLAGS)"   \
	    $(addprefix ./cmd/,$(BINS))

# Extra files for the images. ko copies 'cmd/<bin>/kodata/' to '/var/run/ko' (KO_DATA_PATH).
# ko rejects symlinks pointing outside kodata, so files are hard-linked copies.
kodata:
	rm -rf cmd/*/kodata
	if [[ " $(BINS) " == *" kubeconform "* ]]; then                    \
	    if [[ ! -d schema-bundle ]]; then                              \
	        echo "schema-bundle missing, run scripts/source-schemas.sh"; \
	        exit 1;                                                    \
	    fi;                                                            \
	    mkdir -p cmd/kubeconform/kodata;                               \
	    cp -al schema-bundle cmd/kubeconform/kodata/schema-bundle;     \
	fi
	if [[ " $(BINS) " == *" package-compositor "* ]]; then             \
	    mkdir -p cmd/package-compositor/kodata;                        \
	    cp ssh/known_hosts cmd/package-compositor/kodata/known_hosts;  \
	fi

BASE_HELM_REF = .base-helm.ref
# The binaries being built that need the base-helm image
BINS_ON_BASE_HELM := $(filter $(HELM_BINS),$(BINS))

base-helm: # @HELP builds and pushes the multi-arch base image for functions needing external CLIs
base-helm:
	mkdir -p .apko-sbom
	apko publish build/base-helm.apko.yaml $(REGISTRY)/base-helm:$(VERSION) --sbom-path .apko-sbom
	# apko '--image-refs' also lists the per-arch images, we need the index
	echo "$(REGISTRY)/base-helm@$$(crane digest $(REGISTRY)/base-helm:$(VERSION))" > $(BASE_HELM_REF)
	echo "base-helm: $$(cat $(BASE_HELM_REF))"

# Builds (or pushes) all images with ko. $(1) are the extra ko arguments.
define ko-build
	for bin in $(BINS); do                                                  \
	    base="";                                                            \
	    if [[ " $(HELM_BINS) " == *" $$bin "* ]]; then                      \
	        base="$$(cat $(BASE_HELM_REF))";                                \
	    fi;                                                                 \
	    echo "container: $(REGISTRY)/$$bin:$(VERSION)";                     \
	    env KO_DOCKER_REPO=$(REGISTRY)/$$bin                                \
	    VERSION=$(VERSION)                                                  \
	    $${base:+KO_DEFAULTBASEIMAGE=$$base}                                \
	    ko build --bare --image-user 65535 $(1) ./cmd/$$bin;                \
	done
endef

container: # @HELP builds containers for all platforms (without pushing)
container: kodata
	$(if $(BINS_ON_BASE_HELM),test -f $(BASE_HELM_REF) || (echo "$(BASE_HELM_REF) missing, run 'make base-helm' first"; exit 1))
	$(call ko-build,--push=false)

push: # @HELP builds and pushes multi-arch containers to the defined registry
push: kodata $(if $(BINS_ON_BASE_HELM),base-helm)
	rm -rf .ko-refs
	mkdir -p .ko-refs
	$(call ko-build,--tags $(VERSION)$(comma)latest --image-refs .ko-refs/$$bin)

sign: # @HELP signs the multi-arch containers pushed by 'make push'
sign:
	# '.ko-refs/<bin>' is written by 'make push', the first line is the
	# 'repo@sha256:...' reference of the multi-arch index (then per-platform images)
	cosign sign --yes $$(head -qn1 .ko-refs/*)

verify: # @HELP verifies cosign signatures of the pushed containers
verify:
	for bin in $(BINS); do                                                  \
	    img=$(REGISTRY)/$$bin:$(VERSION);                                   \
	    echo ">> Verify $$img";                                             \
	    cosign verify                                                       \
	        --certificate-identity-regexp 'https://github.com/krm-functions/catalog/.github/workflows/build.yaml@refs/.*' \
	        --certificate-oidc-issuer https://token.actions.githubusercontent.com \
	        $$img | jq .;                                                   \
	done

version: # @HELP outputs the version string
version:
	echo $(VERSION)

test: # @HELP runs tests
test:
	CGO_ENABLED=0 GOFLAGS="$(GOFLAGS)" go test ./...

lint: # @HELP runs golangci-lint
lint:
	golangci-lint run ./...

fmt: # @HELP formats Go imports
fmt:
	goimports-reviser cmd/ pkg/

clean: # @HELP removes built binaries and temporary files
clean:
	rm -rf bin .ko-refs .apko-sbom $(BASE_HELM_REF) cmd/*/kodata

help: # @HELP prints this message
help:
	echo "VARIABLES:"
	echo "  BINS = $(BINS)"
	echo "  HELM_BINS = $(HELM_BINS)"
	echo "  VERSION = $(VERSION)"
	echo "  OS = $(OS)"
	echo "  ARCH = $(ARCH)"
	echo "  DBG = $(DBG)"
	echo "  GOFLAGS = $(GOFLAGS)"
	echo "  REGISTRY = $(REGISTRY)"
	echo
	echo "TARGETS:"
	grep -hE '^.*: *# *@HELP' $(MAKEFILE_LIST)    \
	    | awk '                                   \
	        BEGIN {FS = ": *# *@HELP"};           \
	        { printf "  %-30s %s\n", $$1, $$2 };  \
	    '
