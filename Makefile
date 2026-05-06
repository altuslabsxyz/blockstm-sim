#!/usr/bin/make -f

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

BUILD_TAGS ?=
LDFLAGS := -X github.com/altuslabsxyz/blockstm-sim/version.Version=$(VERSION) \
           -X github.com/altuslabsxyz/blockstm-sim/version.Commit=$(COMMIT) \
           -X "github.com/altuslabsxyz/blockstm-sim/version.BuildTags=$(BUILD_TAGS)"

BUILD_FLAGS := -tags "$(BUILD_TAGS)" -ldflags '$(LDFLAGS)'

.PHONY: build build-simharness build-canary test test-canary lint clean

build:
	go build $(BUILD_FLAGS) -o build/blockstm-sim ./cmd/blockstm-sim

build-simharness:
	$(MAKE) build BUILD_TAGS="sdk_hooks simharness"

build-canary:
	$(MAKE) build BUILD_TAGS="sdk_hooks simharness simharness_canary"

test:
	go test ./...

test-canary:
	go test -tags "sdk_hooks simharness simharness_canary" -v -count=1 ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf build/
